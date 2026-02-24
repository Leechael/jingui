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

func TestDeleteApp(t *testing.T) {
	s := newTestStore(t)

	app := &App{
		AppID: "app1", Name: "App 1", ServiceType: "gmail",
		CredentialsEncrypted: []byte("creds"),
	}
	s.CreateApp(app)

	// Delete existing app
	deleted, err := s.DeleteApp("app1")
	if err != nil {
		t.Fatalf("DeleteApp: %v", err)
	}
	if !deleted {
		t.Fatal("expected deleted=true")
	}

	// Verify gone
	got, _ := s.GetApp("app1")
	if got != nil {
		t.Fatal("app should be deleted")
	}

	// Delete nonexistent
	deleted, err = s.DeleteApp("nonexistent")
	if err != nil {
		t.Fatalf("DeleteApp nonexistent: %v", err)
	}
	if deleted {
		t.Fatal("expected deleted=false for nonexistent app")
	}
}

func TestDeleteApp_ForeignKeyBlocks(t *testing.T) {
	s := newTestStore(t)

	s.CreateApp(&App{
		AppID: "app1", Name: "App 1", ServiceType: "gmail",
		CredentialsEncrypted: []byte("creds"),
	})
	s.UpsertUserSecret(&UserSecret{
		AppID: "app1", UserID: "user@example.com",
		SecretEncrypted: []byte("token"),
	})

	_, err := s.DeleteApp("app1")
	if err == nil {
		t.Fatal("expected foreign key error")
	}
	if err != ErrAppHasDependents {
		t.Fatalf("expected ErrAppHasDependents, got: %v", err)
	}
}

func TestDeleteAppCascade(t *testing.T) {
	s := newTestStore(t)

	s.CreateApp(&App{
		AppID: "app1", Name: "App 1", ServiceType: "gmail",
		CredentialsEncrypted: []byte("creds"),
	})
	s.UpsertUserSecret(&UserSecret{
		AppID: "app1", UserID: "user@example.com",
		SecretEncrypted: []byte("token"),
	})
	s.RegisterInstance(&TEEInstance{
		FID: "fid1", PublicKey: []byte("pubkey-32-bytes-placeholder-here"),
		BoundAppID: "app1", BoundUserID: "user@example.com",
	})

	deleted, err := s.DeleteAppCascade("app1")
	if err != nil {
		t.Fatalf("DeleteAppCascade: %v", err)
	}
	if !deleted {
		t.Fatal("expected deleted=true")
	}

	// Verify all gone
	app, _ := s.GetApp("app1")
	if app != nil {
		t.Fatal("app should be deleted")
	}
	secret, _ := s.GetUserSecret("app1", "user@example.com")
	if secret != nil {
		t.Fatal("secret should be deleted")
	}
	inst, _ := s.GetInstance("fid1")
	if inst != nil {
		t.Fatal("instance should be deleted")
	}

	// Cascade nonexistent
	deleted, err = s.DeleteAppCascade("nonexistent")
	if err != nil {
		t.Fatalf("DeleteAppCascade nonexistent: %v", err)
	}
	if deleted {
		t.Fatal("expected deleted=false for nonexistent app")
	}
}

func TestListInstances(t *testing.T) {
	s := newTestStore(t)

	s.CreateApp(&App{
		AppID: "app1", Name: "App 1", ServiceType: "gmail",
		CredentialsEncrypted: []byte("creds"),
	})
	s.UpsertUserSecret(&UserSecret{
		AppID: "app1", UserID: "user@example.com",
		SecretEncrypted: []byte("token"),
	})

	// Empty list
	instances, err := s.ListInstances()
	if err != nil {
		t.Fatalf("ListInstances: %v", err)
	}
	if len(instances) != 0 {
		t.Fatalf("expected 0 instances, got %d", len(instances))
	}

	s.RegisterInstance(&TEEInstance{
		FID: "fid1", PublicKey: []byte("pubkey-32-bytes-placeholder-here"),
		BoundAppID: "app1", BoundUserID: "user@example.com", Label: "inst1",
	})
	s.RegisterInstance(&TEEInstance{
		FID: "fid2", PublicKey: []byte("pubkey-32-bytes-placeholder-two!"),
		BoundAppID: "app1", BoundUserID: "user@example.com", Label: "inst2",
	})

	instances, err = s.ListInstances()
	if err != nil {
		t.Fatalf("ListInstances: %v", err)
	}
	if len(instances) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(instances))
	}
}

