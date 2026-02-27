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
		"vault":            "gmail-app",
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

	// Step 2: Insert vault item (simulating OAuth callback)
	tokenJSON, _ := json.Marshal(map[string]string{
		"refresh_token": "test-refresh-token-value",
	})
	encrypted, err := crypto.EncryptAtRest(masterKey, tokenJSON)
	if err != nil {
		t.Fatalf("EncryptAtRest: %v", err)
	}
	err = store.UpsertVaultItem(&db.VaultItem{
		Vault:           "gmail-app",
		Item:            "user@example.com",
		SecretEncrypted: encrypted,
	})
	if err != nil {
		t.Fatalf("UpsertVaultItem: %v", err)
	}

	// Step 3: Register TEE instance (with admin token)
	var teePriv [32]byte
	rand.Read(teePriv[:])
	teePub, _ := curve25519.X25519(teePriv[:], curve25519.Basepoint)
	teePubHex := hex.EncodeToString(teePub)

	h := sha1.Sum(teePub)
	fid := hex.EncodeToString(h[:])

	instReq := map[string]string{
		"public_key":               teePubHex,
		"bound_vault":              "gmail-app",
		"bound_attestation_app_id": "gmail-app",
		"bound_item":               "user@example.com",
		"label":                    "test-tee",
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

	// Step 6: Access control — wrong vault → 403
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
		t.Errorf("expected 403 for wrong vault, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Step 7: Access control — wrong item → 403
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
		t.Errorf("expected 403 for wrong item, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	fmt.Println("Integration test: all secrets resolved, encrypted, decrypted, and access control verified")
}

// --- Admin CRUD endpoint tests ---

func TestListApps(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	// Empty list
	resp, err := adminRequest("GET", ts.URL+"/v1/apps", nil)
	if err != nil {
		t.Fatalf("GET /v1/apps: %v", err)
	}
	var apps []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&apps)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(apps) != 0 {
		t.Fatalf("expected 0 apps, got %d", len(apps))
	}

	// Create an app then list
	credJSON := `{"installed":{"client_id":"cid","client_secret":"cs"}}`
	appReq, _ := json.Marshal(map[string]interface{}{
		"vault": "app1", "name": "App 1", "service_type": "gmail",
		"credentials_json": json.RawMessage(credJSON),
	})
	resp, _ = adminRequest("POST", ts.URL+"/v1/apps", appReq)
	resp.Body.Close()

	resp, _ = adminRequest("GET", ts.URL+"/v1/apps", nil)
	json.NewDecoder(resp.Body).Decode(&apps)
	resp.Body.Close()
	if len(apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(apps))
	}
	if apps[0]["vault"] != "app1" {
		t.Errorf("expected app_id=app1, got %v", apps[0]["vault"])
	}
}

