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

func setupTestServer(t *testing.T) (*httptest.Server, *db.Store) {
	t.Helper()

	store, err := db.NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	cfg := &server.Config{
		AdminToken: testAdminToken,
	}

	router := server.NewRouter(store, cfg)
	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)

	return ts, store
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
	ts, store := setupTestServer(t)

	// Step 1: Create a vault
	vaultReq, _ := json.Marshal(map[string]string{
		"id":   "gmail-vault",
		"name": "Gmail Vault",
	})
	resp, err := adminRequest("POST", ts.URL+"/v1/vaults", vaultReq)
	if err != nil {
		t.Fatalf("POST /v1/vaults: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /v1/vaults: status %d, body: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Step 2: Insert vault items (plaintext fields)
	err = store.SetItemFields("gmail-vault", "alice@gmail.com", map[string]string{
		"password":      "test-password-value",
		"refresh_token": "test-refresh-token-value",
		"api_key":       "test-api-key-value",
	})
	if err != nil {
		t.Fatalf("SetItemFields: %v", err)
	}

	// Step 3: Register TEE instance
	var teePriv [32]byte
	rand.Read(teePriv[:])
	teePub, _ := curve25519.X25519(teePriv[:], curve25519.Basepoint)
	teePubHex := hex.EncodeToString(teePub)

	h := sha1.Sum(teePub)
	fid := hex.EncodeToString(h[:])

	instReq, _ := json.Marshal(map[string]string{
		"public_key":    teePubHex,
		"dstack_app_id": "dstack-app-1",
		"label":         "test-tee",
	})
	resp, err = adminRequest("POST", ts.URL+"/v1/instances", instReq)
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

	// Step 4: Grant vault access to instance
	resp, err = adminRequest("POST", ts.URL+"/v1/vaults/gmail-vault/instances/"+fid, nil)
	if err != nil {
		t.Fatalf("POST grant access: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("grant access: status %d, body: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Step 5: Fetch secrets (no admin token needed)
	refs := []string{
		"jingui://gmail-vault/alice@gmail.com/password",
		"jingui://gmail-vault/alice@gmail.com/refresh_token",
		"jingui://gmail-vault/alice@gmail.com/api_key",
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
		t.Fatalf("POST replay: %v", err)
	}
	if replayResp.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(replayResp.Body)
		replayResp.Body.Close()
		t.Fatalf("expected 401 for replay, got %d: %s", replayResp.StatusCode, body)
	}
	replayResp.Body.Close()

	// Step 6: Decrypt and verify
	expected := map[string]string{
		"jingui://gmail-vault/alice@gmail.com/password":      "test-password-value",
		"jingui://gmail-vault/alice@gmail.com/refresh_token": "test-refresh-token-value",
		"jingui://gmail-vault/alice@gmail.com/api_key":       "test-api-key-value",
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

	// Step 7: Access control — wrong vault → 403
	badReq := map[string]interface{}{
		"fid":               fid,
		"secret_references": []string{"jingui://other-vault/alice@gmail.com/password"},
	}
	challengeID, challengeResponse = solveFetchChallenge(t, ts.URL, fid, teePriv)
	badReq["challenge_id"] = challengeID
	badReq["challenge_response"] = challengeResponse
	badBody, _ := json.Marshal(badReq)
	resp, _ = http.Post(ts.URL+"/v1/secrets/fetch", "application/json", bytes.NewReader(badBody))
	if resp.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 403 for wrong vault, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Step 8: Access control — missing field → 404
	badReq2 := map[string]interface{}{
		"fid":               fid,
		"secret_references": []string{"jingui://gmail-vault/alice@gmail.com/nonexistent_field"},
	}
	challengeID, challengeResponse = solveFetchChallenge(t, ts.URL, fid, teePriv)
	badReq2["challenge_id"] = challengeID
	badReq2["challenge_response"] = challengeResponse
	badBody2, _ := json.Marshal(badReq2)
	resp, _ = http.Post(ts.URL+"/v1/secrets/fetch", "application/json", bytes.NewReader(badBody2))
	if resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 404 for missing field, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	fmt.Println("Integration test: all secrets resolved, encrypted, decrypted, and access control verified")
}

// --- Admin CRUD endpoint tests ---

func TestListVaults_HTTP(t *testing.T) {
	ts, _ := setupTestServer(t)

	// Empty list
	resp, _ := adminRequest("GET", ts.URL+"/v1/vaults", nil)
	var vaults []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&vaults)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(vaults) != 0 {
		t.Fatalf("expected 0, got %d", len(vaults))
	}

	// Create then list
	vaultReq, _ := json.Marshal(map[string]string{"id": "v1", "name": "V1"})
	resp, _ = adminRequest("POST", ts.URL+"/v1/vaults", vaultReq)
	resp.Body.Close()

	resp, _ = adminRequest("GET", ts.URL+"/v1/vaults", nil)
	json.NewDecoder(resp.Body).Decode(&vaults)
	resp.Body.Close()
	if len(vaults) != 1 {
		t.Fatalf("expected 1, got %d", len(vaults))
	}
	if vaults[0]["id"] != "v1" {
		t.Errorf("expected id=v1, got %v", vaults[0]["id"])
	}
}

func TestGetVault_HTTP(t *testing.T) {
	ts, _ := setupTestServer(t)

	// 404
	resp, _ := adminRequest("GET", ts.URL+"/v1/vaults/nonexistent", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Create then get
	vaultReq, _ := json.Marshal(map[string]string{"id": "v1", "name": "V1"})
	resp, _ = adminRequest("POST", ts.URL+"/v1/vaults", vaultReq)
	resp.Body.Close()

	resp, _ = adminRequest("GET", ts.URL+"/v1/vaults/v1", nil)
	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if body["id"] != "v1" {
		t.Errorf("expected id=v1, got %v", body["id"])
	}
}

func TestDeleteVault_HTTP(t *testing.T) {
	ts, store := setupTestServer(t)

	// 404
	resp, _ := adminRequest("DELETE", ts.URL+"/v1/vaults/nonexistent", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Create vault with items → 409 without cascade
	vaultReq, _ := json.Marshal(map[string]string{"id": "v1", "name": "V1"})
	resp, _ = adminRequest("POST", ts.URL+"/v1/vaults", vaultReq)
	resp.Body.Close()

	store.SetItemFields("v1", "item1", map[string]string{"k": "v"})

	resp, _ = adminRequest("DELETE", ts.URL+"/v1/vaults/v1", nil)
	if resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 409, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Cascade delete → 200
	resp, _ = adminRequest("DELETE", ts.URL+"/v1/vaults/v1?cascade=true", nil)
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
	resp, _ = adminRequest("GET", ts.URL+"/v1/vaults/v1", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestDeleteVault_Simple(t *testing.T) {
	ts, _ := setupTestServer(t)

	vaultReq, _ := json.Marshal(map[string]string{"id": "v1", "name": "V1"})
	resp, _ := adminRequest("POST", ts.URL+"/v1/vaults", vaultReq)
	resp.Body.Close()

	resp, _ = adminRequest("DELETE", ts.URL+"/v1/vaults/v1", nil)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()
}

func TestVaultItems_HTTP(t *testing.T) {
	ts, _ := setupTestServer(t)

	// Create vault
	vaultReq, _ := json.Marshal(map[string]string{"id": "v1", "name": "V1"})
	resp, _ := adminRequest("POST", ts.URL+"/v1/vaults", vaultReq)
	resp.Body.Close()

	// List items — empty
	resp, _ = adminRequest("GET", ts.URL+"/v1/vaults/v1/items", nil)
	var sections []string
	json.NewDecoder(resp.Body).Decode(&sections)
	resp.Body.Close()
	if len(sections) != 0 {
		t.Fatalf("expected 0 sections, got %d", len(sections))
	}

	// Put item
	putReq, _ := json.Marshal(map[string]interface{}{
		"fields": map[string]string{"password": "secret", "api_key": "key123"},
	})
	resp, _ = adminRequest("PUT", ts.URL+"/v1/vaults/v1/items/alice@gmail.com", putReq)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("PUT item: expected 200, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// List items — 1 section
	resp, _ = adminRequest("GET", ts.URL+"/v1/vaults/v1/items", nil)
	json.NewDecoder(resp.Body).Decode(&sections)
	resp.Body.Close()
	if len(sections) != 1 || sections[0] != "alice@gmail.com" {
		t.Fatalf("expected [alice@gmail.com], got %v", sections)
	}

	// Get item
	resp, _ = adminRequest("GET", ts.URL+"/v1/vaults/v1/items/alice@gmail.com", nil)
	var itemBody map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&itemBody)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET item: expected 200, got %d", resp.StatusCode)
	}
	rawKeys, ok := itemBody["keys"].([]interface{})
	if !ok {
		t.Fatalf("expected keys array, got %v", itemBody["keys"])
	}
	keySet := make(map[string]bool, len(rawKeys))
	for _, k := range rawKeys {
		keySet[k.(string)] = true
	}
	if !keySet["password"] || !keySet["api_key"] {
		t.Errorf("expected keys to contain password and api_key, got %v", rawKeys)
	}

	// Delete item
	resp, _ = adminRequest("DELETE", ts.URL+"/v1/vaults/v1/items/alice@gmail.com", nil)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("DELETE item: expected 200, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Delete nonexistent → 404
	resp, _ = adminRequest("DELETE", ts.URL+"/v1/vaults/v1/items/nonexistent", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestListInstances_HTTP(t *testing.T) {
	ts, _ := setupTestServer(t)

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

	// Register instance
	var teePriv [32]byte
	rand.Read(teePriv[:])
	teePub, _ := curve25519.X25519(teePriv[:], curve25519.Basepoint)
	instReq, _ := json.Marshal(map[string]string{
		"public_key":    hex.EncodeToString(teePub),
		"dstack_app_id": "app1",
		"label":         "test",
	})
	resp, _ = adminRequest("POST", ts.URL+"/v1/instances", instReq)
	resp.Body.Close()

	resp, _ = adminRequest("GET", ts.URL+"/v1/instances", nil)
	json.NewDecoder(resp.Body).Decode(&instances)
	resp.Body.Close()
	if len(instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(instances))
	}
	pk, ok := instances[0]["public_key"].(string)
	if !ok || len(pk) != 64 {
		t.Errorf("expected 64-char hex public_key, got %q", pk)
	}
}

func TestGetInstance_HTTP(t *testing.T) {
	ts, _ := setupTestServer(t)

	// 404
	resp, _ := adminRequest("GET", ts.URL+"/v1/instances/nonexistent", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Create and get
	var teePriv [32]byte
	rand.Read(teePriv[:])
	teePub, _ := curve25519.X25519(teePriv[:], curve25519.Basepoint)
	instReq, _ := json.Marshal(map[string]string{
		"public_key":    hex.EncodeToString(teePub),
		"dstack_app_id": "app1",
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
	ts, _ := setupTestServer(t)

	// 404
	resp, _ := adminRequest("DELETE", ts.URL+"/v1/instances/nonexistent", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Create and delete
	var teePriv [32]byte
	rand.Read(teePriv[:])
	teePub, _ := curve25519.X25519(teePriv[:], curve25519.Basepoint)
	instReq, _ := json.Marshal(map[string]string{
		"public_key":    hex.EncodeToString(teePub),
		"dstack_app_id": "app1",
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

	resp, _ = adminRequest("GET", ts.URL+"/v1/instances/"+fid, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestVaultInstanceAccess_HTTP(t *testing.T) {
	ts, _ := setupTestServer(t)

	// Create vault and instance
	vaultReq, _ := json.Marshal(map[string]string{"id": "v1", "name": "V1"})
	resp, _ := adminRequest("POST", ts.URL+"/v1/vaults", vaultReq)
	resp.Body.Close()

	var teePriv [32]byte
	rand.Read(teePriv[:])
	teePub, _ := curve25519.X25519(teePriv[:], curve25519.Basepoint)
	instReq, _ := json.Marshal(map[string]string{
		"public_key":    hex.EncodeToString(teePub),
		"dstack_app_id": "app1",
	})
	resp, _ = adminRequest("POST", ts.URL+"/v1/instances", instReq)
	var instResp map[string]string
	json.NewDecoder(resp.Body).Decode(&instResp)
	resp.Body.Close()
	fid := instResp["fid"]

	// List vault instances — empty
	resp, _ = adminRequest("GET", ts.URL+"/v1/vaults/v1/instances", nil)
	var vaultInstances []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&vaultInstances)
	resp.Body.Close()
	if len(vaultInstances) != 0 {
		t.Fatalf("expected 0, got %d", len(vaultInstances))
	}

	// Grant access
	resp, _ = adminRequest("POST", ts.URL+"/v1/vaults/v1/instances/"+fid, nil)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("grant: expected 200, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// List vault instances — 1
	resp, _ = adminRequest("GET", ts.URL+"/v1/vaults/v1/instances", nil)
	json.NewDecoder(resp.Body).Decode(&vaultInstances)
	resp.Body.Close()
	if len(vaultInstances) != 1 {
		t.Fatalf("expected 1, got %d", len(vaultInstances))
	}

	// Revoke access
	resp, _ = adminRequest("DELETE", ts.URL+"/v1/vaults/v1/instances/"+fid, nil)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("revoke: expected 200, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// List vault instances — empty again
	resp, _ = adminRequest("GET", ts.URL+"/v1/vaults/v1/instances", nil)
	json.NewDecoder(resp.Body).Decode(&vaultInstances)
	resp.Body.Close()
	if len(vaultInstances) != 0 {
		t.Fatalf("expected 0 after revoke, got %d", len(vaultInstances))
	}
}

func TestAdminCRUD_RequiresAuth(t *testing.T) {
	ts, _ := setupTestServer(t)

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/v1/vaults"},
		{"GET", "/v1/vaults/test"},
		{"DELETE", "/v1/vaults/test"},
		{"GET", "/v1/instances"},
		{"GET", "/v1/instances/test"},
		{"DELETE", "/v1/instances/test"},
		{"GET", "/v1/vaults/test/items"},
		{"GET", "/v1/vaults/test/instances"},
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
	ts, _ := setupTestServer(t)

	// POST /v1/vaults without token → 401
	vaultBody, _ := json.Marshal(map[string]string{"id": "x", "name": "x"})
	resp, err := http.Post(ts.URL+"/v1/vaults", "application/json", bytes.NewReader(vaultBody))
	if err != nil {
		t.Fatalf("POST /v1/vaults: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("POST /v1/vaults without token: expected 401, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// POST /v1/instances without token → 401
	instBody, _ := json.Marshal(map[string]string{
		"public_key": "aa", "dstack_app_id": "x",
	})
	resp, err = http.Post(ts.URL+"/v1/instances", "application/json", bytes.NewReader(instBody))
	if err != nil {
		t.Fatalf("POST /v1/instances: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("POST /v1/instances without token: expected 401, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// POST /v1/vaults with wrong token → 401
	req, _ := http.NewRequest("POST", ts.URL+"/v1/vaults", bytes.NewReader(vaultBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer wrong-token-here-1234")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /v1/vaults wrong token: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("POST /v1/vaults wrong token: expected 401, got %d", resp.StatusCode)
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
	if resp.StatusCode == http.StatusUnauthorized {
		t.Error("POST /v1/secrets/fetch should not require admin auth")
	}
	resp.Body.Close()
}
