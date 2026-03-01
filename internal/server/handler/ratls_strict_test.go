package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aspect-build/jingui/internal/attestation"
	"github.com/aspect-build/jingui/internal/server/db"
	"github.com/gin-gonic/gin"
)

func newStrictChallengeRouter(t *testing.T) *gin.Engine {
	store, err := db.NewStore(":memory:")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.CreateVault(&db.Vault{ID: "a1", Name: "app"}); err != nil {
		t.Fatalf("create vault: %v", err)
	}
	if err := store.RegisterInstance(&db.TEEInstance{FID: "f1", PublicKey: bytes.Repeat([]byte{2}, 32), DstackAppID: "a1"}); err != nil {
		t.Fatalf("register instance: %v", err)
	}
	if err := store.GrantVaultAccess("a1", "f1"); err != nil {
		t.Fatalf("grant vault access: %v", err)
	}

	r := gin.New()
	r.POST("/v1/secrets/challenge", HandleIssueChallenge(store, true, attestation.NewRATLSVerifier(), attestation.NewDstackInfoCollector("")))
	return r
}

func TestIssueChallenge_StrictRequiresClientAttestation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := newStrictChallengeRouter(t)

	body, _ := json.Marshal(map[string]any{"fid": "f1"})
	req := httptest.NewRequest(http.MethodPost, "/v1/secrets/challenge", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestIssueChallenge_StrictRequiresClientAttestationAppID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := newStrictChallengeRouter(t)

	body, _ := json.Marshal(map[string]any{
		"fid": "f1",
		"client_attestation": map[string]any{
			"app_cert": "dummy",
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/secrets/challenge", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestIssueChallenge_StrictRejectsMismatchedClientAttestationAppID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := newStrictChallengeRouter(t)

	body, _ := json.Marshal(map[string]any{
		"fid": "f1",
		"client_attestation": map[string]any{
			"app_id":   "wrong",
			"app_cert": "dummy",
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/secrets/challenge", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestIssueChallenge_StrictRejectsEmptyVerifiedAppID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store, err := db.NewStore(":memory:")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.CreateVault(&db.Vault{ID: "a1", Name: "app"}); err != nil {
		t.Fatalf("create vault: %v", err)
	}
	if err := store.RegisterInstance(&db.TEEInstance{FID: "f1", PublicKey: bytes.Repeat([]byte{2}, 32), DstackAppID: "a1"}); err != nil {
		t.Fatalf("register instance: %v", err)
	}
	if err := store.GrantVaultAccess("a1", "f1"); err != nil {
		t.Fatalf("grant vault access: %v", err)
	}

	// Verifier returns empty AppID — simulates cert without app_id OID extension.
	verifier := testVerifier{identity: attestation.VerifiedIdentity{AppID: ""}}
	collector := fakeCollector{bundle: attestation.Bundle{AppID: "server-app", AppCert: "pem"}}
	r := gin.New()
	r.POST("/v1/secrets/challenge", HandleIssueChallenge(store, true, verifier, collector))

	body, _ := json.Marshal(map[string]any{
		"fid": "f1",
		"client_attestation": map[string]any{
			"app_id":   "a1",
			"app_cert": "dummy-cert",
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/secrets/challenge", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for empty verified app_id, got %d body=%s", w.Code, w.Body.String())
	}
}
