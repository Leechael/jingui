package handler

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/aspect-build/jingui/internal/attestation"
	"github.com/aspect-build/jingui/internal/crypto"
	"github.com/aspect-build/jingui/internal/logx"
	"github.com/aspect-build/jingui/internal/refparser"
	"github.com/aspect-build/jingui/internal/server/db"
	"github.com/gin-gonic/gin"
)

const challengeTTL = 2 * time.Minute

type issueChallengeRequest struct {
	FID               string              `json:"fid" binding:"required"`
	ClientAttestation *attestation.Bundle `json:"client_attestation,omitempty"`
}

type issueChallengeResponse struct {
	ChallengeID       string              `json:"challenge_id"`
	Challenge         string              `json:"challenge"`
	ServerAttestation *attestation.Bundle `json:"server_attestation,omitempty"`
}

type fetchSecretsRequest struct {
	FID               string   `json:"fid" binding:"required"`
	SecretReferences  []string `json:"secret_references" binding:"required"`
	ChallengeID       string   `json:"challenge_id" binding:"required"`
	ChallengeResponse string   `json:"challenge_response" binding:"required"`
}

type fetchSecretsResponse struct {
	Secrets map[string]string `json:"secrets"`
}

type challengeEntry struct {
	FID        string
	Nonce      []byte
	ExpiresAt  time.Time
	RAVerified bool
	StrictMode bool
}

type challengeStore struct {
	mu      sync.Mutex
	entries map[string]challengeEntry
}

var fetchChallengeStore = &challengeStore{
	entries: make(map[string]challengeEntry),
}

func (s *challengeStore) issue(fid string, nonce []byte, raVerified bool, strictMode bool, now time.Time) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.gcLocked(now)

	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return "", fmt.Errorf("generate challenge id: %w", err)
	}
	id := hex.EncodeToString(idBytes)

	nonceCopy := make([]byte, len(nonce))
	copy(nonceCopy, nonce)

	s.entries[id] = challengeEntry{
		FID:        fid,
		Nonce:      nonceCopy,
		ExpiresAt:  now.Add(challengeTTL),
		RAVerified: raVerified,
		StrictMode: strictMode,
	}
	return id, nil
}

func (s *challengeStore) consume(challengeID, fid string, response []byte, strictMode bool, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.gcLocked(now)

	entry, ok := s.entries[challengeID]
	if !ok {
		return fmt.Errorf("challenge not found or expired")
	}
	delete(s.entries, challengeID)

	if entry.FID != fid {
		return fmt.Errorf("challenge fid mismatch")
	}
	if strictMode {
		if !entry.StrictMode {
			return fmt.Errorf("challenge mode mismatch")
		}
		if !entry.RAVerified {
			return fmt.Errorf("challenge is not RA-verified")
		}
	}
	if subtle.ConstantTimeCompare(entry.Nonce, response) != 1 {
		return fmt.Errorf("invalid challenge response")
	}
	return nil
}

func (s *challengeStore) gcLocked(now time.Time) {
	for id, e := range s.entries {
		if now.After(e.ExpiresAt) {
			delete(s.entries, id)
		}
	}
}

