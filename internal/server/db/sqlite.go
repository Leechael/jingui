package db

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// Store wraps a SQLite database connection.
type Store struct {
	db *sql.DB
}

// NewStore opens or creates a SQLite database and runs migrations.
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	// Enable foreign key enforcement (off by default in SQLite)
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS apps (
			app_id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			service_type TEXT NOT NULL,
			required_scopes TEXT NOT NULL DEFAULT '',
			credentials_encrypted BLOB NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS user_secrets (
			app_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			secret_encrypted BLOB NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (app_id, user_id),
			FOREIGN KEY (app_id) REFERENCES apps(app_id)
		)`,
		`CREATE TABLE IF NOT EXISTS tee_instances (
				fid TEXT PRIMARY KEY,
				public_key BLOB NOT NULL UNIQUE,
				bound_app_id TEXT NOT NULL,
				bound_attestation_app_id TEXT NOT NULL,
				bound_user_id TEXT NOT NULL,
				label TEXT NOT NULL DEFAULT '',
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				last_used_at DATETIME,
				FOREIGN KEY (bound_app_id, bound_user_id) REFERENCES user_secrets(app_id, user_id)
			)`,
		`CREATE TABLE IF NOT EXISTS debug_policies (
			app_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			allow_read_debug INTEGER NOT NULL DEFAULT 1,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (app_id, user_id)
		)`,
	}

	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			return fmt.Errorf("exec migration: %w", err)
		}
	}

	// Upgrade legacy tee_instances schema (from older versions) to include:
	//   - UNIQUE(public_key)
	//   - bound_attestation_app_id column
	//   - FOREIGN KEY(bound_app_id, bound_user_id) -> user_secrets(app_id, user_id)
	if err := s.upgradeTEEInstancesSchema(); err != nil {
		return err
	}

	return nil
}

func (s *Store) upgradeTEEInstancesSchema() error {
	var tableSQL string
	if err := s.db.QueryRow(
		`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'tee_instances'`,
	).Scan(&tableSQL); err != nil {
		return fmt.Errorf("read tee_instances schema: %w", err)
	}

	schema := strings.ToLower(strings.Join(strings.Fields(tableSQL), " "))
	hasPubKeyUnique := strings.Contains(schema, "public_key blob not null unique")
	hasAttestationID := strings.Contains(schema, "bound_attestation_app_id text not null")
	hasCompositeFK := strings.Contains(schema, "foreign key (bound_app_id, bound_user_id) references user_secrets(app_id, user_id)")

	if hasPubKeyUnique && hasAttestationID && hasCompositeFK {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tee_instances schema upgrade: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`CREATE TABLE tee_instances_new (
		fid TEXT PRIMARY KEY,
		public_key BLOB NOT NULL UNIQUE,
		bound_app_id TEXT NOT NULL,
		bound_attestation_app_id TEXT NOT NULL,
		bound_user_id TEXT NOT NULL,
		label TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		last_used_at DATETIME,
		FOREIGN KEY (bound_app_id, bound_user_id) REFERENCES user_secrets(app_id, user_id)
	)`); err != nil {
		return fmt.Errorf("create tee_instances_new: %w", err)
	}

	if _, err := tx.Exec(`INSERT INTO tee_instances_new
		(fid, public_key, bound_app_id, bound_attestation_app_id, bound_user_id, label, created_at, last_used_at)
		SELECT fid, public_key, bound_app_id, '', bound_user_id, label, created_at, last_used_at
		FROM tee_instances`); err != nil {
		return fmt.Errorf("copy tee_instances data: %w", err)
	}

	if _, err := tx.Exec(`DROP TABLE tee_instances`); err != nil {
		return fmt.Errorf("drop old tee_instances: %w", err)
	}
	if _, err := tx.Exec(`ALTER TABLE tee_instances_new RENAME TO tee_instances`); err != nil {
		return fmt.Errorf("rename tee_instances_new: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tee_instances schema upgrade: %w", err)
	}
	return nil
}
