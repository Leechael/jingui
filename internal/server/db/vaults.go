package db

import (
	"database/sql"
	"errors"
	"fmt"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

var (
	ErrVaultDuplicate     = errors.New("vault already exists")
	ErrVaultHasDependents = errors.New("vault has dependent records; delete them first or use ?cascade=true")
)

// CreateVault inserts a new vault.
func (s *Store) CreateVault(v *Vault) error {
	_, err := s.db.Exec(
		`INSERT INTO vaults (id, name) VALUES (?, ?)`,
		v.ID, v.Name,
	)
	if err != nil {
		var sqliteErr *sqlite.Error
		if errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY {
			return ErrVaultDuplicate
		}
		return fmt.Errorf("insert vault: %w", err)
	}
	return nil
}

// GetVault retrieves a vault by ID.
func (s *Store) GetVault(id string) (*Vault, error) {
	v := &Vault{}
	err := s.db.QueryRow(
		`SELECT id, name, created_at FROM vaults WHERE id = ?`, id,
	).Scan(&v.ID, &v.Name, &v.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get vault: %w", err)
	}
	return v, nil
}

// UpdateVault updates the name of a vault. Returns true if a row was updated.
func (s *Store) UpdateVault(v *Vault) (bool, error) {
	res, err := s.db.Exec(
		`UPDATE vaults SET name = ? WHERE id = ?`,
		v.Name, v.ID,
	)
	if err != nil {
		return false, fmt.Errorf("update vault: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ListVaults returns all vaults ordered by creation time.
func (s *Store) ListVaults() ([]Vault, error) {
	rows, err := s.db.Query(
		`SELECT id, name, created_at FROM vaults ORDER BY created_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("list vaults: %w", err)
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

// DeleteVault deletes a vault by ID. Returns ErrVaultHasDependents if FK constraints prevent deletion.
func (s *Store) DeleteVault(id string) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM vaults WHERE id = ?`, id)
	if err != nil {
		var sqliteErr *sqlite.Error
		if errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY {
			return false, ErrVaultHasDependents
		}
		return false, fmt.Errorf("delete vault: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// DeleteVaultCascade deletes a vault and all dependent records in a transaction.
func (s *Store) DeleteVaultCascade(id string) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Delete debug_policies referencing this vault
	if _, err := tx.Exec(`DELETE FROM debug_policies WHERE vault_id = ?`, id); err != nil {
		return false, fmt.Errorf("delete debug_policies for vault: %w", err)
	}

	// Delete vault_instance_access entries
	if _, err := tx.Exec(`DELETE FROM vault_instance_access WHERE vault_id = ?`, id); err != nil {
		return false, fmt.Errorf("delete vault_instance_access for vault: %w", err)
	}

	// Delete vault_items
	if _, err := tx.Exec(`DELETE FROM vault_items WHERE vault_id = ?`, id); err != nil {
		return false, fmt.Errorf("delete vault_items for vault: %w", err)
	}

	// Delete the vault itself
	res, err := tx.Exec(`DELETE FROM vaults WHERE id = ?`, id)
	if err != nil {
		return false, fmt.Errorf("delete vault: %w", err)
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
