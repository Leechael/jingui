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
		Vault:                "test-app",
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
	if got.Vault != "test-app" || got.Name != "Test App" {
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

func TestVaultItemCRUD(t *testing.T) {
	s := newTestStore(t)

	// Need app first (foreign key)
	app := &App{
		Vault: "app1", Name: "App 1", ServiceType: "gmail",
		CredentialsEncrypted: []byte("creds"),
	}
	s.CreateApp(app)

	item := &VaultItem{
		Vault:           "app1",
		Item:            "user@example.com",
		SecretEncrypted: []byte("encrypted-token"),
	}

	if err := s.UpsertVaultItem(item); err != nil {
		t.Fatalf("UpsertVaultItem: %v", err)
	}

	got, err := s.GetVaultItem("app1", "user@example.com")
	if err != nil {
		t.Fatalf("GetVaultItem: %v", err)
	}
	if got == nil {
		t.Fatal("GetVaultItem returned nil")
	}
	if string(got.SecretEncrypted) != "encrypted-token" {
		t.Errorf("SecretEncrypted = %q", got.SecretEncrypted)
	}

	// Upsert (update)
	item.SecretEncrypted = []byte("updated-token")
	if err := s.UpsertVaultItem(item); err != nil {
		t.Fatalf("UpsertVaultItem update: %v", err)
	}

	got, err = s.GetVaultItem("app1", "user@example.com")
	if err != nil {
		t.Fatalf("GetVaultItem after update: %v", err)
	}
	if string(got.SecretEncrypted) != "updated-token" {
		t.Errorf("SecretEncrypted after update = %q", got.SecretEncrypted)
	}
}

func TestTEEInstanceCRUD(t *testing.T) {
	s := newTestStore(t)

	app := &App{
		Vault: "app1", Name: "App 1", ServiceType: "gmail",
		CredentialsEncrypted: []byte("creds"),
	}
	s.CreateApp(app)

	// tee_instances now references vault_items(app_id, item)
	if err := s.UpsertVaultItem(&VaultItem{
		Vault:           "app1",
		Item:            "user@example.com",
		SecretEncrypted: []byte("token"),
	}); err != nil {
		t.Fatalf("UpsertVaultItem: %v", err)
	}

	inst := &TEEInstance{
		FID:                   "abc123",
		PublicKey:             []byte("pubkey-32-bytes-placeholder-here"),
		BoundVault:            "app1",
		BoundAttestationAppID: "app1",
		BoundItem:             "user@example.com",
		Label:                 "test-instance",
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
	if got.BoundVault != "app1" || got.BoundItem != "user@example.com" {
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

	// app exists, but no matching vault_item (app_id, item)
	if err := s.CreateApp(&App{
		Vault:                "app1",
		Name:                 "App 1",
		ServiceType:          "gmail",
		CredentialsEncrypted: []byte("creds"),
	}); err != nil {
		t.Fatalf("CreateApp: %v", err)
	}

	err := s.RegisterInstance(&TEEInstance{
		FID:                   "no-secret",
		PublicKey:             []byte("another-32-bytes-public-key-value"),
		BoundVault:            "app1",
		BoundAttestationAppID: "app1",
		BoundItem:             "missing@example.com",
	})
	if err == nil {
		t.Fatal("expected foreign-key error when no matching vault_item exists")
	}
	if err != ErrInstanceAppUserNotFound {
		t.Fatalf("expected ErrInstanceAppUserNotFound, got: %v", err)
	}
}

func TestDeleteApp(t *testing.T) {
	s := newTestStore(t)

	app := &App{
		Vault: "app1", Name: "App 1", ServiceType: "gmail",
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
		Vault: "app1", Name: "App 1", ServiceType: "gmail",
		CredentialsEncrypted: []byte("creds"),
	})
	s.UpsertVaultItem(&VaultItem{
		Vault: "app1", Item: "user@example.com",
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
		Vault: "app1", Name: "App 1", ServiceType: "gmail",
		CredentialsEncrypted: []byte("creds"),
	})
	s.UpsertVaultItem(&VaultItem{
		Vault: "app1", Item: "user@example.com",
		SecretEncrypted: []byte("token"),
	})
	s.RegisterInstance(&TEEInstance{
		FID: "fid1", PublicKey: []byte("pubkey-32-bytes-placeholder-here"),
		BoundVault: "app1", BoundAttestationAppID: "app1", BoundItem: "user@example.com",
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
	item, _ := s.GetVaultItem("app1", "user@example.com")
	if item != nil {
		t.Fatal("item should be deleted")
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
		Vault: "app1", Name: "App 1", ServiceType: "gmail",
		CredentialsEncrypted: []byte("creds"),
	})
	s.UpsertVaultItem(&VaultItem{
		Vault: "app1", Item: "user@example.com",
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
		BoundVault: "app1", BoundAttestationAppID: "app1", BoundItem: "user@example.com", Label: "inst1",
	})
	s.RegisterInstance(&TEEInstance{
		FID: "fid2", PublicKey: []byte("pubkey-32-bytes-placeholder-two!"),
		BoundVault: "app1", BoundAttestationAppID: "app1", BoundItem: "user@example.com", Label: "inst2",
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
		Vault: "app1", Name: "App 1", ServiceType: "gmail",
		CredentialsEncrypted: []byte("creds"),
	})
	s.UpsertVaultItem(&VaultItem{
		Vault: "app1", Item: "user@example.com",
		SecretEncrypted: []byte("token"),
	})
	s.RegisterInstance(&TEEInstance{
		FID: "fid1", PublicKey: []byte("pubkey-32-bytes-placeholder-here"),
		BoundVault: "app1", BoundAttestationAppID: "app1", BoundItem: "user@example.com",
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

func TestListVaultItems(t *testing.T) {
	s := newTestStore(t)

	s.CreateApp(&App{
		Vault: "app1", Name: "App 1", ServiceType: "gmail",
		CredentialsEncrypted: []byte("creds"),
	})
	s.CreateApp(&App{
		Vault: "app2", Name: "App 2", ServiceType: "drive",
		CredentialsEncrypted: []byte("creds"),
	})

	// Empty
	items, err := s.ListVaultItems()
	if err != nil {
		t.Fatalf("ListVaultItems: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}

	s.UpsertVaultItem(&VaultItem{Vault: "app1", Item: "user1@example.com", SecretEncrypted: []byte("t1")})
	s.UpsertVaultItem(&VaultItem{Vault: "app1", Item: "user2@example.com", SecretEncrypted: []byte("t2")})
	s.UpsertVaultItem(&VaultItem{Vault: "app2", Item: "user1@example.com", SecretEncrypted: []byte("t3")})

	items, err = s.ListVaultItems()
	if err != nil {
		t.Fatalf("ListVaultItems: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	// Encrypted blob should not be populated
	for _, vi := range items {
		if len(vi.SecretEncrypted) != 0 {
			t.Fatal("SecretEncrypted should not be populated in list")
		}
	}
}

func TestListVaultItemsByVault(t *testing.T) {
	s := newTestStore(t)

	s.CreateApp(&App{Vault: "app1", Name: "App 1", ServiceType: "gmail", CredentialsEncrypted: []byte("creds")})
	s.CreateApp(&App{Vault: "app2", Name: "App 2", ServiceType: "drive", CredentialsEncrypted: []byte("creds")})

	s.UpsertVaultItem(&VaultItem{Vault: "app1", Item: "user1@example.com", SecretEncrypted: []byte("t1")})
	s.UpsertVaultItem(&VaultItem{Vault: "app1", Item: "user2@example.com", SecretEncrypted: []byte("t2")})
	s.UpsertVaultItem(&VaultItem{Vault: "app2", Item: "user1@example.com", SecretEncrypted: []byte("t3")})

	items, err := s.ListVaultItemsByVault("app1")
	if err != nil {
		t.Fatalf("ListVaultItemsByVault: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items for app1, got %d", len(items))
	}

	items, err = s.ListVaultItemsByVault("app2")
	if err != nil {
		t.Fatalf("ListVaultItemsByVault: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item for app2, got %d", len(items))
	}

	items, err = s.ListVaultItemsByVault("nonexistent")
	if err != nil {
		t.Fatalf("ListVaultItemsByVault nonexistent: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestDeleteVaultItem(t *testing.T) {
	s := newTestStore(t)

	s.CreateApp(&App{Vault: "app1", Name: "App 1", ServiceType: "gmail", CredentialsEncrypted: []byte("creds")})
	s.UpsertVaultItem(&VaultItem{Vault: "app1", Item: "user@example.com", SecretEncrypted: []byte("token")})

	deleted, err := s.DeleteVaultItem("app1", "user@example.com")
	if err != nil {
		t.Fatalf("DeleteVaultItem: %v", err)
	}
	if !deleted {
		t.Fatal("expected deleted=true")
	}

	got, _ := s.GetVaultItem("app1", "user@example.com")
	if got != nil {
		t.Fatal("item should be deleted")
	}

	// Nonexistent
	deleted, err = s.DeleteVaultItem("app1", "nonexistent")
	if err != nil {
		t.Fatalf("DeleteVaultItem nonexistent: %v", err)
	}
	if deleted {
		t.Fatal("expected deleted=false")
	}
}

func TestDeleteVaultItem_ForeignKeyBlocks(t *testing.T) {
	s := newTestStore(t)

	s.CreateApp(&App{Vault: "app1", Name: "App 1", ServiceType: "gmail", CredentialsEncrypted: []byte("creds")})
	s.UpsertVaultItem(&VaultItem{Vault: "app1", Item: "user@example.com", SecretEncrypted: []byte("token")})
	s.RegisterInstance(&TEEInstance{
		FID: "fid1", PublicKey: []byte("pubkey-32-bytes-placeholder-here"),
		BoundVault: "app1", BoundAttestationAppID: "app1", BoundItem: "user@example.com",
	})

	_, err := s.DeleteVaultItem("app1", "user@example.com")
	if err == nil {
		t.Fatal("expected foreign key error")
	}
	if err != ErrItemHasDependents {
		t.Fatalf("expected ErrItemHasDependents, got: %v", err)
	}
}

func TestDeleteVaultItemCascade(t *testing.T) {
	s := newTestStore(t)

	s.CreateApp(&App{Vault: "app1", Name: "App 1", ServiceType: "gmail", CredentialsEncrypted: []byte("creds")})
	s.UpsertVaultItem(&VaultItem{Vault: "app1", Item: "user@example.com", SecretEncrypted: []byte("token")})
	s.RegisterInstance(&TEEInstance{
		FID: "fid1", PublicKey: []byte("pubkey-32-bytes-placeholder-here"),
		BoundVault: "app1", BoundAttestationAppID: "app1", BoundItem: "user@example.com",
	})

	deleted, err := s.DeleteVaultItemCascade("app1", "user@example.com")
	if err != nil {
		t.Fatalf("DeleteVaultItemCascade: %v", err)
	}
	if !deleted {
		t.Fatal("expected deleted=true")
	}

	item, _ := s.GetVaultItem("app1", "user@example.com")
	if item != nil {
		t.Fatal("item should be deleted")
	}
	inst, _ := s.GetInstance("fid1")
	if inst != nil {
		t.Fatal("instance should be deleted")
	}

	// Nonexistent
	deleted, err = s.DeleteVaultItemCascade("app1", "nonexistent")
	if err != nil {
		t.Fatalf("DeleteVaultItemCascade nonexistent: %v", err)
	}
	if deleted {
		t.Fatal("expected deleted=false")
	}
}

func TestTEEInstanceRegister_PublicKeyUnique(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateApp(&App{
		Vault:                "app1",
		Name:                 "App 1",
		ServiceType:          "gmail",
		CredentialsEncrypted: []byte("creds"),
	}); err != nil {
		t.Fatalf("CreateApp: %v", err)
	}
	if err := s.UpsertVaultItem(&VaultItem{
		Vault:           "app1",
		Item:            "user@example.com",
		SecretEncrypted: []byte("token"),
	}); err != nil {
		t.Fatalf("UpsertVaultItem: %v", err)
	}

	pub := []byte("shared-32-byte-public-key-value!!")

	if err := s.RegisterInstance(&TEEInstance{
		FID:                   "fid-1",
		PublicKey:             pub,
		BoundVault:            "app1",
		BoundAttestationAppID: "app1",
		BoundItem:             "user@example.com",
	}); err != nil {
		t.Fatalf("RegisterInstance first: %v", err)
	}

	err := s.RegisterInstance(&TEEInstance{
		FID:                   "fid-2",
		PublicKey:             pub,
		BoundVault:            "app1",
		BoundAttestationAppID: "app1",
		BoundItem:             "user@example.com",
	})
	if err == nil {
		t.Fatal("expected unique constraint error for duplicate public_key")
	}
	if err != ErrInstanceDuplicateKey {
		t.Fatalf("expected ErrInstanceDuplicateKey, got: %v", err)
	}
}
