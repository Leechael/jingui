package db

import "time"

// App represents a registered OAuth application.
type App struct {
	Vault                string    `json:"vault"`
	Name                 string    `json:"name"`
	ServiceType          string    `json:"service_type"`
	RequiredScopes       string    `json:"required_scopes"`
	CredentialsEncrypted []byte    `json:"-"`
	CreatedAt            time.Time `json:"created_at"`
}

// VaultItem represents an OAuth token stored for an item+vault combination.
type VaultItem struct {
	Vault           string    `json:"vault"`
	Item            string    `json:"item"`
	SecretEncrypted []byte    `json:"-"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// TEEInstance represents a registered TEE instance with its public key.
type TEEInstance struct {
	FID                   string     `json:"fid"`
	PublicKey             []byte     `json:"public_key"`
	BoundVault            string     `json:"bound_vault"`
	BoundAttestationAppID string     `json:"bound_attestation_app_id"`
	BoundItem             string     `json:"bound_item"`
	Label                 string     `json:"label"`
	CreatedAt             time.Time  `json:"created_at"`
	LastUsedAt            *time.Time `json:"last_used_at"`
}

// DebugPolicy controls whether an item may run jingui read in runtime.
type DebugPolicy struct {
	Vault          string    `json:"vault"`
	Item           string    `json:"item"`
	AllowReadDebug bool      `json:"allow_read_debug"`
	UpdatedAt      time.Time `json:"updated_at"`
}
