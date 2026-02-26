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

// UserSecret represents an OAuth token stored for a user+vault combination.
type UserSecret struct {
	Vault           string    `json:"vault"`
	UserID          string    `json:"user_id"`
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
	BoundUserID           string     `json:"bound_user_id"`
	Label                 string     `json:"label"`
	CreatedAt             time.Time  `json:"created_at"`
	LastUsedAt            *time.Time `json:"last_used_at"`
}

// DebugPolicy controls whether a user may run jingui read in runtime.
type DebugPolicy struct {
	Vault          string    `json:"vault"`
	UserID         string    `json:"user_id"`
	AllowReadDebug bool      `json:"allow_read_debug"`
	UpdatedAt      time.Time `json:"updated_at"`
}