func TestGetApp(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	// 404
	resp, _ := adminRequest("GET", ts.URL+"/v1/apps/nonexistent", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Create then get
	credJSON := `{"installed":{"client_id":"cid","client_secret":"cs"}}`
	appReq, _ := json.Marshal(map[string]interface{}{
		"vault": "app1", "name": "App 1", "service_type": "gmail",
		"credentials_json": json.RawMessage(credJSON),
	})
	resp, _ = adminRequest("POST", ts.URL+"/v1/apps", appReq)
	resp.Body.Close()

	resp, _ = adminRequest("GET", ts.URL+"/v1/apps/app1", nil)
	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if body["vault"] != "app1" {
		t.Errorf("expected app_id=app1, got %v", body["vault"])
	}
	if body["has_credentials"] != true {
		t.Errorf("expected has_credentials=true, got %v", body["has_credentials"])
	}
	// Should NOT contain credentials_encrypted
	if _, ok := body["credentials_encrypted"]; ok {
		t.Error("response should not contain credentials_encrypted")
	}
}

func TestDeleteApp_HTTP(t *testing.T) {
	ts, store, masterKey := setupTestServer(t)

	// 404
	resp, _ := adminRequest("DELETE", ts.URL+"/v1/apps/nonexistent", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Create app with dependent item → 409 without cascade
	credJSON := `{"installed":{"client_id":"cid","client_secret":"cs"}}`
	appReq, _ := json.Marshal(map[string]interface{}{
		"vault": "app1", "name": "App 1", "service_type": "gmail",
		"credentials_json": json.RawMessage(credJSON),
	})
	resp, _ = adminRequest("POST", ts.URL+"/v1/apps", appReq)
	resp.Body.Close()

	encrypted, _ := crypto.EncryptAtRest(masterKey, []byte(`{"refresh_token":"tok"}`))
	store.UpsertVaultItem(&db.VaultItem{
		Vault: "app1", Item: "u@example.com", SecretEncrypted: encrypted,
	})

	resp, _ = adminRequest("DELETE", ts.URL+"/v1/apps/app1", nil)
	if resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 409, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Cascade delete → 200
	resp, _ = adminRequest("DELETE", ts.URL+"/v1/apps/app1?cascade=true", nil)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
	var delBody map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&delBody)
	resp.Body.Close()
	if delBody["status"] != "deleted" {
		t.Errorf("expected status=deleted, got %v", delBody["status"])
	}

	// Verify app is gone
	resp, _ = adminRequest("GET", ts.URL+"/v1/apps/app1", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestDeleteApp_Simple(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	credJSON := `{"installed":{"client_id":"cid","client_secret":"cs"}}`
	appReq, _ := json.Marshal(map[string]interface{}{
		"vault": "app1", "name": "App 1", "service_type": "gmail",
		"credentials_json": json.RawMessage(credJSON),
	})
	resp, _ := adminRequest("POST", ts.URL+"/v1/apps", appReq)
	resp.Body.Close()

	// Simple delete (no dependents) → 200
	resp, _ = adminRequest("DELETE", ts.URL+"/v1/apps/app1", nil)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()
}

func TestListInstances_HTTP(t *testing.T) {
	ts, store, masterKey := setupTestServer(t)

	// Empty list
	resp, _ := adminRequest("GET", ts.URL+"/v1/instances", nil)
	var instances []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&instances)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(instances) != 0 {
		t.Fatalf("expected 0 instances, got %d", len(instances))
	}

	// Create app + item + instance, then list
	credJSON := `{"installed":{"client_id":"cid","client_secret":"cs"}}`
	appReq, _ := json.Marshal(map[string]interface{}{
		"vault": "app1", "name": "App 1", "service_type": "gmail",
		"credentials_json": json.RawMessage(credJSON),
	})
	resp, _ = adminRequest("POST", ts.URL+"/v1/apps", appReq)
	resp.Body.Close()

	encrypted, _ := crypto.EncryptAtRest(masterKey, []byte(`{"refresh_token":"tok"}`))
	store.UpsertVaultItem(&db.VaultItem{
		Vault: "app1", Item: "u@example.com", SecretEncrypted: encrypted,
	})

	var teePriv [32]byte
	rand.Read(teePriv[:])
	teePub, _ := curve25519.X25519(teePriv[:], curve25519.Basepoint)
	instReq, _ := json.Marshal(map[string]string{
		"public_key": hex.EncodeToString(teePub), "bound_vault": "app1",
		"bound_attestation_app_id": "app1",
		"bound_item":               "u@example.com", "label": "test",
	})
	resp, _ = adminRequest("POST", ts.URL+"/v1/instances", instReq)
	resp.Body.Close()

	resp, _ = adminRequest("GET", ts.URL+"/v1/instances", nil)
	json.NewDecoder(resp.Body).Decode(&instances)
	resp.Body.Close()
	if len(instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(instances))
	}
	// public_key should be hex string, not base64
	pk, ok := instances[0]["public_key"].(string)
	if !ok || len(pk) != 64 {
		t.Errorf("expected 64-char hex public_key, got %q", pk)
	}
}

func TestGetInstance_HTTP(t *testing.T) {
	ts, store, masterKey := setupTestServer(t)

	// 404
	resp, _ := adminRequest("GET", ts.URL+"/v1/instances/nonexistent", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Create and get
	credJSON := `{"installed":{"client_id":"cid","client_secret":"cs"}}`
	appReq, _ := json.Marshal(map[string]interface{}{
		"vault": "app1", "name": "App 1", "service_type": "gmail",
		"credentials_json": json.RawMessage(credJSON),
	})
	resp, _ = adminRequest("POST", ts.URL+"/v1/apps", appReq)
	resp.Body.Close()

	encrypted, _ := crypto.EncryptAtRest(masterKey, []byte(`{"refresh_token":"tok"}`))
	store.UpsertVaultItem(&db.VaultItem{
		Vault: "app1", Item: "u@example.com", SecretEncrypted: encrypted,
	})

	var teePriv [32]byte
	rand.Read(teePriv[:])
	teePub, _ := curve25519.X25519(teePriv[:], curve25519.Basepoint)
	instReq, _ := json.Marshal(map[string]string{
		"public_key": hex.EncodeToString(teePub), "bound_vault": "app1",
		"bound_attestation_app_id": "app1",
		"bound_item":               "u@example.com",
	})
	resp, _ = adminRequest("POST", ts.URL+"/v1/instances", instReq)
	var instResp map[string]string
	json.NewDecoder(resp.Body).Decode(&instResp)
	resp.Body.Close()
	fid := instResp["fid"]

	resp, _ = adminRequest("GET", ts.URL+"/v1/instances/"+fid, nil)
	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if body["fid"] != fid {
		t.Errorf("expected fid=%s, got %v", fid, body["fid"])
	}
	if body["public_key"] != hex.EncodeToString(teePub) {
		t.Error("public_key should be hex-encoded")
	}
}

func TestDeleteInstance_HTTP(t *testing.T) {
	ts, store, masterKey := setupTestServer(t)

	// 404
	resp, _ := adminRequest("DELETE", ts.URL+"/v1/instances/nonexistent", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Create and delete
	credJSON := `{"installed":{"client_id":"cid","client_secret":"cs"}}`
	appReq, _ := json.Marshal(map[string]interface{}{
		"vault": "app1", "name": "App 1", "service_type": "gmail",
		"credentials_json": json.RawMessage(credJSON),
	})
	resp, _ = adminRequest("POST", ts.URL+"/v1/apps", appReq)
	resp.Body.Close()

	encrypted, _ := crypto.EncryptAtRest(masterKey, []byte(`{"refresh_token":"tok"}`))
	store.UpsertVaultItem(&db.VaultItem{
		Vault: "app1", Item: "u@example.com", SecretEncrypted: encrypted,
	})

	var teePriv [32]byte
	rand.Read(teePriv[:])
	teePub, _ := curve25519.X25519(teePriv[:], curve25519.Basepoint)
	instReq, _ := json.Marshal(map[string]string{
		"public_key": hex.EncodeToString(teePub), "bound_vault": "app1",
		"bound_attestation_app_id": "app1",
		"bound_item":               "u@example.com",
	})
	resp, _ = adminRequest("POST", ts.URL+"/v1/instances", instReq)
	var instResp map[string]string
	json.NewDecoder(resp.Body).Decode(&instResp)
	resp.Body.Close()
	fid := instResp["fid"]

	resp, _ = adminRequest("DELETE", ts.URL+"/v1/instances/"+fid, nil)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
	var delBody map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&delBody)
	resp.Body.Close()
	if delBody["status"] != "deleted" {
		t.Errorf("expected status=deleted, got %v", delBody["status"])
	}

	// Verify gone
	resp, _ = adminRequest("GET", ts.URL+"/v1/instances/"+fid, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestListSecrets_HTTP(t *testing.T) {
	ts, store, masterKey := setupTestServer(t)

	// Empty list
	resp, _ := adminRequest("GET", ts.URL+"/v1/secrets", nil)
	var secrets []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&secrets)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(secrets) != 0 {
		t.Fatalf("expected 0 secrets, got %d", len(secrets))
	}

	// Create app + items
	credJSON := `{"installed":{"client_id":"cid","client_secret":"cs"}}`
	for _, appID := range []string{"app1", "app2"} {
		appReq, _ := json.Marshal(map[string]interface{}{
			"vault": appID, "name": appID, "service_type": "gmail",
			"credentials_json": json.RawMessage(credJSON),
		})
		resp, _ = adminRequest("POST", ts.URL+"/v1/apps", appReq)
		resp.Body.Close()
	}

	encrypted, _ := crypto.EncryptAtRest(masterKey, []byte(`{"refresh_token":"tok"}`))
	store.UpsertVaultItem(&db.VaultItem{Vault: "app1", Item: "u1@example.com", SecretEncrypted: encrypted})
	store.UpsertVaultItem(&db.VaultItem{Vault: "app1", Item: "u2@example.com", SecretEncrypted: encrypted})
	store.UpsertVaultItem(&db.VaultItem{Vault: "app2", Item: "u1@example.com", SecretEncrypted: encrypted})

	// List all
	resp, _ = adminRequest("GET", ts.URL+"/v1/secrets", nil)
	json.NewDecoder(resp.Body).Decode(&secrets)
	resp.Body.Close()
	if len(secrets) != 3 {
		t.Fatalf("expected 3 secrets, got %d", len(secrets))
	}
	// Should NOT contain secret_encrypted
	for _, s := range secrets {
		if _, ok := s["secret_encrypted"]; ok {
			t.Error("response should not contain secret_encrypted")
		}
	}

	// Filter by vault
	resp, _ = adminRequest("GET", ts.URL+"/v1/secrets?vault=app1", nil)
	json.NewDecoder(resp.Body).Decode(&secrets)
	resp.Body.Close()
	if len(secrets) != 2 {
		t.Fatalf("expected 2 secrets for app1, got %d", len(secrets))
	}

	resp, _ = adminRequest("GET", ts.URL+"/v1/secrets?vault=app2", nil)
	json.NewDecoder(resp.Body).Decode(&secrets)
	resp.Body.Close()
	if len(secrets) != 1 {
		t.Fatalf("expected 1 secret for app2, got %d", len(secrets))
	}
}

func TestGetSecret_HTTP(t *testing.T) {
	ts, store, masterKey := setupTestServer(t)

	// 404
	resp, _ := adminRequest("GET", ts.URL+"/v1/secrets/app1/user1", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Create and get
	credJSON := `{"installed":{"client_id":"cid","client_secret":"cs"}}`
	appReq, _ := json.Marshal(map[string]interface{}{
		"vault": "app1", "name": "App 1", "service_type": "gmail",
		"credentials_json": json.RawMessage(credJSON),
	})
	resp, _ = adminRequest("POST", ts.URL+"/v1/apps", appReq)
	resp.Body.Close()

	encrypted, _ := crypto.EncryptAtRest(masterKey, []byte(`{"refresh_token":"tok"}`))
	store.UpsertVaultItem(&db.VaultItem{
		Vault: "app1", Item: "u@example.com", SecretEncrypted: encrypted,
	})

	resp, _ = adminRequest("GET", ts.URL+"/v1/secrets/app1/u@example.com", nil)
	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if body["vault"] != "app1" {
		t.Errorf("expected vault=app1, got %v", body["vault"])
	}
	if body["has_secret"] != true {
		t.Errorf("expected has_secret=true, got %v", body["has_secret"])
	}
	if _, ok := body["secret_encrypted"]; ok {
		t.Error("response should not contain secret_encrypted")
	}
}

func TestDeleteSecret_HTTP(t *testing.T) {
	ts, store, masterKey := setupTestServer(t)

	// 404
	resp, _ := adminRequest("DELETE", ts.URL+"/v1/secrets/app1/user1", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Create app + item + instance → 409 without cascade
	credJSON := `{"installed":{"client_id":"cid","client_secret":"cs"}}`
	appReq, _ := json.Marshal(map[string]interface{}{
		"vault": "app1", "name": "App 1", "service_type": "gmail",
		"credentials_json": json.RawMessage(credJSON),
	})
	resp, _ = adminRequest("POST", ts.URL+"/v1/apps", appReq)
	resp.Body.Close()

	encrypted, _ := crypto.EncryptAtRest(masterKey, []byte(`{"refresh_token":"tok"}`))
	store.UpsertVaultItem(&db.VaultItem{
		Vault: "app1", Item: "u@example.com", SecretEncrypted: encrypted,
	})

	var teePriv [32]byte
	rand.Read(teePriv[:])
	teePub, _ := curve25519.X25519(teePriv[:], curve25519.Basepoint)
	instReq, _ := json.Marshal(map[string]string{
		"public_key": hex.EncodeToString(teePub), "bound_vault": "app1",
		"bound_attestation_app_id": "app1",
		"bound_item":               "u@example.com",
	})
	resp, _ = adminRequest("POST", ts.URL+"/v1/instances", instReq)
	resp.Body.Close()

	resp, _ = adminRequest("DELETE", ts.URL+"/v1/secrets/app1/u@example.com", nil)
	if resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 409, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Cascade → 200
	resp, _ = adminRequest("DELETE", ts.URL+"/v1/secrets/app1/u@example.com?cascade=true", nil)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
	var delBody map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&delBody)
	resp.Body.Close()
	if delBody["status"] != "deleted" {
		t.Errorf("expected status=deleted, got %v", delBody["status"])
	}

	// Verify gone
	resp, _ = adminRequest("GET", ts.URL+"/v1/secrets/app1/u@example.com", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestDeleteSecret_Simple(t *testing.T) {
	ts, store, masterKey := setupTestServer(t)

	credJSON := `{"installed":{"client_id":"cid","client_secret":"cs"}}`
	appReq, _ := json.Marshal(map[string]interface{}{
		"vault": "app1", "name": "App 1", "service_type": "gmail",
		"credentials_json": json.RawMessage(credJSON),
	})
	resp, _ := adminRequest("POST", ts.URL+"/v1/apps", appReq)
	resp.Body.Close()

	encrypted, _ := crypto.EncryptAtRest(masterKey, []byte(`{"refresh_token":"tok"}`))
	store.UpsertVaultItem(&db.VaultItem{
		Vault: "app1", Item: "u@example.com", SecretEncrypted: encrypted,
	})

	// No dependents → simple delete works
	resp, _ = adminRequest("DELETE", ts.URL+"/v1/secrets/app1/u@example.com", nil)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()
}

func TestAdminCRUD_RequiresAuth(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/v1/apps"},
		{"GET", "/v1/apps/test"},
		{"DELETE", "/v1/apps/test"},
		{"GET", "/v1/instances"},
		{"GET", "/v1/instances/test"},
		{"DELETE", "/v1/instances/test"},
		{"GET", "/v1/secrets"},
		{"GET", "/v1/secrets/app/user"},
		{"DELETE", "/v1/secrets/app/user"},
	}

	for _, ep := range endpoints {
		req, _ := http.NewRequest(ep.method, ts.URL+ep.path, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", ep.method, ep.path, err)
		}
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s without auth: expected 401, got %d", ep.method, ep.path, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestAdminAuth_Rejected(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	// POST /v1/apps without token → 401
	appBody, _ := json.Marshal(map[string]interface{}{
		"vault": "x", "name": "x", "service_type": "x",
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
		"public_key": "aa", "bound_vault": "x", "bound_attestation_app_id": "x", "bound_item": "x",
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
