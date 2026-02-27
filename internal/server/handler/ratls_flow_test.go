package handler

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aspect-build/jingui/internal/attestation"
	jcrypto "github.com/aspect-build/jingui/internal/crypto"
	"github.com/aspect-build/jingui/internal/server/db"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/curve25519"
)

type testVerifier struct {
	identity attestation.VerifiedIdentity
	err      error
}

func (t testVerifier) Verify(_ context.Context, _ attestation.Bundle) (attestation.VerifiedIdentity, error) {
	return t.identity, t.err
}

type fakeCollector struct{ bundle attestation.Bundle }

func (f fakeCollector) Collect(_ context.Context) (attestation.Bundle, error) { return f.bundle, nil }

func setupStrictFlow(t *testing.T) (*gin.Engine, [32]byte, string) {
	t.Helper()
	store, err := db.NewStore(":memory:")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	masterKey := [32]byte{}
	for i := range masterKey {
		masterKey[i] = byte(i + 1)
	}

	app := &db.App{Vault: "a1", Name: "app", ServiceType: "gmail", CredentialsEncrypted: []byte("x")}
	if err := store.CreateApp(app); err != nil {
		t.Fatalf("create app: %v", err)
	}
	if err := store.UpsertVaultItem(&db.VaultItem{Vault: "a1", Item: "u1", SecretEncrypted: []byte("x")}); err != nil {
		t.Fatalf("upsert user secret: %v", err)
	}

	var priv [32]byte
	for i := range priv {
		priv[i] = byte(i + 2)
	}
	pub, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		t.Fatalf("pub derive: %v", err)
	}
	h := sha1.Sum(pub)
	fid := hex.EncodeToString(h[:])
	if err := store.RegisterInstance(&db.TEEInstance{FID: fid, PublicKey: pub, BoundVault: "a1", BoundAttestationAppID: "a1", BoundItem: "u1"}); err != nil {
		t.Fatalf("register instance: %v", err)
	}

	r := gin.New()
	verifier := testVerifier{identity: attestation.VerifiedIdentity{AppID: "a1"}}
	collector := fakeCollector{bundle: attestation.Bundle{AppID: "server-app", AppCert: "pem"}}
	r.POST("/v1/secrets/challenge", HandleIssueChallenge(store, true, verifier, collector))
	r.POST("/v1/secrets/fetch", HandleFetchSecrets(store, masterKey, true))

	return r, priv, fid
}

func TestStrictFlow_ChallengeThenFetchState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r, priv, fid := setupStrictFlow(t)

	chReq, _ := json.Marshal(map[string]any{
		"fid": fid,
		"client_attestation": map[string]any{
			"app_id":   "a1",
			"app_cert": "dummy-cert",
		},
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/secrets/challenge", bytes.NewReader(chReq))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("challenge status=%d body=%s", w.Code, w.Body.String())
	}

	var chResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &chResp); err != nil {
		t.Fatalf("decode challenge resp: %v", err)
	}
	challengeID := chResp["challenge_id"].(string)
	challengeB64 := chResp["challenge"].(string)
	blob, _ := base64.StdEncoding.DecodeString(challengeB64)
	plain, err := jcrypto.Decrypt(priv, blob)
	if err != nil {
		t.Fatalf("decrypt challenge: %v", err)
	}

	fetchReq, _ := json.Marshal(map[string]any{
		"fid":                fid,
		"secret_references":  []string{"jingui://a1/u1/client_id"},
		"challenge_id":       challengeID,
		"challenge_response": base64.StdEncoding.EncodeToString(plain),
	})
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/v1/secrets/fetch", bytes.NewReader(fetchReq))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)
	// secret resolution fails because fixtures are dummy, but auth/state gate should pass and reach resolver.
	if w2.Code == http.StatusUnauthorized {
		t.Fatalf("expected non-401 after RA-verified challenge, got body=%s", w2.Body.String())
	}
}