func TestDeleteInstance(t *testing.T) {
	s := newTestStore(t)

	s.CreateApp(&App{
		AppID: "app1", Name: "App 1", ServiceType: "gmail",
		CredentialsEncrypted: []byte("creds"),
	})
	s.UpsertUserSecret(&UserSecret{
		AppID: "app1", UserID: "user@example.com",
		SecretEncrypted: []byte("token"),
	})
	s.RegisterInstance(&TEEInstance{
		FID: "fid1", PublicKey: []byte("pubkey-32-bytes-placeholder-here"),
		BoundAppID: "app1", BoundUserID: "user@example.com",
	})

	deleted, err := s.DeleteInstance("fid1")
	if err != nil {
		t.Fatalf("DeleteInstance: %v", err)
	}
	if !deleted {
		t.Fatal("expected deleted=true")
	}

	got, _ := s.GetInstance("fid1")
	if got != nil {
		t.Fatal("instance should be deleted")
	}

	// Nonexistent
	deleted, err = s.DeleteInstance("nonexistent")
	if err != nil {
		t.Fatalf("DeleteInstance nonexistent: %v", err)
	}
	if deleted {
		t.Fatal("expected deleted=false")
	}
}

func TestListUserSecrets(t *testing.T) {
	s := newTestStore(t)

	s.CreateApp(&App{
		AppID: "app1", Name: "App 1", ServiceType: "gmail",
		CredentialsEncrypted: []byte("creds"),
	})
	s.CreateApp(&App{
		AppID: "app2", Name: "App 2", ServiceType: "drive",
		CredentialsEncrypted: []byte("creds"),
	})

	// Empty
	secrets, err := s.ListUserSecrets()
	if err != nil {
		t.Fatalf("ListUserSecrets: %v", err)
	}
	if len(secrets) != 0 {
		t.Fatalf("expected 0 secrets, got %d", len(secrets))
	}

	s.UpsertUserSecret(&UserSecret{AppID: "app1", UserID: "user1@example.com", SecretEncrypted: []byte("t1")})
	s.UpsertUserSecret(&UserSecret{AppID: "app1", UserID: "user2@example.com", SecretEncrypted: []byte("t2")})
	s.UpsertUserSecret(&UserSecret{AppID: "app2", UserID: "user1@example.com", SecretEncrypted: []byte("t3")})

	secrets, err = s.ListUserSecrets()
	if err != nil {
		t.Fatalf("ListUserSecrets: %v", err)
	}
	if len(secrets) != 3 {
		t.Fatalf("expected 3 secrets, got %d", len(secrets))
	}
	// Encrypted blob should not be populated
	for _, s := range secrets {
		if len(s.SecretEncrypted) != 0 {
			t.Fatal("SecretEncrypted should not be populated in list")
		}
	}
}

func TestListUserSecretsByApp(t *testing.T) {
	s := newTestStore(t)

	s.CreateApp(&App{AppID: "app1", Name: "App 1", ServiceType: "gmail", CredentialsEncrypted: []byte("creds")})
	s.CreateApp(&App{AppID: "app2", Name: "App 2", ServiceType: "drive", CredentialsEncrypted: []byte("creds")})

	s.UpsertUserSecret(&UserSecret{AppID: "app1", UserID: "user1@example.com", SecretEncrypted: []byte("t1")})
	s.UpsertUserSecret(&UserSecret{AppID: "app1", UserID: "user2@example.com", SecretEncrypted: []byte("t2")})
	s.UpsertUserSecret(&UserSecret{AppID: "app2", UserID: "user1@example.com", SecretEncrypted: []byte("t3")})

	secrets, err := s.ListUserSecretsByApp("app1")
	if err != nil {
		t.Fatalf("ListUserSecretsByApp: %v", err)
	}
	if len(secrets) != 2 {
		t.Fatalf("expected 2 secrets for app1, got %d", len(secrets))
	}

	secrets, err = s.ListUserSecretsByApp("app2")
	if err != nil {
		t.Fatalf("ListUserSecretsByApp: %v", err)
	}
	if len(secrets) != 1 {
		t.Fatalf("expected 1 secret for app2, got %d", len(secrets))
	}

	secrets, err = s.ListUserSecretsByApp("nonexistent")
	if err != nil {
		t.Fatalf("ListUserSecretsByApp nonexistent: %v", err)
	}
	if len(secrets) != 0 {
		t.Fatalf("expected 0 secrets, got %d", len(secrets))
	}
}

