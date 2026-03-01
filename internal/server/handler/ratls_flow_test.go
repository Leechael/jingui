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

	// Create vault
	if err := store.CreateVault(&db.Vault{ID: "a1", Name: "app"}); err != nil {
		t.Fatalf("create vault: %v", err)
	}

	// Store a field
	if err := store.SetItemFields("a1", "u1", map[string]string{"client_id": "test-cid"}); err != nil {
		t.Fatalf("set item fields: %v", err)
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
	if err := store.RegisterInstance(&db.TEEInstance{FID: fid, PublicKey: pub, DstackAppID: "a1"}); err != nil {
		t.Fatalf("register instance: %v", err)
	}

	// Grant vault access
	if err := store.GrantVaultAccess("a1", fid); err != nil {
		t.Fatalf("grant vault access: %v", err)
	}

	r := gin.New()
	verifier := testVerifier{identity: attestation.VerifiedIdentity{AppID: "a1"}}
	collector := fakeCollector{bundle: attestation.Bundle{AppID: "server-app", AppCert: "pem"}}
	r.POST("/v1/secrets/challenge", HandleIssueChallenge(store, true, verifier, collector))
	r.POST("/v1/secrets/fetch", HandleFetchSecrets(store, true))

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
	// With real data now, fetch should succeed
	if w2.Code == http.StatusUnauthorized {
		t.Fatalf("expected non-401 after RA-verified challenge, got body=%s", w2.Body.String())
	}
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w2.Code, w2.Body.String())
	}
}
