package internal

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aspect-build/jingui/internal/crypto"
	"github.com/aspect-build/jingui/internal/server"
	"github.com/aspect-build/jingui/internal/server/db"
	"golang.org/x/crypto/curve25519"
)

const testAdminToken = "test-admin-token-1234567890"

func setupTestServer(t *testing.T) (*httptest.Server, *db.Store, [32]byte) {
	t.Helper()
	var masterKey [32]byte
	rand.Read(masterKey[:])

	store, err := db.NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	cfg := &server.Config{
		MasterKey:  masterKey,
		AdminToken: testAdminToken,
	}

	router := server.NewRouter(store, cfg)
	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)
	cfg.BaseURL = ts.URL

	return ts, store, masterKey
}

func adminRequest(method, url string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+testAdminToken)
	return http.DefaultClient.Do(req)
}

func solveFetchChallenge(t *testing.T, serverURL, fid string, teePriv [32]byte) (string, string) {
	t.Helper()

	chReqBody, _ := json.Marshal(map[string]string{"fid": fid})
	resp, err := http.Post(serverURL+"/v1/secrets/challenge", "application/json", bytes.NewReader(chReqBody))
	if err != nil {
		t.Fatalf("POST /v1/secrets/challenge: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("POST /v1/secrets/challenge: status %d, body: %s", resp.StatusCode, body)
	}

	var chResp struct {
		ChallengeID string `json:"challenge_id"`
		Challenge   string `json:"challenge"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&chResp); err != nil {
		resp.Body.Close()
		t.Fatalf("decode challenge response: %v", err)
	}
	resp.Body.Close()

	blob, err := base64.StdEncoding.DecodeString(chResp.Challenge)
	if err != nil {
		t.Fatalf("decode challenge blob: %v", err)
	}
	plain, err := crypto.Decrypt(teePriv, blob)
	if err != nil {
		t.Fatalf("decrypt challenge: %v", err)
	}

	return chResp.ChallengeID, base64.StdEncoding.EncodeToString(plain)
}

func TestEndToEnd(t *testing.T) {
	ts, store, masterKey := setupTestServer(t)

	// Step 1: Register an app (with admin token)
	credJSON := `{"installed":{"client_id":"test-client-id","client_secret":"test-client-secret","redirect_uris":["http://localhost"]}}`
	appReq := map[string]interface{}{
		"app_id":           "gmail-app",
		"name":             "Test Gmail App",
		"service_type":     "gmail",
		"required_scopes":  "https://mail.google.com/",
		"credentials_json": json.RawMessage(credJSON),
	}
	appBody, _ := json.Marshal(appReq)
	resp, err := adminRequest("POST", ts.URL+"/v1/apps", appBody)
	if err != nil {
		t.Fatalf("POST /v1/apps: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /v1/apps: status %d, body: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Step 2: Insert user secret (simulating OAuth callback)
	tokenJSON, _ := json.Marshal(map[string]string{
		"refresh_token": "test-refresh-token-value",
	})
	encrypted, err := crypto.EncryptAtRest(masterKey, tokenJSON)
	if err != nil {
		t.Fatalf("EncryptAtRest: %v", err)
	}
	err = store.UpsertUserSecret(&db.UserSecret{
		AppID:           "gmail-app",
		UserID:          "user@example.com",
		SecretEncrypted: encrypted,
	})
	if err != nil {
		t.Fatalf("UpsertUserSecret: %v", err)
	}

	// Step 3: Register TEE instance (with admin token)
	var teePriv [32]byte
	rand.Read(teePriv[:])
	teePub, _ := curve25519.X25519(teePriv[:], curve25519.Basepoint)
	teePubHex := hex.EncodeToString(teePub)

	h := sha1.Sum(teePub)
	fid := hex.EncodeToString(h[:])

	instReq := map[string]string{
		"public_key":    teePubHex,
		"bound_app_id":  "gmail-app",
		"bound_user_id": "user@example.com",
		"label":         "test-tee",
	}
	instBody, _ := json.Marshal(instReq)
	resp, err = adminRequest("POST", ts.URL+"/v1/instances", instBody)
	if err != nil {
		t.Fatalf("POST /v1/instances: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /v1/instances: status %d, body: %s", resp.StatusCode, body)
	}
	var instResp map[string]string
	json.NewDecoder(resp.Body).Decode(&instResp)
	resp.Body.Close()
	if instResp["fid"] != fid {
		t.Fatalf("FID mismatch: got %q, want %q", instResp["fid"], fid)
	}

	// Step 4: Fetch secrets (no admin token needed)
	refs := []string{
		"jingui://gmail-app/user@example.com/client_id",
		"jingui://gmail-app/user@example.com/client_secret",
		"jingui://gmail-app/user@example.com/refresh_token",
	}
	fetchReq := map[string]interface{}{
		"fid":               fid,
		"secret_references": refs,
	}
	challengeID, challengeResponse := solveFetchChallenge(t, ts.URL, fid, teePriv)
	fetchReq["challenge_id"] = challengeID
	fetchReq["challenge_response"] = challengeResponse
	fetchBody, _ := json.Marshal(fetchReq)
	resp, err = http.Post(ts.URL+"/v1/secrets/fetch", "application/json", bytes.NewReader(fetchBody))
	if err != nil {
		t.Fatalf("POST /v1/secrets/fetch: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /v1/secrets/fetch: status %d, body: %s", resp.StatusCode, body)
	}

	var fetchResp struct {
		Secrets map[string]string `json:"secrets"`
	}
	json.NewDecoder(resp.Body).Decode(&fetchResp)
	resp.Body.Close()

	if len(fetchResp.Secrets) != 3 {
		t.Fatalf("expected 3 secrets, got %d", len(fetchResp.Secrets))
	}

	// Replay same challenge should fail (one-time challenge)
	replayResp, err := http.Post(ts.URL+"/v1/secrets/fetch", "application/json", bytes.NewReader(fetchBody))
	if err != nil {
		t.Fatalf("POST replay /v1/secrets/fetch: %v", err)
	}
	if replayResp.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(replayResp.Body)
		replayResp.Body.Close()
		t.Fatalf("expected 401 for challenge replay, got %d: %s", replayResp.StatusCode, body)
	}
	replayResp.Body.Close()

	// Step 5: Decrypt and verify
	expected := map[string]string{
		"jingui://gmail-app/user@example.com/client_id":     "test-client-id",
		"jingui://gmail-app/user@example.com/client_secret": "test-client-secret",
		"jingui://gmail-app/user@example.com/refresh_token": "test-refresh-token-value",
	}

	for ref, b64 := range fetchResp.Secrets {
		blob, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			t.Fatalf("base64 decode %s: %v", ref, err)
		}
		plain, err := crypto.Decrypt(teePriv, blob)
		if err != nil {
			t.Fatalf("Decrypt %s: %v", ref, err)
		}
		want, ok := expected[ref]
		if !ok {
			t.Fatalf("unexpected ref: %s", ref)
		}
		if string(plain) != want {
			t.Errorf("secret %s = %q, want %q", ref, plain, want)
		}
	}

	// Step 6: Access control — wrong app_id → 403
	badReq, _ := json.Marshal(map[string]interface{}{
		"fid":               fid,
		"secret_references": []string{"jingui://other-app/user@example.com/client_id"},
	})
	challengeID, challengeResponse = solveFetchChallenge(t, ts.URL, fid, teePriv)
	var badReqMap map[string]interface{}
	if err := json.Unmarshal(badReq, &badReqMap); err != nil {
		t.Fatalf("unmarshal badReq: %v", err)
	}
	badReqMap["challenge_id"] = challengeID
	badReqMap["challenge_response"] = challengeResponse
	badReq, _ = json.Marshal(badReqMap)
	resp, _ = http.Post(ts.URL+"/v1/secrets/fetch", "application/json", bytes.NewReader(badReq))
	if resp.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 403 for wrong app_id, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Step 7: Access control — wrong user_id → 403
	badReq2, _ := json.Marshal(map[string]interface{}{
		"fid":               fid,
		"secret_references": []string{"jingui://gmail-app/other@example.com/client_id"},
	})
	challengeID, challengeResponse = solveFetchChallenge(t, ts.URL, fid, teePriv)
	var badReqMap2 map[string]interface{}
	if err := json.Unmarshal(badReq2, &badReqMap2); err != nil {
		t.Fatalf("unmarshal badReq2: %v", err)
	}
	badReqMap2["challenge_id"] = challengeID
	badReqMap2["challenge_response"] = challengeResponse
	badReq2, _ = json.Marshal(badReqMap2)
	resp, _ = http.Post(ts.URL+"/v1/secrets/fetch", "application/json", bytes.NewReader(badReq2))
	if resp.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 403 for wrong user_id, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	fmt.Println("Integration test: all secrets resolved, encrypted, decrypted, and access control verified")
}

func TestAdminAuth_Rejected(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	// POST /v1/apps without token → 401
	appBody, _ := json.Marshal(map[string]interface{}{
		"app_id": "x", "name": "x", "service_type": "x",
		"credentials_json": json.RawMessage(`{"installed":{"client_id":"a","client_secret":"b"}}`),
	})
	resp, err := http.Post(ts.URL+"/v1/apps", "application/json", bytes.NewReader(appBody))
	if err != nil {
		t.Fatalf("POST /v1/apps: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("POST /v1/apps without token: expected 401, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// POST /v1/instances without token → 401
	instBody, _ := json.Marshal(map[string]string{
		"public_key": "aa", "bound_app_id": "x", "bound_user_id": "x",
	})
	resp, err = http.Post(ts.URL+"/v1/instances", "application/json", bytes.NewReader(instBody))
	if err != nil {
		t.Fatalf("POST /v1/instances: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("POST /v1/instances without token: expected 401, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// POST /v1/apps with wrong token → 401
	req, _ := http.NewRequest("POST", ts.URL+"/v1/apps", bytes.NewReader(appBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer wrong-token-here-1234")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /v1/apps wrong token: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("POST /v1/apps wrong token: expected 401, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// POST /v1/secrets/fetch without token → still works (not admin-protected)
	fetchBody, _ := json.Marshal(map[string]interface{}{
		"fid": "nonexistent", "secret_references": []string{},
	})
	resp, err = http.Post(ts.URL+"/v1/secrets/fetch", "application/json", bytes.NewReader(fetchBody))
	if err != nil {
		t.Fatalf("POST /v1/secrets/fetch: %v", err)
	}
	// Should not be 401 (it may be 400 or 404 depending on validation, but not 401)
	if resp.StatusCode == http.StatusUnauthorized {
		t.Error("POST /v1/secrets/fetch should not require admin auth")
	}
	resp.Body.Close()
}
