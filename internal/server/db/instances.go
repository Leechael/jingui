package db

import (
	"database/sql"
	"errors"
	"fmt"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// Sentinel errors for RegisterInstance.
var (
	ErrInstanceDuplicateFID = errors.New("instance with this FID already exists")
	ErrInstanceDuplicateKey = errors.New("instance with this public key already exists")
)

// RegisterInstance inserts a new TEE instance.
func (s *Store) RegisterInstance(inst *TEEInstance) error {
	_, err := s.db.Exec(
		`INSERT INTO tee_instances (fid, label, public_key, dstack_app_id)
		 VALUES (?, ?, ?, ?)`,
		inst.FID, inst.Label, inst.PublicKey, inst.DstackAppID,
	)
	if err != nil {
		var sqliteErr *sqlite.Error
		if errors.As(err, &sqliteErr) {
			switch sqliteErr.Code() {
			case sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY:
				return ErrInstanceDuplicateFID
			case sqlite3.SQLITE_CONSTRAINT_UNIQUE:
				return ErrInstanceDuplicateKey
			}
		}
		return fmt.Errorf("register instance: %w", err)
	}
	return nil
}

// GetInstance retrieves a TEE instance by FID.
func (s *Store) GetInstance(fid string) (*TEEInstance, error) {
	inst := &TEEInstance{}
	err := s.db.QueryRow(
		`SELECT fid, label, public_key, dstack_app_id, created_at, last_used_at
		 FROM tee_instances WHERE fid = ?`, fid,
	).Scan(&inst.FID, &inst.Label, &inst.PublicKey, &inst.DstackAppID, &inst.CreatedAt, &inst.LastUsedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get instance: %w", err)
	}
	return inst, nil
}

// ListInstances returns all registered TEE instances.
func (s *Store) ListInstances() ([]TEEInstance, error) {
	rows, err := s.db.Query(
		`SELECT fid, label, public_key, dstack_app_id, created_at, last_used_at
		 FROM tee_instances ORDER BY created_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("list instances: %w", err)
	}
	defer rows.Close()

	var instances []TEEInstance
	for rows.Next() {
		var inst TEEInstance
		if err := rows.Scan(&inst.FID, &inst.Label, &inst.PublicKey, &inst.DstackAppID, &inst.CreatedAt, &inst.LastUsedAt); err != nil {
			return nil, fmt.Errorf("scan instance: %w", err)
		}
		instances = append(instances, inst)
	}
	return instances, rows.Err()
}

// DeleteInstance deletes a TEE instance and its junction/debug_policy entries.
func (s *Store) DeleteInstance(fid string) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Delete debug policies referencing this instance
	if _, err := tx.Exec(`DELETE FROM debug_policies WHERE fid = ?`, fid); err != nil {
		return false, fmt.Errorf("delete debug_policies for instance: %w", err)
	}

	// Delete junction entries
	if _, err := tx.Exec(`DELETE FROM vault_instance_access WHERE fid = ?`, fid); err != nil {
		return false, fmt.Errorf("delete vault_instance_access for instance: %w", err)
	}

	// Delete the instance itself
	res, err := tx.Exec(`DELETE FROM tee_instances WHERE fid = ?`, fid)
	if err != nil {
		return false, fmt.Errorf("delete instance: %w", err)
	}

	n, _ := res.RowsAffected()
	if n == 0 {
		return false, nil
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit tx: %w", err)
	}
	return true, nil
}

// UpdateInstance updates dstack_app_id and label for a TEE instance.
func (s *Store) UpdateInstance(fid, dstackAppID, label string) (bool, error) {
	res, err := s.db.Exec(
		`UPDATE tee_instances SET dstack_app_id = ?, label = ? WHERE fid = ?`,
		dstackAppID, label, fid,
	)
	if err != nil {
		return false, fmt.Errorf("update instance: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// UpdateLastUsed updates the last_used_at timestamp for a TEE instance.
func (s *Store) UpdateLastUsed(fid string) error {
	_, err := s.db.Exec(
		`UPDATE tee_instances SET last_used_at = CURRENT_TIMESTAMP WHERE fid = ?`, fid,
	)
	if err != nil {
		return fmt.Errorf("update last used: %w", err)
	}
	return nil
}

// GrantVaultAccess inserts a vault↔instance junction entry.
func (s *Store) GrantVaultAccess(vaultID, fid string) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO vault_instance_access (vault_id, fid) VALUES (?, ?)`,
		vaultID, fid,
	)
	if err != nil {
		return fmt.Errorf("grant vault access: %w", err)
	}
	return nil
}

// RevokeVaultAccess deletes a vault↔instance junction entry.
func (s *Store) RevokeVaultAccess(vaultID, fid string) (bool, error) {
	res, err := s.db.Exec(
		`DELETE FROM vault_instance_access WHERE vault_id = ? AND fid = ?`,
		vaultID, fid,
	)
	if err != nil {
		return false, fmt.Errorf("revoke vault access: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ListInstanceVaults returns vaults accessible by an instance.
func (s *Store) ListInstanceVaults(fid string) ([]Vault, error) {
	rows, err := s.db.Query(
		`SELECT v.id, v.name, v.created_at
		 FROM vaults v
		 INNER JOIN vault_instance_access a ON v.id = a.vault_id
		 WHERE a.fid = ?
		 ORDER BY v.created_at`, fid,
	)
	if err != nil {
		return nil, fmt.Errorf("list instance vaults: %w", err)
	}
	defer rows.Close()

	var vaults []Vault
	for rows.Next() {
		var v Vault
		if err := rows.Scan(&v.ID, &v.Name, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan vault: %w", err)
		}
		vaults = append(vaults, v)
	}
	return vaults, rows.Err()
}

// ListVaultInstances returns instances with access to a vault.
func (s *Store) ListVaultInstances(vaultID string) ([]TEEInstance, error) {
	rows, err := s.db.Query(
		`SELECT t.fid, t.label, t.public_key, t.dstack_app_id, t.created_at, t.last_used_at
		 FROM tee_instances t
		 INNER JOIN vault_instance_access a ON t.fid = a.fid
		 WHERE a.vault_id = ?
		 ORDER BY t.created_at`, vaultID,
	)
	if err != nil {
		return nil, fmt.Errorf("list vault instances: %w", err)
	}
	defer rows.Close()

	var instances []TEEInstance
	for rows.Next() {
		var inst TEEInstance
		if err := rows.Scan(&inst.FID, &inst.Label, &inst.PublicKey, &inst.DstackAppID, &inst.CreatedAt, &inst.LastUsedAt); err != nil {
			return nil, fmt.Errorf("scan instance: %w", err)
		}
		instances = append(instances, inst)
	}
	return instances, rows.Err()
}

// HasVaultAccess checks if an instance has access to a vault.
func (s *Store) HasVaultAccess(vaultID, fid string) (bool, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM vault_instance_access WHERE vault_id = ? AND fid = ?`,
		vaultID, fid,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check vault access: %w", err)
	}
	return count > 0, nil
}