// HandleIssueChallenge handles POST /v1/secrets/challenge.
// It returns an ECIES-encrypted random challenge bound to the given FID.
func HandleIssueChallenge(store *db.Store, strict bool, verifier attestation.Verifier, serverCollector attestation.Collector) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req issueChallengeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		inst, err := store.GetInstance(req.FID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		if inst == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "instance not found"})
			return
		}
		if len(inst.PublicKey) != 32 {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid instance public key length"})
			return
		}

		var serverAtt *attestation.Bundle
		if strict {
			logx.Debugf("ratls.server.challenge strict=true fid=%s bound_vault=%s bound_attestation_app_id=%s", req.FID, inst.BoundVault, inst.BoundAttestationAppID)
			if verifier == nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "attestation verifier is not configured"})
				return
			}
			if req.ClientAttestation == nil {
				logx.Warnf("ratls.server.challenge rejected: missing client_attestation fid=%s", req.FID)
				c.JSON(http.StatusUnauthorized, gin.H{"error": "client_attestation is required in strict RA-TLS mode"})
				return
			}
			if strings.TrimSpace(req.ClientAttestation.AppID) == "" {
				logx.Warnf("ratls.server.challenge rejected: missing client_attestation.app_id fid=%s", req.FID)
				c.JSON(http.StatusUnauthorized, gin.H{"error": "client_attestation.app_id is required in strict RA-TLS mode"})
				return
			}
			if strings.TrimSpace(inst.BoundAttestationAppID) == "" {
				logx.Warnf("ratls.server.challenge rejected: instance missing bound_attestation_app_id fid=%s", req.FID)
				c.JSON(http.StatusUnauthorized, gin.H{"error": "instance is missing bound_attestation_app_id"})
				return
			}
			if req.ClientAttestation.AppID != inst.BoundAttestationAppID {
				logx.Warnf("ratls.server.challenge rejected: request app_id mismatch fid=%s attested_app_id=%s bound_attestation_app_id=%s", req.FID, req.ClientAttestation.AppID, inst.BoundAttestationAppID)
				c.JSON(http.StatusForbidden, gin.H{"error": "client attestation app_id mismatch"})
				return
			}

			identity, err := verifier.Verify(c.Request.Context(), *req.ClientAttestation)
			if err != nil {
				logx.Warnf("ratls.server.challenge rejected: verify failed fid=%s err=%v", req.FID, err)
				c.JSON(http.StatusUnauthorized, gin.H{"error": "client attestation verification failed"})
				return
			}
			if identity.AppID == "" {
				logx.Warnf("ratls.server.challenge rejected: verified cert missing app_id extension fid=%s", req.FID)
				c.JSON(http.StatusUnauthorized, gin.H{"error": "client attestation certificate does not contain app_id"})
				return
			}
			if identity.AppID != inst.BoundAttestationAppID {
				logx.Warnf("ratls.server.challenge rejected: verified app_id mismatch fid=%s verified_app_id=%s bound_attestation_app_id=%s", req.FID, identity.AppID, inst.BoundAttestationAppID)
				c.JSON(http.StatusForbidden, gin.H{"error": "client RA app_id mismatch"})
				return
			}
			logx.Debugf("ratls.server.challenge peer=client verified_app_id=%q instance_id=%q device_id=%q", identity.AppID, identity.InstanceID, identity.DeviceID)

			if serverCollector == nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "server attestation collector is not configured"})
				return
			}
			bundle, err := serverCollector.Collect(c.Request.Context())
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to collect server attestation: " + err.Error()})
				return
			}
			serverAtt = &bundle
			logx.Debugf("ratls.server.challenge peer=server provided app_id=%q instance_id=%q device_id=%q", bundle.AppID, bundle.Instance, bundle.DeviceID)
		}

		nonce := make([]byte, 32)
		if _, err := rand.Read(nonce); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate challenge"})
			return
		}

		var pubKey [32]byte
		copy(pubKey[:], inst.PublicKey)
		challengeBlob, err := crypto.Encrypt(pubKey, nonce)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encrypt challenge"})
			return
		}

		challengeID, err := fetchChallengeStore.issue(req.FID, nonce, !strict || req.ClientAttestation != nil, strict, time.Now())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to issue challenge"})
			return
		}

		c.JSON(http.StatusOK, issueChallengeResponse{
			ChallengeID:       challengeID,
			Challenge:         base64.StdEncoding.EncodeToString(challengeBlob),
			ServerAttestation: serverAtt,
		})
	}
}

