package db

import (
	"database/sql"
	"errors"
	"fmt"
)

// ErrFieldNotFound is returned by GetFieldValue when the requested field does
// not exist. Callers should use errors.Is to distinguish this from real DB
// errors.
var ErrFieldNotFound = errors.New("field not found")

// UpsertField inserts or updates a single field in a vault item.
func (s *Store) UpsertField(vaultID, section, itemName, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO vault_items (vault_id, section, item_name, value)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(vault_id, section, item_name) DO UPDATE SET
		   value = excluded.value,
		   updated_at = CURRENT_TIMESTAMP`,
		vaultID, section, itemName, value,
	)
	if err != nil {
		return fmt.Errorf("upsert field: %w", err)
	}
	return nil
}

// SetItemFields batch upserts all fields for a section, replacing existing fields.
// Fields not in the map are deleted.
func (s *Store) SetItemFields(vaultID, section string, fields map[string]string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Delete existing fields for this vault+section
	if _, err := tx.Exec(
		`DELETE FROM vault_items WHERE vault_id = ? AND section = ?`,
		vaultID, section,
	); err != nil {
		return fmt.Errorf("delete old fields: %w", err)
	}

	// Insert new fields
	for name, value := range fields {
		if _, err := tx.Exec(
			`INSERT INTO vault_items (vault_id, section, item_name, value)
			 VALUES (?, ?, ?, ?)`,
			vaultID, section, name, value,
		); err != nil {
			return fmt.Errorf("insert field %q: %w", name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// GetItemFields returns all fields for a vault+section.
func (s *Store) GetItemFields(vaultID, section string) ([]VaultItem, error) {
	rows, err := s.db.Query(
		`SELECT rowid, vault_id, item_name, section, value, created_at, updated_at
		 FROM vault_items WHERE vault_id = ? AND section = ? ORDER BY item_name`,
		vaultID, section,
	)
	if err != nil {
		return nil, fmt.Errorf("get item fields: %w", err)
	}
	defer rows.Close()

	var items []VaultItem
	for rows.Next() {
		var vi VaultItem
		if err := rows.Scan(&vi.ID, &vi.VaultID, &vi.ItemName, &vi.Section, &vi.Value, &vi.CreatedAt, &vi.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan vault item: %w", err)
		}
		items = append(items, vi)
	}
	return items, rows.Err()
}

// GetFieldValue returns the value of a single field.
func (s *Store) GetFieldValue(vaultID, section, itemName string) (string, error) {
	var value string
	err := s.db.QueryRow(
		`SELECT value FROM vault_items WHERE vault_id = ? AND section = ? AND item_name = ?`,
		vaultID, section, itemName,
	).Scan(&value)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("%w: %s/%s/%s", ErrFieldNotFound, vaultID, section, itemName)
	}
	if err != nil {
		return "", fmt.Errorf("get field value: %w", err)
	}
	return value, nil
}

// ListSections returns distinct sections for a vault.
func (s *Store) ListSections(vaultID string) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT DISTINCT section FROM vault_items WHERE vault_id = ? ORDER BY section`,
		vaultID,
	)
	if err != nil {
		return nil, fmt.Errorf("list sections: %w", err)
	}
	defer rows.Close()

	var sections []string
	for rows.Next() {
		var section string
		if err := rows.Scan(&section); err != nil {
			return nil, fmt.Errorf("scan section: %w", err)
		}
		sections = append(sections, section)
	}
	return sections, rows.Err()
}

// MergeItemFields upserts provided fields and deletes specified keys without
// touching other existing fields in the section.
func (s *Store) MergeItemFields(vaultID, section string, upsert map[string]string, deleteKeys []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	for _, key := range deleteKeys {
		if _, err := tx.Exec(
			`DELETE FROM vault_items WHERE vault_id = ? AND section = ? AND item_name = ?`,
			vaultID, section, key,
		); err != nil {
			return fmt.Errorf("delete key %q: %w", key, err)
		}
	}

	for name, value := range upsert {
		if _, err := tx.Exec(
			`INSERT INTO vault_items (vault_id, section, item_name, value)
			 VALUES (?, ?, ?, ?)
			 ON CONFLICT(vault_id, section, item_name) DO UPDATE SET
			   value = excluded.value,
			   updated_at = CURRENT_TIMESTAMP`,
			vaultID, section, name, value,
		); err != nil {
			return fmt.Errorf("upsert field %q: %w", name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// DeleteSection deletes all fields in a section. Returns true if any rows were deleted.
func (s *Store) DeleteSection(vaultID, section string) (bool, error) {
	res, err := s.db.Exec(
		`DELETE FROM vault_items WHERE vault_id = ? AND section = ?`,
		vaultID, section,
	)
	if err != nil {
		return false, fmt.Errorf("delete section: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// DeleteField deletes a single field. Returns true if a row was deleted.
func (s *Store) DeleteField(vaultID, section, itemName string) (bool, error) {
	res, err := s.db.Exec(
		`DELETE FROM vault_items WHERE vault_id = ? AND section = ? AND item_name = ?`,
		vaultID, section, itemName,
	)
	if err != nil {
		return false, fmt.Errorf("delete field: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
