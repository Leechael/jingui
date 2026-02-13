package db

import (
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestAppCRUD(t *testing.T) {
	s := newTestStore(t)

	app := &App{
		AppID:                "test-app",
		Name:                 "Test App",
		ServiceType:          "gmail",
		RequiredScopes:       "https://mail.google.com/",
		CredentialsEncrypted: []byte("encrypted-creds"),
	}

	if err := s.CreateApp(app); err != nil {
		t.Fatalf("CreateApp: %v", err)
	}

	got, err := s.GetApp("test-app")
	if err != nil {
		t.Fatalf("GetApp: %v", err)
	}
	if got == nil {
		t.Fatal("GetApp returned nil")
	}
	if got.AppID != "test-app" || got.Name != "Test App" {
		t.Errorf("got app %+v", got)
	}

	apps, err := s.ListApps()
	if err != nil {
		t.Fatalf("ListApps: %v", err)
	}
	if len(apps) != 1 {
		t.Fatalf("ListApps: got %d apps", len(apps))
	}

	// Not found
	got, err = s.GetApp("nonexistent")
	if err != nil {
		t.Fatalf("GetApp: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for nonexistent app")
	}
}

func TestUserSecretCRUD(t *testing.T) {
	s := newTestStore(t)

	// Need app first (foreign key)
	app := &App{
		AppID: "app1", Name: "App 1", ServiceType: "gmail",
		CredentialsEncrypted: []byte("creds"),
	}
	s.CreateApp(app)

	secret := &UserSecret{
		AppID:           "app1",
		UserID:          "user@example.com",
		SecretEncrypted: []byte("encrypted-token"),
	}

	if err := s.UpsertUserSecret(secret); err != nil {
		t.Fatalf("UpsertUserSecret: %v", err)
	}

	got, err := s.GetUserSecret("app1", "user@example.com")
	if err != nil {
		t.Fatalf("GetUserSecret: %v", err)
	}
	if got == nil {
		t.Fatal("GetUserSecret returned nil")
	}
	if string(got.SecretEncrypted) != "encrypted-token" {
		t.Errorf("SecretEncrypted = %q", got.SecretEncrypted)
	}

	// Upsert (update)
	secret.SecretEncrypted = []byte("updated-token")
	if err := s.UpsertUserSecret(secret); err != nil {
		t.Fatalf("UpsertUserSecret update: %v", err)
	}

	got, err = s.GetUserSecret("app1", "user@example.com")
	if err != nil {
		t.Fatalf("GetUserSecret after update: %v", err)
	}
	if string(got.SecretEncrypted) != "updated-token" {
		t.Errorf("SecretEncrypted after update = %q", got.SecretEncrypted)
	}
}

func TestTEEInstanceCRUD(t *testing.T) {
	s := newTestStore(t)

	app := &App{
		AppID: "app1", Name: "App 1", ServiceType: "gmail",
		CredentialsEncrypted: []byte("creds"),
	}
	s.CreateApp(app)

	// tee_instances now references user_secrets(app_id, user_id)
	if err := s.UpsertUserSecret(&UserSecret{
		AppID:           "app1",
		UserID:          "user@example.com",
		SecretEncrypted: []byte("token"),
	}); err != nil {
		t.Fatalf("UpsertUserSecret: %v", err)
	}

	inst := &TEEInstance{
		FID:         "abc123",
		PublicKey:   []byte("pubkey-32-bytes-placeholder-here"),
		BoundAppID:  "app1",
		BoundUserID: "user@example.com",
		Label:       "test-instance",
	}

	if err := s.RegisterInstance(inst); err != nil {
		t.Fatalf("RegisterInstance: %v", err)
	}

	got, err := s.GetInstance("abc123")
	if err != nil {
		t.Fatalf("GetInstance: %v", err)
	}
	if got == nil {
		t.Fatal("GetInstance returned nil")
	}
	if got.BoundAppID != "app1" || got.BoundUserID != "user@example.com" {
		t.Errorf("got instance %+v", got)
	}

	// Update last used
	if err := s.UpdateLastUsed("abc123"); err != nil {
		t.Fatalf("UpdateLastUsed: %v", err)
	}

	got, err = s.GetInstance("abc123")
	if err != nil {
		t.Fatalf("GetInstance after update: %v", err)
	}
	if got.LastUsedAt == nil {
		t.Error("LastUsedAt should not be nil after UpdateLastUsed")
	}
}

func TestTEEInstanceRegister_ForeignKeyEnforced(t *testing.T) {
	s := newTestStore(t)

	// app exists, but no matching user_secret (app_id,user_id)
	if err := s.CreateApp(&App{
		AppID:                "app1",
		Name:                 "App 1",
		ServiceType:          "gmail",
		CredentialsEncrypted: []byte("creds"),
	}); err != nil {
		t.Fatalf("CreateApp: %v", err)
	}

	err := s.RegisterInstance(&TEEInstance{
		FID:         "no-secret",
		PublicKey:   []byte("another-32-bytes-public-key-value"),
		BoundAppID:  "app1",
		BoundUserID: "missing@example.com",
	})
	if err == nil {
		t.Fatal("expected foreign-key error when no matching user_secret exists")
	}
	if err != ErrInstanceAppUserNotFound {
		t.Fatalf("expected ErrInstanceAppUserNotFound, got: %v", err)
	}
}

func TestTEEInstanceRegister_PublicKeyUnique(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateApp(&App{
		AppID:                "app1",
		Name:                 "App 1",
		ServiceType:          "gmail",
		CredentialsEncrypted: []byte("creds"),
	}); err != nil {
		t.Fatalf("CreateApp: %v", err)
	}
	if err := s.UpsertUserSecret(&UserSecret{
		AppID:           "app1",
		UserID:          "user@example.com",
		SecretEncrypted: []byte("token"),
	}); err != nil {
		t.Fatalf("UpsertUserSecret: %v", err)
	}

	pub := []byte("shared-32-byte-public-key-value!!")

	if err := s.RegisterInstance(&TEEInstance{
		FID:         "fid-1",
		PublicKey:   pub,
		BoundAppID:  "app1",
		BoundUserID: "user@example.com",
	}); err != nil {
		t.Fatalf("RegisterInstance first: %v", err)
	}

	err := s.RegisterInstance(&TEEInstance{
		FID:         "fid-2",
		PublicKey:   pub,
		BoundAppID:  "app1",
		BoundUserID: "user@example.com",
	})
	if err == nil {
		t.Fatal("expected unique constraint error for duplicate public_key")
	}
	if err != ErrInstanceDuplicateKey {
		t.Fatalf("expected ErrInstanceDuplicateKey, got: %v", err)
	}
}