// HandleFetchSecrets handles POST /v1/secrets/fetch.
func HandleFetchSecrets(store *db.Store, masterKey [32]byte, strict bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req fetchSecretsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		challengeResponse, err := base64.StdEncoding.DecodeString(req.ChallengeResponse)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "challenge_response must be valid base64"})
			return
		}
		if err := fetchChallengeStore.consume(req.ChallengeID, req.FID, challengeResponse, strict, time.Now()); err != nil {
			logx.Warnf("ratls.server.fetch rejected: challenge verification failed fid=%s challenge_id=%s err=%v", req.FID, req.ChallengeID, err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "challenge verification failed: " + err.Error()})
			return
		}
		if strict {
			logx.Debugf("ratls.server.fetch strict challenge verification passed fid=%s challenge_id=%s", req.FID, req.ChallengeID)
		}

		// Look up TEE instance by FID
		inst, err := store.GetInstance(req.FID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		if inst == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "instance not found"})
			return
		}

		// Update last used
		_ = store.UpdateLastUsed(req.FID)

		// Build recipient public key
		var pubKey [32]byte
		copy(pubKey[:], inst.PublicKey)

		command := strings.ToLower(strings.TrimSpace(c.GetHeader("X-Jingui-Command")))
		if command == "read" {
			policy, err := store.GetDebugPolicy(inst.BoundVault, inst.BoundItem)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load debug policy"})
				return
			}
			if policy != nil && !policy.AllowReadDebug {
				c.JSON(http.StatusForbidden, gin.H{"error": "debug read is disabled for this item"})
				return
			}
		}

		secrets := make(map[string]string)

		for _, refStr := range req.SecretReferences {
			ref, err := refparser.Parse(refStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid reference: " + refStr})
				return
			}

			// Validate: ref's vault must match instance's bound_vault.
			if ref.Vault != inst.BoundVault {
				c.JSON(http.StatusForbidden, gin.H{"error": "vault mismatch for reference: " + refStr})
				return
			}

			// Validate: ref's item must match instance's bound_item.
			if ref.Item != inst.BoundItem {
				c.JSON(http.StatusForbidden, gin.H{"error": "item mismatch for reference: " + refStr})
				return
			}

			var plainValue []byte

			switch ref.FieldName {
			case "client_id", "client_secret":
				plainValue, err = extractAppField(store, masterKey, ref.Vault, ref.FieldName)
			case "refresh_token":
				plainValue, err = extractVaultItemField(store, masterKey, ref.Vault, ref.Item, ref.FieldName)
			default:
				c.JSON(http.StatusBadRequest, gin.H{"error": "unknown field: " + ref.FieldName})
				return
			}

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve " + refStr + ": " + err.Error()})
				return
			}

			// ECIES encrypt the value with the TEE instance's public key
			encrypted, err := crypto.Encrypt(pubKey, plainValue)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "encryption failed"})
				return
			}

			secrets[refStr] = base64.StdEncoding.EncodeToString(encrypted)
		}

		c.JSON(http.StatusOK, fetchSecretsResponse{Secrets: secrets})
	}
}

func extractAppField(store *db.Store, masterKey [32]byte, vault, fieldName string) ([]byte, error) {
	app, err := store.GetApp(vault)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, fmt.Errorf("vault not found: %s", vault)
	}

	credJSON, err := crypto.DecryptAtRest(masterKey, app.CredentialsEncrypted)
	if err != nil {
		return nil, err
	}

	creds, err := parseGoogleCreds(credJSON)
	if err != nil {
		return nil, err
	}

	switch fieldName {
	case "client_id":
		return []byte(creds.ClientID), nil
	case "client_secret":
		return []byte(creds.ClientSecret), nil
	default:
		return nil, fmt.Errorf("unknown app field: %s", fieldName)
	}
}

func extractVaultItemField(store *db.Store, masterKey [32]byte, vault, item, fieldName string) ([]byte, error) {
	vi, err := store.GetVaultItem(vault, item)
	if err != nil {
		return nil, err
	}
	if vi == nil {
		return nil, fmt.Errorf("secret not found for %s/%s", vault, item)
	}

	secretJSON, err := crypto.DecryptAtRest(masterKey, vi.SecretEncrypted)
	if err != nil {
		return nil, err
	}

	var secretMap map[string]string
	if err := json.Unmarshal(secretJSON, &secretMap); err != nil {
		return nil, err
	}

	val, ok := secretMap[fieldName]
	if !ok {
		return nil, fmt.Errorf("field %q not found in secret", fieldName)
	}
	return []byte(val), nil
}
