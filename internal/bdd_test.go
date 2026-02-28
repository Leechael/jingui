//go:build bdd

package internal

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/aspect-build/jingui/internal/crypto"
	"github.com/aspect-build/jingui/internal/server"
	"github.com/aspect-build/jingui/internal/server/db"
	"github.com/cucumber/godog"
	"golang.org/x/crypto/curve25519"
)

// bddContext holds per-scenario state.
type bddContext struct {
	ts    *httptest.Server
	store *db.Store

	// TEE instance state
	teePriv [32]byte
	fid     string

	// last HTTP response
	lastStatus int
	lastBody   []byte

	// challenge flow state
	challengeID       string
	challengeResponse string

	// fetched secrets (encrypted)
	fetchedSecrets map[string]string
}

func (b *bddContext) reset() {
	if b.ts != nil {
		b.ts.Close()
	}
	if b.store != nil {
		b.store.Close()
	}
	*b = bddContext{}
}

// ── Given steps ─────────────────────────────────────────────────────

func (b *bddContext) theServerIsRunning() error {
	if b.ts != nil {
		return nil // already running
	}

	store, err := db.NewStore(":memory:")
	if err != nil {
		return fmt.Errorf("NewStore: %w", err)
	}

	cfg := &server.Config{
		AdminToken: testAdminToken,
	}

	router := server.NewRouter(store, cfg)
	ts := httptest.NewServer(router)

	b.ts = ts
	b.store = store
	return nil
}

func (b *bddContext) aVaultExistsWithItems(vaultID, section string, table *godog.Table) error {
	// Create vault
	body, _ := json.Marshal(map[string]string{
		"id":   vaultID,
		"name": vaultID,
	})
	resp, err := adminRequest("POST", b.ts.URL+"/v1/vaults", body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("create vault %s: got status %d", vaultID, resp.StatusCode)
	}

	// Store items
	fields := make(map[string]string)
	for _, row := range table.Rows[1:] { // skip header
		fields[row.Cells[0].Value] = row.Cells[1].Value
	}
	if err := b.store.SetItemFields(vaultID, section, fields); err != nil {
		return fmt.Errorf("set item fields: %w", err)
	}
	return nil
}

func (b *bddContext) aTEEInstanceIsRegisteredWithDstackAppID(dstackAppID string) error {
	var priv [32]byte
	rand.Read(priv[:])
	pub, _ := curve25519.X25519(priv[:], curve25519.Basepoint)
	pubHex := hex.EncodeToString(pub)

	h := sha1.Sum(pub)
	fid := hex.EncodeToString(h[:])

	body, _ := json.Marshal(map[string]string{
		"public_key":    pubHex,
		"dstack_app_id": dstackAppID,
		"label":         "bdd-tee",
	})
	resp, err := adminRequest("POST", b.ts.URL+"/v1/instances", body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("register instance: got status %d", resp.StatusCode)
	}

	b.teePriv = priv
	b.fid = fid
	return nil
}

func (b *bddContext) theInstanceHasAccessToVault(vaultID string) error {
	resp, err := adminRequest("POST", b.ts.URL+"/v1/vaults/"+vaultID+"/instances/"+b.fid, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("grant vault access: got status %d", resp.StatusCode)
	}
	return nil
}

// ── When steps ──────────────────────────────────────────────────────

func (b *bddContext) iCreateAVault(vaultID, name string) error {
	body, _ := json.Marshal(map[string]string{
		"id":   vaultID,
		"name": name,
	})
	resp, err := adminRequest("POST", b.ts.URL+"/v1/vaults", body)
	if err != nil {
		return err
	}
	b.lastBody, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	b.lastStatus = resp.StatusCode
	return nil
}

func (b *bddContext) iPUTItems(vaultID, section string, table *godog.Table) error {
	fields := make(map[string]string)
	for _, row := range table.Rows[1:] {
		fields[row.Cells[0].Value] = row.Cells[1].Value
	}
	body, _ := json.Marshal(map[string]interface{}{
		"fields": fields,
	})
	resp, err := adminRequest("PUT", b.ts.URL+"/v1/vaults/"+vaultID+"/items/"+section, body)
	if err != nil {
		return err
	}
	b.lastBody, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	b.lastStatus = resp.StatusCode
	return nil
}

func (b *bddContext) iGETItemsForVault(vaultID string) error {
	resp, err := adminRequest("GET", b.ts.URL+"/v1/vaults/"+vaultID+"/items", nil)
	if err != nil {
		return err
	}
	b.lastBody, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	b.lastStatus = resp.StatusCode
	return nil
}

func (b *bddContext) iPOSTTo(path string) error {
	url := b.ts.URL + path
	resp, err := adminRequest("POST", url, []byte("{}"))
	if err != nil {
		return err
	}
	b.lastBody, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	b.lastStatus = resp.StatusCode
	return nil
}

func (b *bddContext) iPOSTToWithJSON(path string, jsonDoc *godog.DocString) error {
	// Replace {{fid}} template
	content := strings.ReplaceAll(jsonDoc.Content, "{{fid}}", b.fid)

	url := b.ts.URL + path
	resp, err := http.Post(url, "application/json", bytes.NewReader([]byte(content)))
	if err != nil {
		return err
	}
	b.lastBody, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	b.lastStatus = resp.StatusCode
	return nil
}

