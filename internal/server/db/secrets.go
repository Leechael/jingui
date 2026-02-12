package db

import (
	"database/sql"
	"fmt"
)

// UpsertUserSecret inserts or updates a user secret.
func (s *Store) UpsertUserSecret(secret *UserSecret) error {
	_, err := s.db.Exec(
		`INSERT INTO user_secrets (app_id, user_id, secret_encrypted)
		 VALUES (?, ?, ?)
		 ON CONFLICT(app_id, user_id) DO UPDATE SET
		   secret_encrypted = excluded.secret_encrypted,
		   updated_at = CURRENT_TIMESTAMP`,
		secret.AppID, secret.UserID, secret.SecretEncrypted,
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
	).Scan(&us.AppID, &us.UserID, &us.SecretEncrypted, &us.CreatedAt, &us.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user secret: %w", err)
	}
	return us, nil
}