func TestDeleteUserSecret(t *testing.T) {
	s := newTestStore(t)

	s.CreateApp(&App{AppID: "app1", Name: "App 1", ServiceType: "gmail", CredentialsEncrypted: []byte("creds")})
	s.UpsertUserSecret(&UserSecret{AppID: "app1", UserID: "user@example.com", SecretEncrypted: []byte("token")})

	deleted, err := s.DeleteUserSecret("app1", "user@example.com")
	if err != nil {
		t.Fatalf("DeleteUserSecret: %v", err)
	}
	if !deleted {
		t.Fatal("expected deleted=true")
	}

	got, _ := s.GetUserSecret("app1", "user@example.com")
	if got != nil {
		t.Fatal("secret should be deleted")
	}

	// Nonexistent
	deleted, err = s.DeleteUserSecret("app1", "nonexistent")
	if err != nil {
		t.Fatalf("DeleteUserSecret nonexistent: %v", err)
	}
	if deleted {
		t.Fatal("expected deleted=false")
	}
}

func TestDeleteUserSecret_ForeignKeyBlocks(t *testing.T) {
	s := newTestStore(t)

	s.CreateApp(&App{AppID: "app1", Name: "App 1", ServiceType: "gmail", CredentialsEncrypted: []byte("creds")})
	s.UpsertUserSecret(&UserSecret{AppID: "app1", UserID: "user@example.com", SecretEncrypted: []byte("token")})
	s.RegisterInstance(&TEEInstance{
		FID: "fid1", PublicKey: []byte("pubkey-32-bytes-placeholder-here"),
		BoundAppID: "app1", BoundUserID: "user@example.com",
	})

	_, err := s.DeleteUserSecret("app1", "user@example.com")
	if err == nil {
		t.Fatal("expected foreign key error")
	}
	if err != ErrSecretHasDependents {
		t.Fatalf("expected ErrSecretHasDependents, got: %v", err)
	}
}

func TestDeleteUserSecretCascade(t *testing.T) {
	s := newTestStore(t)

	s.CreateApp(&App{AppID: "app1", Name: "App 1", ServiceType: "gmail", CredentialsEncrypted: []byte("creds")})
	s.UpsertUserSecret(&UserSecret{AppID: "app1", UserID: "user@example.com", SecretEncrypted: []byte("token")})
	s.RegisterInstance(&TEEInstance{
		FID: "fid1", PublicKey: []byte("pubkey-32-bytes-placeholder-here"),
		BoundAppID: "app1", BoundUserID: "user@example.com",
	})

	deleted, err := s.DeleteUserSecretCascade("app1", "user@example.com")
	if err != nil {
		t.Fatalf("DeleteUserSecretCascade: %v", err)
	}
	if !deleted {
		t.Fatal("expected deleted=true")
	}

	secret, _ := s.GetUserSecret("app1", "user@example.com")
	if secret != nil {
		t.Fatal("secret should be deleted")
	}
	inst, _ := s.GetInstance("fid1")
	if inst != nil {
		t.Fatal("instance should be deleted")
	}

	// Nonexistent
	deleted, err = s.DeleteUserSecretCascade("app1", "nonexistent")
	if err != nil {
		t.Fatalf("DeleteUserSecretCascade nonexistent: %v", err)
	}
	if deleted {
		t.Fatal("expected deleted=false")
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
