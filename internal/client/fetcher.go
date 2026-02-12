package client

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

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
	FID string `json:"fid"`
}

type challengeResponse struct {
	ChallengeID string `json:"challenge_id"`
	Challenge   string `json:"challenge"`
}

// Fetch sends a POST /v1/secrets/fetch request and returns the encrypted blobs (base64-decoded).
// allowInsecure controls whether plain HTTP is permitted.
func Fetch(serverURL string, privateKey [32]byte, fid string, refs []string, allowInsecure bool) (map[string][]byte, error) {
	if !strings.HasPrefix(serverURL, "https://") {
		if !allowInsecure {
			return nil, fmt.Errorf("server URL %q is not HTTPS; use --insecure to allow plaintext HTTP", serverURL)
		}
		fmt.Fprintf(os.Stderr, "jingui: WARNING: communicating over plaintext HTTP (%s)\n", serverURL)
	}

	challenge, err := requestChallenge(serverURL, fid)
	if err != nil {
		return nil, err
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

	resp, err := http.Post(serverURL+"/v1/secrets/fetch", "application/json", bytes.NewReader(body))
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

func requestChallenge(serverURL, fid string) (*challengeResponse, error) {
	reqBody := challengeRequest{FID: fid}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal challenge request: %w", err)
	}

	resp, err := http.Post(serverURL+"/v1/secrets/challenge", "application/json", bytes.NewReader(body))
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