func (b *bddContext) iRequestAChallenge() error {
	body, _ := json.Marshal(map[string]string{"fid": b.fid})
	resp, err := http.Post(b.ts.URL+"/v1/secrets/challenge", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("challenge request: status %d, body: %s", resp.StatusCode, respBody)
	}

	var chResp struct {
		ChallengeID string `json:"challenge_id"`
		Challenge   string `json:"challenge"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&chResp); err != nil {
		return err
	}

	blob, err := base64.StdEncoding.DecodeString(chResp.Challenge)
	if err != nil {
		return err
	}
	plain, err := crypto.Decrypt(b.teePriv, blob)
	if err != nil {
		return err
	}

	b.challengeID = chResp.ChallengeID
	b.challengeResponse = base64.StdEncoding.EncodeToString(plain)
	return nil
}

func (b *bddContext) iSolveAndFetchSecrets(table *godog.Table) error {
	var refs []string
	for _, row := range table.Rows[1:] {
		refs = append(refs, row.Cells[0].Value)
	}

	body, _ := json.Marshal(map[string]interface{}{
		"fid":                b.fid,
		"secret_references":  refs,
		"challenge_id":       b.challengeID,
		"challenge_response": b.challengeResponse,
	})
	resp, err := http.Post(b.ts.URL+"/v1/secrets/fetch", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	b.lastBody, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	b.lastStatus = resp.StatusCode

	if resp.StatusCode == http.StatusOK {
		var fetchResp struct {
			Secrets map[string]string `json:"secrets"`
		}
		if err := json.Unmarshal(b.lastBody, &fetchResp); err != nil {
			return err
		}
		b.fetchedSecrets = fetchResp.Secrets
	}
	return nil
}

// ── Then steps ──────────────────────────────────────────────────────

func (b *bddContext) theResponseStatusShouldBe(expected int) error {
	if b.lastStatus != expected {
		return fmt.Errorf("expected status %d, got %d (body: %s)", expected, b.lastStatus, b.lastBody)
	}
	return nil
}

func (b *bddContext) theResponseJSONShouldBe(key, expected string) error {
	var m map[string]interface{}
	if err := json.Unmarshal(b.lastBody, &m); err != nil {
		return fmt.Errorf("parse response JSON: %w", err)
	}
	val, ok := m[key]
	if !ok {
		return fmt.Errorf("key %q not found in response", key)
	}
	if fmt.Sprint(val) != expected {
		return fmt.Errorf("expected %q = %q, got %q", key, expected, val)
	}
	return nil
}

func (b *bddContext) theDecryptedSecretShouldBe(ref, expected string) error {
	b64, ok := b.fetchedSecrets[ref]
	if !ok {
		return fmt.Errorf("secret %q not in response", ref)
	}
	blob, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return fmt.Errorf("base64 decode: %w", err)
	}
	plain, err := crypto.Decrypt(b.teePriv, blob)
	if err != nil {
		return fmt.Errorf("decrypt: %w", err)
	}
	if string(plain) != expected {
		return fmt.Errorf("expected %q, got %q", expected, string(plain))
	}
	return nil
}

// ── Suite runner ────────────────────────────────────────────────────

func TestBDD(t *testing.T) {
	b := &bddContext{}

	suite := godog.TestSuite{
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
				b.reset()
				return ctx, nil
			})

			// Given
			sc.Step(`^the server is running$`, b.theServerIsRunning)
			sc.Step(`^a vault "([^"]*)" exists with items for "([^"]*)":$`, b.aVaultExistsWithItems)
			sc.Step(`^a TEE instance is registered with dstack app id "([^"]*)"$`, b.aTEEInstanceIsRegisteredWithDstackAppID)
			sc.Step(`^the instance has access to vault "([^"]*)"$`, b.theInstanceHasAccessToVault)

			// When
			sc.Step(`^I create a vault "([^"]*)" with name "([^"]*)"$`, b.iCreateAVault)
			sc.Step(`^I PUT items for vault "([^"]*)" section "([^"]*)" with fields:$`, b.iPUTItems)
			sc.Step(`^I GET items for vault "([^"]*)"$`, b.iGETItemsForVault)
			sc.Step(`^I POST to "([^"]*)"$`, b.iPOSTTo)
			sc.Step(`^I POST to "([^"]*)" with JSON:$`, b.iPOSTToWithJSON)
			sc.Step(`^I request a challenge for the TEE instance$`, b.iRequestAChallenge)
			sc.Step(`^I solve the challenge and fetch secrets:$`, b.iSolveAndFetchSecrets)

			// Then
			sc.Step(`^the response status should be (\d+)$`, b.theResponseStatusShouldBe)
			sc.Step(`^the response JSON "([^"]*)" should be "([^"]*)"$`, b.theResponseJSONShouldBe)
			sc.Step(`^the decrypted secret "([^"]*)" should be "([^"]*)"$`, b.theDecryptedSecretShouldBe)
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"../features"},
			TestingT: t,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("BDD tests failed")
	}

	// Final cleanup
	b.reset()
}

func init() {
	// Suppress Gin debug output during BDD tests
	os.Setenv("GIN_MODE", "release")
}
