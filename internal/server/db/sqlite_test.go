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

func TestVaultCRUD(t *testing.T) {
	s := newTestStore(t)

	v := &Vault{ID: "test-vault", Name: "Test Vault"}
	if err := s.CreateVault(v); err != nil {
		t.Fatalf("CreateVault: %v", err)
	}

	got, err := s.GetVault("test-vault")
	if err != nil {
		t.Fatalf("GetVault: %v", err)
	}
	if got == nil {
		t.Fatal("GetVault returned nil")
	}
	if got.ID != "test-vault" || got.Name != "Test Vault" {
		t.Errorf("got vault %+v", got)
	}

	vaults, err := s.ListVaults()
	if err != nil {
		t.Fatalf("ListVaults: %v", err)
	}
	if len(vaults) != 1 {
		t.Fatalf("ListVaults: got %d vaults", len(vaults))
	}

	// Not found
	got, err = s.GetVault("nonexistent")
	if err != nil {
		t.Fatalf("GetVault nonexistent: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for nonexistent vault")
	}
}

func TestVaultDuplicate(t *testing.T) {
	s := newTestStore(t)

	s.CreateVault(&Vault{ID: "v1", Name: "V1"})
	err := s.CreateVault(&Vault{ID: "v1", Name: "V1 dup"})
	if err != ErrVaultDuplicate {
		t.Fatalf("expected ErrVaultDuplicate, got: %v", err)
	}
}

