package db

import (
	"database/sql"
	"errors"
	"fmt"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// Sentinel errors for app operations.
var (
	ErrAppDuplicate = errors.New("app already exists")
)

// CreateApp inserts a new app into the database.
func (s *Store) CreateApp(app *App) error {
	_, err := s.db.Exec(
		`INSERT INTO apps (app_id, name, service_type, required_scopes, credentials_encrypted)
		 VALUES (?, ?, ?, ?, ?)`,
		app.Vault, app.Name, app.ServiceType, app.RequiredScopes, app.CredentialsEncrypted,
	)
	if err != nil {
		var sqliteErr *sqlite.Error
		if errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY {
			return ErrAppDuplicate
		}
		return fmt.Errorf("insert app: %w", err)
	}
	return nil
}

// GetApp retrieves an app by its ID.
func (s *Store) GetApp(appID string) (*App, error) {
	app := &App{}
	err := s.db.QueryRow(
		`SELECT app_id, name, service_type, required_scopes, credentials_encrypted, created_at
		 FROM apps WHERE app_id = ?`, appID,
	).Scan(&app.Vault, &app.Name, &app.ServiceType, &app.RequiredScopes, &app.CredentialsEncrypted, &app.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get app: %w", err)
	}
	return app, nil
}

// UpdateApp updates app metadata and encrypted credentials.
// Returns true if an existing row was updated.
func (s *Store) UpdateApp(app *App) (bool, error) {
	res, err := s.db.Exec(
		`UPDATE apps
		 SET name = ?, service_type = ?, required_scopes = ?, credentials_encrypted = ?
		 WHERE app_id = ?`,
		app.Name, app.ServiceType, app.RequiredScopes, app.CredentialsEncrypted, app.Vault,
	)
	if err != nil {
		return false, fmt.Errorf("update app: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ListApps returns all registered apps.
func (s *Store) ListApps() ([]App, error) {
	rows, err := s.db.Query(
		`SELECT app_id, name, service_type, required_scopes, created_at FROM apps ORDER BY created_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("list apps: %w", err)
	}
	defer rows.Close()

	var apps []App
	for rows.Next() {
		var a App
		if err := rows.Scan(&a.Vault, &a.Name, &a.ServiceType, &a.RequiredScopes, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan app: %w", err)
		}
		apps = append(apps, a)
	}
	return apps, rows.Err()
}

// ErrAppHasDependents is returned when deleting an app that still has vault_items.
var ErrAppHasDependents = errors.New("app has dependent records; delete them first or use ?cascade=true")

// DeleteApp deletes an app by ID. Returns true if a row was deleted.
// Returns ErrAppHasDependents if foreign key constraints prevent deletion.
func (s *Store) DeleteApp(appID string) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM apps WHERE app_id = ?`, appID)
	if err != nil {
		var sqliteErr *sqlite.Error
		if errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY {
			return false, ErrAppHasDependents
		}
		return false, fmt.Errorf("delete app: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// DeleteAppCascade deletes an app and all its dependent vault_items and tee_instances in a transaction.
// Returns true if the app existed and was deleted.
func (s *Store) DeleteAppCascade(appID string) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Delete tee_instances that reference vault_items of this app
	if _, err := tx.Exec(`DELETE FROM tee_instances WHERE bound_app_id = ?`, appID); err != nil {
		return false, fmt.Errorf("delete instances for app: %w", err)
	}

	// Delete vault_items for this app
	if _, err := tx.Exec(`DELETE FROM vault_items WHERE app_id = ?`, appID); err != nil {
		return false, fmt.Errorf("delete items for app: %w", err)
	}

	// Delete the app itself
	res, err := tx.Exec(`DELETE FROM apps WHERE app_id = ?`, appID)
	if err != nil {
		return false, fmt.Errorf("delete app: %w", err)
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
