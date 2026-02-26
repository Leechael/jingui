package db

import (
	"database/sql"
	"errors"
	"fmt"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// UpsertUserSecret inserts or updates a user secret.
func (s *Store) UpsertUserSecret(secret *UserSecret) error {
	_, err := s.db.Exec(
		`INSERT INTO user_secrets (app_id, user_id, secret_encrypted)
		 VALUES (?, ?, ?)
		 ON CONFLICT(app_id, user_id) DO UPDATE SET
		   secret_encrypted = excluded.secret_encrypted,
		   updated_at = CURRENT_TIMESTAMP`,
		secret.Vault, secret.UserID, secret.SecretEncrypted,
	)
	if err != nil {
		return fmt.Errorf("upsert user secret: %w", err)
	}
	return nil
}

// GetUserSecret retrieves a user secret by app_id and user_id.
func (s *Store) GetUserSecret(appID, userID string) (*UserSecret, error) {
	us := &UserSecret{}
	err := s.db.QueryRow(
		`SELECT app_id, user_id, secret_encrypted, created_at, updated_at
		 FROM user_secrets WHERE app_id = ? AND user_id = ?`, appID, userID,
	).Scan(&us.Vault, &us.UserID, &us.SecretEncrypted, &us.CreatedAt, &us.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user secret: %w", err)
	}
	return us, nil
}

// ListUserSecrets returns all user secrets (metadata only, no encrypted blob).
func (s *Store) ListUserSecrets() ([]UserSecret, error) {
	rows, err := s.db.Query(
		`SELECT app_id, user_id, created_at, updated_at FROM user_secrets ORDER BY created_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("list user secrets: %w", err)
	}
	defer rows.Close()

	var secrets []UserSecret
	for rows.Next() {
		var us UserSecret
		if err := rows.Scan(&us.Vault, &us.UserID, &us.CreatedAt, &us.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan user secret: %w", err)
		}
		secrets = append(secrets, us)
	}
	return secrets, rows.Err()
}

// ListUserSecretsByApp returns all user secrets for a given app (metadata only).
func (s *Store) ListUserSecretsByApp(appID string) ([]UserSecret, error) {
	rows, err := s.db.Query(
		`SELECT app_id, user_id, created_at, updated_at FROM user_secrets WHERE app_id = ? ORDER BY created_at`,
		appID,
	)
	if err != nil {
		return nil, fmt.Errorf("list user secrets by app: %w", err)
	}
	defer rows.Close()

	var secrets []UserSecret
	for rows.Next() {
		var us UserSecret
		if err := rows.Scan(&us.Vault, &us.UserID, &us.CreatedAt, &us.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan user secret: %w", err)
		}
		secrets = append(secrets, us)
	}
	return secrets, rows.Err()
}

// ErrSecretHasDependents is returned when deleting a user secret that still has tee_instances.
var ErrSecretHasDependents = errors.New("user secret has dependent instances; delete them first or use ?cascade=true")

// DeleteUserSecret deletes a user secret by app_id and user_id. Returns true if a row was deleted.
// Returns ErrSecretHasDependents if foreign key constraints prevent deletion.
func (s *Store) DeleteUserSecret(appID, userID string) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM user_secrets WHERE app_id = ? AND user_id = ?`, appID, userID)
	if err != nil {
		var sqliteErr *sqlite.Error
		if errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY {
			return false, ErrSecretHasDependents
		}
		return false, fmt.Errorf("delete user secret: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// DeleteUserSecretCascade deletes a user secret and all dependent tee_instances in a transaction.
// Returns true if the secret existed and was deleted.
func (s *Store) DeleteUserSecretCascade(appID, userID string) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Delete tee_instances that reference this user_secret
	if _, err := tx.Exec(`DELETE FROM tee_instances WHERE bound_app_id = ? AND bound_user_id = ?`, appID, userID); err != nil {
		return false, fmt.Errorf("delete instances for secret: %w", err)
	}

	// Delete the user_secret itself
	res, err := tx.Exec(`DELETE FROM user_secrets WHERE app_id = ? AND user_id = ?`, appID, userID)
	if err != nil {
		return false, fmt.Errorf("delete user secret: %w", err)
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
