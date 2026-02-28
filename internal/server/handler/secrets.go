package handler

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
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
			logx.Debugf("ratls.server.challenge strict=true fid=%s dstack_app_id=%s", req.FID, inst.DstackAppID)
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
			if strings.TrimSpace(inst.DstackAppID) == "" {
				logx.Warnf("ratls.server.challenge rejected: instance missing dstack_app_id fid=%s", req.FID)
				c.JSON(http.StatusUnauthorized, gin.H{"error": "instance is missing dstack_app_id"})
				return
			}
			if req.ClientAttestation.AppID != inst.DstackAppID {
				logx.Warnf("ratls.server.challenge rejected: request app_id mismatch fid=%s attested_app_id=%s dstack_app_id=%s", req.FID, req.ClientAttestation.AppID, inst.DstackAppID)
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
			if identity.AppID != inst.DstackAppID {
				logx.Warnf("ratls.server.challenge rejected: verified app_id mismatch fid=%s verified_app_id=%s dstack_app_id=%s", req.FID, identity.AppID, inst.DstackAppID)
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
func HandleFetchSecrets(store *db.Store, strict bool) gin.HandlerFunc {
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

		// Debug policy check: for "read" commands, check if any vault the instance
		// has access to has a debug policy that disables read
		command := strings.ToLower(strings.TrimSpace(c.GetHeader("X-Jingui-Command")))
		if command == "read" {
			// We'll check per-reference below; for the general check we need at least one ref
			// Actually, we check per-vault in the loop below
		}

		secrets := make(map[string]string)

		for _, refStr := range req.SecretReferences {
			ref, err := refparser.Parse(refStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid reference: " + refStr})
				return
			}

			// Access control: check if instance has access to this vault
			hasAccess, err := store.HasVaultAccess(ref.Vault, inst.FID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
				return
			}
			if !hasAccess {
				c.JSON(http.StatusForbidden, gin.H{"error": "vault mismatch for reference: " + refStr})
				return
			}

			// Debug policy check per vault+instance
			if command == "read" {
				policy, err := store.GetDebugPolicy(ref.Vault, req.FID)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load debug policy"})
					return
				}
				if policy != nil && !policy.AllowRead {
					c.JSON(http.StatusForbidden, gin.H{"error": "debug read is disabled for this vault"})
					return
				}
			}

			// Map ref to DB: section=ref.Item, itemName=ref.FieldName
			// For 4-segment refs: jingui://vault/item/section/field → section=item, itemName=field (section in ref becomes sub-group)
			dbSection := ref.Item
			dbItemName := ref.FieldName

			value, err := store.GetFieldValue(ref.Vault, dbSection, dbItemName)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "field not found for reference: " + refStr})
				return
			}

			// ECIES encrypt the value with the TEE instance's public key
			encrypted, err := crypto.Encrypt(pubKey, []byte(value))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "encryption failed"})
				return
			}

			secrets[refStr] = base64.StdEncoding.EncodeToString(encrypted)
		}

		c.JSON(http.StatusOK, fetchSecretsResponse{Secrets: secrets})
	}
}
