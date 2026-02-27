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
	ErrInstanceDuplicateFID    = errors.New("instance with this FID already exists")
	ErrInstanceDuplicateKey    = errors.New("instance with this public key already exists")
	ErrInstanceAppUserNotFound = errors.New("bound vault/item not found: the vault must be registered and the item must have completed OAuth authorization before registering an instance")
)

// RegisterInstance inserts a new TEE instance.
func (s *Store) RegisterInstance(inst *TEEInstance) error {
	_, err := s.db.Exec(
		`INSERT INTO tee_instances (fid, public_key, bound_app_id, bound_attestation_app_id, bound_item, label)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		inst.FID, inst.PublicKey, inst.BoundVault, inst.BoundAttestationAppID, inst.BoundItem, inst.Label,
	)
	if err != nil {
		var sqliteErr *sqlite.Error
		if errors.As(err, &sqliteErr) {
			switch sqliteErr.Code() {
			case sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY:
				return ErrInstanceAppUserNotFound
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
		`SELECT fid, public_key, bound_app_id, bound_attestation_app_id, bound_item, label, created_at, last_used_at
		 FROM tee_instances WHERE fid = ?`, fid,
	).Scan(&inst.FID, &inst.PublicKey, &inst.BoundVault, &inst.BoundAttestationAppID, &inst.BoundItem, &inst.Label, &inst.CreatedAt, &inst.LastUsedAt)
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
		`SELECT fid, public_key, bound_app_id, bound_attestation_app_id, bound_item, label, created_at, last_used_at
		 FROM tee_instances ORDER BY created_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("list instances: %w", err)
	}
	defer rows.Close()

	var instances []TEEInstance
	for rows.Next() {
		var inst TEEInstance
		if err := rows.Scan(&inst.FID, &inst.PublicKey, &inst.BoundVault, &inst.BoundAttestationAppID, &inst.BoundItem, &inst.Label, &inst.CreatedAt, &inst.LastUsedAt); err != nil {
			return nil, fmt.Errorf("scan instance: %w", err)
		}
		instances = append(instances, inst)
	}
	return instances, rows.Err()
}

// DeleteInstance deletes a TEE instance by FID. Returns true if a row was deleted.
func (s *Store) DeleteInstance(fid string) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM tee_instances WHERE fid = ?`, fid)
	if err != nil {
		return false, fmt.Errorf("delete instance: %w", err)
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
