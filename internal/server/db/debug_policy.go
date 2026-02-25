package db

import (
	"database/sql"
	"fmt"
)

func (s *Store) UpsertDebugPolicy(appID, userID string, allow bool) error {
	allowInt := 0
	if allow {
		allowInt = 1
	}
	_, err := s.db.Exec(
		`INSERT INTO debug_policies (app_id, user_id, allow_read_debug)
		 VALUES (?, ?, ?)
		 ON CONFLICT(app_id, user_id) DO UPDATE SET
			allow_read_debug = excluded.allow_read_debug,
			updated_at = CURRENT_TIMESTAMP`,
		appID, userID, allowInt,
	)
	if err != nil {
		return fmt.Errorf("upsert debug policy: %w", err)
	}
	return nil
}

func (s *Store) GetDebugPolicy(appID, userID string) (*DebugPolicy, error) {
	p := &DebugPolicy{}
	var allowInt int
	err := s.db.QueryRow(
		`SELECT app_id, user_id, allow_read_debug, updated_at
		 FROM debug_policies WHERE app_id = ? AND user_id = ?`, appID, userID,
	).Scan(&p.AppID, &p.UserID, &allowInt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get debug policy: %w", err)
	}
	p.AllowReadDebug = allowInt != 0
	return p, nil
}
