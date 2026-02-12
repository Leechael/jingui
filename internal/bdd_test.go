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
	"strconv"
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
	ts        *httptest.Server
	store     *db.Store
	masterKey [32]byte

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
	var masterKey [32]byte
	if _, err := rand.Read(masterKey[:]); err != nil {
		return err
	}

	store, err := db.NewStore(":memory:")
	if err != nil {
		return fmt.Errorf("NewStore: %w", err)
	}

	cfg := &server.Config{
		MasterKey:  masterKey,
		AdminToken: testAdminToken,
	}

	router := server.NewRouter(store, cfg)
	ts := httptest.NewServer(router)
	cfg.BaseURL = ts.URL

	b.ts = ts
	b.store = store
	b.masterKey = masterKey
	return nil
}

func (b *bddContext) anAppExistsWithCredentials(appID, serviceType string, credJSON *godog.DocString) error {
	body, _ := json.Marshal(map[string]interface{}{
		"app_id":           appID,
		"name":             appID,
		"service_type":     serviceType,
		"credentials_json": json.RawMessage(credJSON.Content),
	})
	resp, err := adminRequest("POST", b.ts.URL+"/v1/apps", body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("register app %s: got status %d", appID, resp.StatusCode)
	}
	return nil
}

func (b *bddContext) userHasSecretsForApp(userID, appID string, table *godog.Table) error {
	secrets := make(map[string]string)
	for _, row := range table.Rows[1:] { // skip header
		secrets[row.Cells[0].Value] = row.Cells[1].Value
	}
	secretJSON, _ := json.Marshal(secrets)
	encrypted, err := crypto.EncryptAtRest(b.masterKey, secretJSON)
	if err != nil {
		return err
	}
	return b.store.UpsertUserSecret(&db.UserSecret{
		AppID:           appID,
		UserID:          userID,
		SecretEncrypted: encrypted,
	})
}

func (b *bddContext) aTEEInstanceIsRegistered(appID, userID string) error {
	var priv [32]byte
	rand.Read(priv[:])
	pub, _ := curve25519.X25519(priv[:], curve25519.Basepoint)
	pubHex := hex.EncodeToString(pub)

	h := sha1.Sum(pub)
	fid := hex.EncodeToString(h[:])

	body, _ := json.Marshal(map[string]string{
		"public_key":    pubHex,
		"bound_app_id":  appID,
		"bound_user_id": userID,
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

// ── When steps ──────────────────────────────────────────────────────

func (b *bddContext) iRegisterAnApp(appID, serviceType string, credJSON *godog.DocString) error {
	body, _ := json.Marshal(map[string]interface{}{
		"app_id":           appID,
		"name":             appID,
		"service_type":     serviceType,
		"credentials_json": json.RawMessage(credJSON.Content),
	})
	resp, err := adminRequest("POST", b.ts.URL+"/v1/apps", body)
	if err != nil {
		return err
	}
	b.lastBody, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	b.lastStatus = resp.StatusCode
	return nil
}

func (b *bddContext) iPUTCredentials(appID, userID string, table *godog.Table) error {
	secrets := make(map[string]string)
	for _, row := range table.Rows[1:] {
		secrets[row.Cells[0].Value] = row.Cells[1].Value
	}
	body, _ := json.Marshal(map[string]interface{}{
		"user_id": userID,
		"secrets": secrets,
	})
	resp, err := adminRequest("PUT", b.ts.URL+"/v1/credentials/"+appID, body)
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

func (b *bddContext) theResponseStatusShouldBeOneOf(codes string) error {
	for _, part := range strings.Split(codes, ",") {
		code, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return fmt.Errorf("invalid status code %q: %w", part, err)
		}
		if b.lastStatus == code {
			return nil
		}
	}
	return fmt.Errorf("expected status one of [%s], got %d (body: %s)", codes, b.lastStatus, b.lastBody)
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
			sc.Step(`^an app "([^"]*)" of type "([^"]*)" exists with credentials:$`, b.anAppExistsWithCredentials)
			sc.Step(`^user "([^"]*)" has secrets for app "([^"]*)":$`, b.userHasSecretsForApp)
			sc.Step(`^a TEE instance is registered for app "([^"]*)" and user "([^"]*)"$`, b.aTEEInstanceIsRegistered)

			// When
			sc.Step(`^I register an app "([^"]*)" of type "([^"]*)" with credentials:$`, b.iRegisterAnApp)
			sc.Step(`^I PUT credentials for app "([^"]*)" with user "([^"]*)" and secrets:$`, b.iPUTCredentials)
			sc.Step(`^I POST to "([^"]*)"$`, b.iPOSTTo)
			sc.Step(`^I POST to "([^"]*)" with JSON:$`, b.iPOSTToWithJSON)
			sc.Step(`^I request a challenge for the TEE instance$`, b.iRequestAChallenge)
			sc.Step(`^I solve the challenge and fetch secrets:$`, b.iSolveAndFetchSecrets)

			// Then
			sc.Step(`^the response status should be (\d+)$`, b.theResponseStatusShouldBe)
			sc.Step(`^the response status should be one of (.+)$`, b.theResponseStatusShouldBeOneOf)
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
