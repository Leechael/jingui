package db

import (
	"database/sql"
	"errors"
	"fmt"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// UpsertVaultItem inserts or updates a vault item.
func (s *Store) UpsertVaultItem(item *VaultItem) error {
	_, err := s.db.Exec(
		`INSERT INTO vault_items (app_id, item, secret_encrypted)
		 VALUES (?, ?, ?)
		 ON CONFLICT(app_id, item) DO UPDATE SET
		   secret_encrypted = excluded.secret_encrypted,
		   updated_at = CURRENT_TIMESTAMP`,
		item.Vault, item.Item, item.SecretEncrypted,
	)
	if err != nil {
		return fmt.Errorf("upsert vault item: %w", err)
	}
	return nil
}

// GetVaultItem retrieves a vault item by vault and item.
func (s *Store) GetVaultItem(vault, item string) (*VaultItem, error) {
	vi := &VaultItem{}
	err := s.db.QueryRow(
		`SELECT app_id, item, secret_encrypted, created_at, updated_at
		 FROM vault_items WHERE app_id = ? AND item = ?`, vault, item,
	).Scan(&vi.Vault, &vi.Item, &vi.SecretEncrypted, &vi.CreatedAt, &vi.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get vault item: %w", err)
	}
	return vi, nil
}

// ListVaultItems returns all vault items (metadata only, no encrypted blob).
func (s *Store) ListVaultItems() ([]VaultItem, error) {
	rows, err := s.db.Query(
		`SELECT app_id, item, created_at, updated_at FROM vault_items ORDER BY created_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("list vault items: %w", err)
	}
	defer rows.Close()

	var items []VaultItem
	for rows.Next() {
		var vi VaultItem
		if err := rows.Scan(&vi.Vault, &vi.Item, &vi.CreatedAt, &vi.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan vault item: %w", err)
		}
		items = append(items, vi)
	}
	return items, rows.Err()
}

// ListVaultItemsByVault returns all vault items for a given vault (metadata only).
func (s *Store) ListVaultItemsByVault(vault string) ([]VaultItem, error) {
	rows, err := s.db.Query(
		`SELECT app_id, item, created_at, updated_at FROM vault_items WHERE app_id = ? ORDER BY created_at`,
		vault,
	)
	if err != nil {
		return nil, fmt.Errorf("list vault items by vault: %w", err)
	}
	defer rows.Close()

	var items []VaultItem
	for rows.Next() {
		var vi VaultItem
		if err := rows.Scan(&vi.Vault, &vi.Item, &vi.CreatedAt, &vi.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan vault item: %w", err)
		}
		items = append(items, vi)
	}
	return items, rows.Err()
}

// ErrItemHasDependents is returned when deleting a vault item that still has tee_instances.
var ErrItemHasDependents = errors.New("vault item has dependent instances; delete them first or use ?cascade=true")

// DeleteVaultItem deletes a vault item by vault and item. Returns true if a row was deleted.
// Returns ErrItemHasDependents if foreign key constraints prevent deletion.
func (s *Store) DeleteVaultItem(vault, item string) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM vault_items WHERE app_id = ? AND item = ?`, vault, item)
	if err != nil {
		var sqliteErr *sqlite.Error
		if errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY {
			return false, ErrItemHasDependents
		}
		return false, fmt.Errorf("delete vault item: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// DeleteVaultItemCascade deletes a vault item and all dependent tee_instances in a transaction.
// Returns true if the item existed and was deleted.
func (s *Store) DeleteVaultItemCascade(vault, item string) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Delete tee_instances that reference this vault item
	if _, err := tx.Exec(`DELETE FROM tee_instances WHERE bound_app_id = ? AND bound_item = ?`, vault, item); err != nil {
		return false, fmt.Errorf("delete instances for item: %w", err)
	}

	// Delete the vault item itself
	res, err := tx.Exec(`DELETE FROM vault_items WHERE app_id = ? AND item = ?`, vault, item)
	if err != nil {
		return false, fmt.Errorf("delete vault item: %w", err)
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