func TestUpdateVault(t *testing.T) {
	s := newTestStore(t)

	s.CreateVault(&Vault{ID: "v1", Name: "V1"})
	ok, err := s.UpdateVault(&Vault{ID: "v1", Name: "Updated"})
	if err != nil {
		t.Fatalf("UpdateVault: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	got, _ := s.GetVault("v1")
	if got.Name != "Updated" {
		t.Errorf("expected name=Updated, got %q", got.Name)
	}

	// Update nonexistent
	ok, err = s.UpdateVault(&Vault{ID: "nope", Name: "x"})
	if err != nil {
		t.Fatalf("UpdateVault nonexistent: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false for nonexistent")
	}
}

func TestDeleteVault(t *testing.T) {
	s := newTestStore(t)

	s.CreateVault(&Vault{ID: "v1", Name: "V1"})
	deleted, err := s.DeleteVault("v1")
	if err != nil {
		t.Fatalf("DeleteVault: %v", err)
	}
	if !deleted {
		t.Fatal("expected deleted=true")
	}

	got, _ := s.GetVault("v1")
	if got != nil {
		t.Fatal("vault should be deleted")
	}

	// Delete nonexistent
	deleted, err = s.DeleteVault("nonexistent")
	if err != nil {
		t.Fatalf("DeleteVault nonexistent: %v", err)
	}
	if deleted {
		t.Fatal("expected deleted=false")
	}
}

func TestDeleteVault_ForeignKeyBlocks(t *testing.T) {
	s := newTestStore(t)

	s.CreateVault(&Vault{ID: "v1", Name: "V1"})
	s.SetItemFields("v1", "item1", map[string]string{"key": "value"})

	_, err := s.DeleteVault("v1")
	if err == nil {
		t.Fatal("expected foreign key error")
	}
	if err != ErrVaultHasDependents {
		t.Fatalf("expected ErrVaultHasDependents, got: %v", err)
	}
}

func TestDeleteVaultCascade(t *testing.T) {
	s := newTestStore(t)

	s.CreateVault(&Vault{ID: "v1", Name: "V1"})
	s.SetItemFields("v1", "item1", map[string]string{"key": "value"})
	s.RegisterInstance(&TEEInstance{
		FID: "fid1", PublicKey: []byte("pubkey-32-bytes-placeholder-here"),
		DstackAppID: "app1",
	})
	s.GrantVaultAccess("v1", "fid1")
	s.UpsertDebugPolicy("v1", "fid1", true)

	deleted, err := s.DeleteVaultCascade("v1")
	if err != nil {
		t.Fatalf("DeleteVaultCascade: %v", err)
	}
	if !deleted {
		t.Fatal("expected deleted=true")
	}

	// Verify vault, items, and access gone
	v, _ := s.GetVault("v1")
	if v != nil {
		t.Fatal("vault should be deleted")
	}
	sections, _ := s.ListSections("v1")
	if len(sections) != 0 {
		t.Fatal("vault items should be deleted")
	}
	has, _ := s.HasVaultAccess("v1", "fid1")
	if has {
		t.Fatal("vault access should be revoked")
	}

	// Instance itself should still exist
	inst, _ := s.GetInstance("fid1")
	if inst == nil {
		t.Fatal("instance should still exist after vault cascade")
	}

	// Nonexistent
	deleted, err = s.DeleteVaultCascade("nonexistent")
	if err != nil {
		t.Fatalf("DeleteVaultCascade nonexistent: %v", err)
	}
	if deleted {
		t.Fatal("expected deleted=false")
	}
}

func TestVaultItemFields(t *testing.T) {
	s := newTestStore(t)
	s.CreateVault(&Vault{ID: "v1", Name: "V1"})

	// Set fields for a section
	err := s.SetItemFields("v1", "alice@gmail.com", map[string]string{
		"password":  "secret123",
		"api_key":   "key-abc",
	})
	if err != nil {
		t.Fatalf("SetItemFields: %v", err)
	}

	// Get fields
	items, err := s.GetItemFields("v1", "alice@gmail.com")
	if err != nil {
		t.Fatalf("GetItemFields: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(items))
	}

	// Get single field value
	val, err := s.GetFieldValue("v1", "alice@gmail.com", "password")
	if err != nil {
		t.Fatalf("GetFieldValue: %v", err)
	}
	if val != "secret123" {
		t.Errorf("expected secret123, got %q", val)
	}

	// List sections
	sections, err := s.ListSections("v1")
	if err != nil {
		t.Fatalf("ListSections: %v", err)
	}
	if len(sections) != 1 || sections[0] != "alice@gmail.com" {
		t.Errorf("unexpected sections: %v", sections)
	}

	// SetItemFields replaces existing
	err = s.SetItemFields("v1", "alice@gmail.com", map[string]string{
		"password": "updated",
	})
	if err != nil {
		t.Fatalf("SetItemFields update: %v", err)
	}
	items, _ = s.GetItemFields("v1", "alice@gmail.com")
	if len(items) != 1 {
		t.Fatalf("expected 1 field after replace, got %d", len(items))
	}
	if items[0].Value != "updated" {
		t.Errorf("expected updated, got %q", items[0].Value)
	}

	// GetFieldValue for missing field
	_, err = s.GetFieldValue("v1", "alice@gmail.com", "api_key")
	if err == nil {
		t.Fatal("expected error for deleted field")
	}
}

func TestUpsertField(t *testing.T) {
	s := newTestStore(t)
	s.CreateVault(&Vault{ID: "v1", Name: "V1"})

	err := s.UpsertField("v1", "item1", "key1", "val1")
	if err != nil {
		t.Fatalf("UpsertField: %v", err)
	}

	val, _ := s.GetFieldValue("v1", "item1", "key1")
	if val != "val1" {
		t.Errorf("expected val1, got %q", val)
	}

	// Upsert same key with new value
	err = s.UpsertField("v1", "item1", "key1", "val2")
	if err != nil {
		t.Fatalf("UpsertField update: %v", err)
	}
	val, _ = s.GetFieldValue("v1", "item1", "key1")
	if val != "val2" {
		t.Errorf("expected val2, got %q", val)
	}
}

func TestDeleteSection(t *testing.T) {
	s := newTestStore(t)
	s.CreateVault(&Vault{ID: "v1", Name: "V1"})
	s.SetItemFields("v1", "item1", map[string]string{"k": "v"})

	deleted, err := s.DeleteSection("v1", "item1")
	if err != nil {
		t.Fatalf("DeleteSection: %v", err)
	}
	if !deleted {
		t.Fatal("expected deleted=true")
	}

	sections, _ := s.ListSections("v1")
	if len(sections) != 0 {
		t.Fatal("section should be deleted")
	}

	// Nonexistent
	deleted, _ = s.DeleteSection("v1", "nonexistent")
	if deleted {
		t.Fatal("expected deleted=false")
	}
}

func TestDeleteField(t *testing.T) {
	s := newTestStore(t)
	s.CreateVault(&Vault{ID: "v1", Name: "V1"})
	s.SetItemFields("v1", "item1", map[string]string{"k1": "v1", "k2": "v2"})

	deleted, err := s.DeleteField("v1", "item1", "k1")
	if err != nil {
		t.Fatalf("DeleteField: %v", err)
	}
	if !deleted {
		t.Fatal("expected deleted=true")
	}

	items, _ := s.GetItemFields("v1", "item1")
	if len(items) != 1 {
		t.Fatalf("expected 1 field remaining, got %d", len(items))
	}
}

func TestMergeItemFields(t *testing.T) {
	s := newTestStore(t)
	s.CreateVault(&Vault{ID: "v1", Name: "V1"})

	// Seed two fields
	s.SetItemFields("v1", "sec1", map[string]string{"a": "1", "b": "2", "c": "3"})

	// Merge: upsert a (update) + d (insert), delete c
	err := s.MergeItemFields("v1", "sec1", map[string]string{"a": "10", "d": "4"}, []string{"c"})
	if err != nil {
		t.Fatalf("MergeItemFields: %v", err)
	}

	items, _ := s.GetItemFields("v1", "sec1")
	got := make(map[string]string, len(items))
	for _, it := range items {
		got[it.ItemName] = it.Value
	}

	// a updated, b untouched, c deleted, d added
	if got["a"] != "10" {
		t.Errorf("expected a=10, got %q", got["a"])
	}
	if got["b"] != "2" {
		t.Errorf("expected b=2, got %q", got["b"])
	}
	if _, ok := got["c"]; ok {
		t.Error("expected c to be deleted")
	}
	if got["d"] != "4" {
		t.Errorf("expected d=4, got %q", got["d"])
	}
	if len(got) != 3 {
		t.Errorf("expected 3 fields, got %d: %v", len(got), got)
	}

	// Merge with only deletes
	err = s.MergeItemFields("v1", "sec1", nil, []string{"b"})
	if err != nil {
		t.Fatalf("MergeItemFields delete-only: %v", err)
	}
	items, _ = s.GetItemFields("v1", "sec1")
	if len(items) != 2 {
		t.Errorf("expected 2 fields after delete, got %d", len(items))
	}

	// Merge with only upserts
	err = s.MergeItemFields("v1", "sec1", map[string]string{"e": "5"}, nil)
	if err != nil {
		t.Fatalf("MergeItemFields upsert-only: %v", err)
	}
	items, _ = s.GetItemFields("v1", "sec1")
	if len(items) != 3 {
		t.Errorf("expected 3 fields after upsert, got %d", len(items))
	}
}

func TestTEEInstanceCRUD(t *testing.T) {
	s := newTestStore(t)

	inst := &TEEInstance{
		FID:         "abc123",
		PublicKey:   []byte("pubkey-32-bytes-placeholder-here"),
		DstackAppID: "dstack-app-1",
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
	if got.DstackAppID != "dstack-app-1" || got.Label != "test-instance" {
		t.Errorf("got instance %+v", got)
	}

	// Update last used
	if err := s.UpdateLastUsed("abc123"); err != nil {
		t.Fatalf("UpdateLastUsed: %v", err)
	}
	got, _ = s.GetInstance("abc123")
	if got.LastUsedAt == nil {
		t.Error("LastUsedAt should not be nil after UpdateLastUsed")
	}
}

func TestTEEInstanceRegister_PublicKeyUnique(t *testing.T) {
	s := newTestStore(t)

	pub := []byte("shared-32-byte-public-key-value!")
	s.RegisterInstance(&TEEInstance{
		FID: "fid-1", PublicKey: pub, DstackAppID: "app1",
	})

	err := s.RegisterInstance(&TEEInstance{
		FID: "fid-2", PublicKey: pub, DstackAppID: "app1",
	})
	if err != ErrInstanceDuplicateKey {
		t.Fatalf("expected ErrInstanceDuplicateKey, got: %v", err)
	}
}

func TestTEEInstanceRegister_FIDUnique(t *testing.T) {
	s := newTestStore(t)

	s.RegisterInstance(&TEEInstance{
		FID: "fid-1", PublicKey: []byte("pubkey-32-bytes-placeholder-here"), DstackAppID: "app1",
	})

	err := s.RegisterInstance(&TEEInstance{
		FID: "fid-1", PublicKey: []byte("another-32-byte-public-key-here!"), DstackAppID: "app2",
	})
	if err != ErrInstanceDuplicateFID {
		t.Fatalf("expected ErrInstanceDuplicateFID, got: %v", err)
	}
}

func TestListInstances(t *testing.T) {
	s := newTestStore(t)

	instances, err := s.ListInstances()
	if err != nil {
		t.Fatalf("ListInstances: %v", err)
	}
	if len(instances) != 0 {
		t.Fatalf("expected 0, got %d", len(instances))
	}

	s.RegisterInstance(&TEEInstance{
		FID: "fid1", PublicKey: []byte("pubkey-32-bytes-placeholder-here"), DstackAppID: "app1", Label: "inst1",
	})
	s.RegisterInstance(&TEEInstance{
		FID: "fid2", PublicKey: []byte("pubkey-32-bytes-placeholder-two!"), DstackAppID: "app1", Label: "inst2",
	})

	instances, _ = s.ListInstances()
	if len(instances) != 2 {
		t.Fatalf("expected 2, got %d", len(instances))
	}
}

func TestDeleteInstance(t *testing.T) {
	s := newTestStore(t)
	s.CreateVault(&Vault{ID: "v1", Name: "V1"})
	s.RegisterInstance(&TEEInstance{
		FID: "fid1", PublicKey: []byte("pubkey-32-bytes-placeholder-here"), DstackAppID: "app1",
	})
	s.GrantVaultAccess("v1", "fid1")
	s.UpsertDebugPolicy("v1", "fid1", true)

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

	// Junction and debug policy should be cleaned up
	has, _ := s.HasVaultAccess("v1", "fid1")
	if has {
		t.Fatal("vault access should be revoked")
	}

	// Nonexistent
	deleted, _ = s.DeleteInstance("nonexistent")
	if deleted {
		t.Fatal("expected deleted=false")
	}
}

func TestUpdateInstance(t *testing.T) {
	s := newTestStore(t)
	s.RegisterInstance(&TEEInstance{
		FID: "fid1", PublicKey: []byte("pubkey-32-bytes-placeholder-here"), DstackAppID: "app1", Label: "old",
	})

	ok, err := s.UpdateInstance("fid1", "app2", "new-label")
	if err != nil {
		t.Fatalf("UpdateInstance: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	got, _ := s.GetInstance("fid1")
	if got.DstackAppID != "app2" || got.Label != "new-label" {
		t.Errorf("got %+v", got)
	}

	// Nonexistent
	ok, _ = s.UpdateInstance("nope", "x", "x")
	if ok {
		t.Fatal("expected ok=false")
	}
}

func TestVaultInstanceAccess(t *testing.T) {
	s := newTestStore(t)
	s.CreateVault(&Vault{ID: "v1", Name: "V1"})
	s.CreateVault(&Vault{ID: "v2", Name: "V2"})
	s.RegisterInstance(&TEEInstance{
		FID: "fid1", PublicKey: []byte("pubkey-32-bytes-placeholder-here"), DstackAppID: "app1",
	})

	// Grant access
	if err := s.GrantVaultAccess("v1", "fid1"); err != nil {
		t.Fatalf("GrantVaultAccess: %v", err)
	}
	if err := s.GrantVaultAccess("v2", "fid1"); err != nil {
		t.Fatalf("GrantVaultAccess v2: %v", err)
	}

	// Check access
	has, _ := s.HasVaultAccess("v1", "fid1")
	if !has {
		t.Fatal("expected access to v1")
	}
	has, _ = s.HasVaultAccess("v2", "fid1")
	if !has {
		t.Fatal("expected access to v2")
	}
	has, _ = s.HasVaultAccess("v1", "fid-other")
	if has {
		t.Fatal("expected no access for unknown fid")
	}

	// List vaults for instance
	vaults, err := s.ListInstanceVaults("fid1")
	if err != nil {
		t.Fatalf("ListInstanceVaults: %v", err)
	}
	if len(vaults) != 2 {
		t.Fatalf("expected 2 vaults, got %d", len(vaults))
	}

	// List instances for vault
	instances, err := s.ListVaultInstances("v1")
	if err != nil {
		t.Fatalf("ListVaultInstances: %v", err)
	}
	if len(instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(instances))
	}

	// Revoke access
	revoked, err := s.RevokeVaultAccess("v1", "fid1")
	if err != nil {
		t.Fatalf("RevokeVaultAccess: %v", err)
	}
	if !revoked {
		t.Fatal("expected revoked=true")
	}
	has, _ = s.HasVaultAccess("v1", "fid1")
	if has {
		t.Fatal("access to v1 should be revoked")
	}

	// Grant is idempotent (INSERT OR IGNORE)
	if err := s.GrantVaultAccess("v2", "fid1"); err != nil {
		t.Fatalf("GrantVaultAccess idempotent: %v", err)
	}
}

func TestDebugPolicy(t *testing.T) {
	s := newTestStore(t)
	s.CreateVault(&Vault{ID: "v1", Name: "V1"})
	s.RegisterInstance(&TEEInstance{
		FID: "fid1", PublicKey: []byte("pubkey-32-bytes-placeholder-here"), DstackAppID: "app1",
	})

	// No policy → nil
	p, err := s.GetDebugPolicy("v1", "fid1")
	if err != nil {
		t.Fatalf("GetDebugPolicy: %v", err)
	}
	if p != nil {
		t.Fatal("expected nil for missing policy")
	}

	// Upsert allow=false
	if err := s.UpsertDebugPolicy("v1", "fid1", false); err != nil {
		t.Fatalf("UpsertDebugPolicy: %v", err)
	}
	p, _ = s.GetDebugPolicy("v1", "fid1")
	if p == nil {
		t.Fatal("expected non-nil policy")
	}
	if p.AllowRead {
		t.Error("expected allow_read=false")
	}

	// Upsert allow=true
	if err := s.UpsertDebugPolicy("v1", "fid1", true); err != nil {
		t.Fatalf("UpsertDebugPolicy update: %v", err)
	}
	p, _ = s.GetDebugPolicy("v1", "fid1")
	if !p.AllowRead {
		t.Error("expected allow_read=true")
	}
}
