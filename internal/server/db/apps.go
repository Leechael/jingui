package db

import (
	"database/sql"
	"fmt"
)

// CreateApp inserts a new app into the database.
func (s *Store) CreateApp(app *App) error {
	_, err := s.db.Exec(
		`INSERT INTO apps (app_id, name, service_type, required_scopes, credentials_encrypted)
		 VALUES (?, ?, ?, ?, ?)`,
		app.AppID, app.Name, app.ServiceType, app.RequiredScopes, app.CredentialsEncrypted,
	)
	if err != nil {
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
	).Scan(&app.AppID, &app.Name, &app.ServiceType, &app.RequiredScopes, &app.CredentialsEncrypted, &app.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get app: %w", err)
	}
	return app, nil
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
		if err := rows.Scan(&a.AppID, &a.Name, &a.ServiceType, &a.RequiredScopes, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan app: %w", err)
		}
		apps = append(apps, a)
	}
	return apps, rows.Err()
}
