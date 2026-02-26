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

	app := &db.App{
		Vault:                "a1",
		Name:                 "app",
		ServiceType:          "gmail",
		RequiredScopes:       "",
		CredentialsEncrypted: []byte{1},
	}
	if err := store.CreateApp(app); err != nil {
		t.Fatalf("create app: %v", err)
	}
	if err := store.UpsertUserSecret(&db.UserSecret{Vault: "a1", UserID: "u1", SecretEncrypted: []byte{1}}); err != nil {
		t.Fatalf("upsert user secret: %v", err)
	}
	if err := store.RegisterInstance(&db.TEEInstance{FID: "f1", PublicKey: bytes.Repeat([]byte{2}, 32), BoundVault: "a1", BoundAttestationAppID: "a1", BoundUserID: "u1"}); err != nil {
		t.Fatalf("register instance: %v", err)
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
