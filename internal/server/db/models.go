package db

import "time"

// Vault represents a secret vault (replaces the old App concept).
type Vault struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// VaultItem represents a single field stored in a vault.
type VaultItem struct {
	ID        int64     `json:"id"`
	VaultID   string    `json:"vault_id"`
	ItemName  string    `json:"item_name"`
	Section   string    `json:"section"`
	Value     string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TEEInstance represents a registered TEE instance with its public key.
type TEEInstance struct {
	FID         string     `json:"fid"`
	Label       string     `json:"label"`
	PublicKey   []byte     `json:"public_key"`
	DstackAppID string     `json:"dstack_app_id"`
	CreatedAt   time.Time  `json:"created_at"`
	LastUsedAt  *time.Time `json:"last_used_at"`
}

// DebugPolicy controls whether debug read is allowed for a vault+instance pair.
type DebugPolicy struct {
	VaultID   string    `json:"vault_id"`
	FID       string    `json:"fid"`
	AllowRead bool      `json:"allow_read"`
	UpdatedAt time.Time `json:"updated_at"`
}
