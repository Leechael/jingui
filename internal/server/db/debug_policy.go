package db

import (
	"database/sql"
	"fmt"
)

// UpsertDebugPolicy inserts or updates a debug policy for a vault+instance pair.
func (s *Store) UpsertDebugPolicy(vaultID, fid string, allow bool) error {
	allowInt := 0
	if allow {
		allowInt = 1
	}
	_, err := s.db.Exec(
		`INSERT INTO debug_policies (vault_id, fid, allow_read)
		 VALUES (?, ?, ?)
		 ON CONFLICT(vault_id, fid) DO UPDATE SET
			allow_read = excluded.allow_read,
			updated_at = CURRENT_TIMESTAMP`,
		vaultID, fid, allowInt,
	)
	if err != nil {
		return fmt.Errorf("upsert debug policy: %w", err)
	}
	return nil
}

// GetDebugPolicy retrieves a debug policy. Returns nil if no policy exists.
func (s *Store) GetDebugPolicy(vaultID, fid string) (*DebugPolicy, error) {
	p := &DebugPolicy{}
	var allowInt int
	err := s.db.QueryRow(
		`SELECT vault_id, fid, allow_read, updated_at
		 FROM debug_policies WHERE vault_id = ? AND fid = ?`, vaultID, fid,
	).Scan(&p.VaultID, &p.FID, &allowInt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get debug policy: %w", err)
	}
	p.AllowRead = allowInt != 0
	return p, nil
}
