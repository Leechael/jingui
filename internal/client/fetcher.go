package client

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aspect-build/jingui/internal/attestation"
	"github.com/aspect-build/jingui/internal/crypto"
	"golang.org/x/crypto/curve25519"
)

// ComputeFID derives the public key from the private key and returns hex(SHA1(pubkey)).
func ComputeFID(privateKey [32]byte) (string, error) {
	pub, err := curve25519.X25519(privateKey[:], curve25519.Basepoint)
	if err != nil {
		return "", fmt.Errorf("derive public key: %w", err)
	}
	h := sha1.Sum(pub)
	return hex.EncodeToString(h[:]), nil
}

type fetchRequest struct {
	FID               string   `json:"fid"`
	SecretReferences  []string `json:"secret_references"`
	ChallengeID       string   `json:"challenge_id"`
	ChallengeResponse string   `json:"challenge_response"`
}

type fetchResponse struct {
	Secrets map[string]string `json:"secrets"`
}

type challengeRequest struct {
	FID               string              `json:"fid"`
	ClientAttestation *attestation.Bundle `json:"client_attestation,omitempty"`
}

type challengeResponse struct {
	ChallengeID       string              `json:"challenge_id"`
	Challenge         string              `json:"challenge"`
	ServerAttestation *attestation.Bundle `json:"server_attestation,omitempty"`
}

func normalizeServerURL(serverURL string) string {
	return strings.TrimRight(serverURL, "/")
}

func ratlsStrictEnabled() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("JINGUI_RATLS_STRICT")))
	if v == "" {
		return true
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		// Fail-safe: unknown values keep strict mode enabled.
		return true
	}
}

func httpClient() *http.Client {
	return &http.Client{Timeout: 20 * time.Second}
}

// Fetch sends a POST /v1/secrets/fetch request and returns the encrypted blobs (base64-decoded).
// allowInsecure controls whether plain HTTP is permitted.
func Fetch(serverURL string, privateKey [32]byte, fid string, refs []string, allowInsecure bool, command string) (map[string][]byte, error) {
	serverURL = normalizeServerURL(serverURL)
	if !strings.HasPrefix(serverURL, "https://") {
		if !allowInsecure {
			return nil, fmt.Errorf("server URL %q is not HTTPS; use --insecure to allow plaintext HTTP", serverURL)
		}
		fmt.Fprintf(os.Stderr, "jingui: WARNING: communicating over plaintext HTTP (%s)\n", serverURL)
	}

	strict := ratlsStrictEnabled()
	var clientAtt *attestation.Bundle
	if strict {
		bundle, err := collectLocalAttestation()
		if err != nil {
			return nil, fmt.Errorf("collect local attestation: %w", err)
		}
		clientAtt = &bundle
	}

	challenge, err := requestChallenge(serverURL, fid, allowInsecure, clientAtt)
	if err != nil {
		return nil, err
	}

	if strict {
		if challenge.ServerAttestation == nil {
			return nil, fmt.Errorf("challenge response missing server_attestation in strict RA-TLS mode")
		}
		if err := verifyServerAttestation(*challenge.ServerAttestation); err != nil {
			return nil, fmt.Errorf("verify server attestation: %w", err)
		}
	}

	challengeBlob, err := base64.StdEncoding.DecodeString(challenge.Challenge)
	if err != nil {
		return nil, fmt.Errorf("decode challenge blob: %w", err)
	}
	challengePlain, err := crypto.Decrypt(privateKey, challengeBlob)
	if err != nil {
		return nil, fmt.Errorf("decrypt challenge: %w", err)
	}

	reqBody := fetchRequest{
		FID:               fid,
		SecretReferences:  refs,
		ChallengeID:       challenge.ChallengeID,
		ChallengeResponse: base64.StdEncoding.EncodeToString(challengePlain),
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, serverURL+"/v1/secrets/fetch", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create fetch request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if command != "" {
		req.Header.Set("X-Jingui-Command", command)
	}

	resp, err := httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch secrets: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result fetchResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	blobs := make(map[string][]byte, len(result.Secrets))
	for ref, b64 := range result.Secrets {
		decoded, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return nil, fmt.Errorf("base64 decode for %s: %w", ref, err)
		}
		blobs[ref] = decoded
	}

	return blobs, nil
}

func collectLocalAttestation() (attestation.Bundle, error) {
	collector := attestation.NewDstackInfoCollector("")
	return collector.Collect(context.Background())
}

func verifyServerAttestation(bundle attestation.Bundle) error {
	verifier := attestation.NewRATLSVerifier()
	_, err := verifier.Verify(context.Background(), bundle)
	return err
}

func requestChallenge(serverURL, fid string, _ bool, clientAtt *attestation.Bundle) (*challengeResponse, error) {
	serverURL = normalizeServerURL(serverURL)
	reqBody := challengeRequest{FID: fid, ClientAttestation: clientAtt}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal challenge request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, serverURL+"/v1/secrets/challenge", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create challenge request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("request challenge: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read challenge response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("challenge endpoint returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result challengeResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal challenge response: %w", err)
	}
	if result.ChallengeID == "" || result.Challenge == "" {
		return nil, fmt.Errorf("challenge response missing required fields")
	}
	return &result, nil
}

// CheckInstance checks server reachability and whether the instance is registered.
func CheckInstance(serverURL, fid string, allowInsecure bool) error {
	serverURL = normalizeServerURL(serverURL)
	if !strings.HasPrefix(serverURL, "https://") && !allowInsecure {
		return fmt.Errorf("server URL %q is not HTTPS; use --insecure to allow plaintext HTTP", serverURL)
	}
	var clientAtt *attestation.Bundle
	if ratlsStrictEnabled() {
		bundle, err := collectLocalAttestation()
		if err != nil {
			return fmt.Errorf("collect local attestation: %w", err)
		}
		clientAtt = &bundle
	}
	_, err := requestChallenge(serverURL, fid, allowInsecure, clientAtt)
	return err
}
