package db

import (
	"context"
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

	// SQLite PRAGMAs like foreign_keys and journal_mode are per-connection.
	// Limit pool to 1 connection so all PRAGMAs apply consistently and
	// migrations can safely toggle foreign_keys on the same connection.
	db.SetMaxOpenConns(1)

	// Enable WAL mode for better concurrency (persistent, stored in DB file)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	// Enable foreign key enforcement (off by default in SQLite, per-connection)
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
		`CREATE TABLE IF NOT EXISTS vault_items (
			app_id TEXT NOT NULL,
			item TEXT NOT NULL,
			secret_encrypted BLOB NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (app_id, item),
			FOREIGN KEY (app_id) REFERENCES apps(app_id)
		)`,
		`CREATE TABLE IF NOT EXISTS tee_instances (
				fid TEXT PRIMARY KEY,
				public_key BLOB NOT NULL UNIQUE,
				bound_app_id TEXT NOT NULL,
				bound_attestation_app_id TEXT NOT NULL,
				bound_item TEXT NOT NULL,
				label TEXT NOT NULL DEFAULT '',
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				last_used_at DATETIME,
				FOREIGN KEY (bound_app_id, bound_item) REFERENCES vault_items(app_id, item)
			)`,
		`CREATE TABLE IF NOT EXISTS debug_policies (
			app_id TEXT NOT NULL,
			item TEXT NOT NULL,
			allow_read_debug INTEGER NOT NULL DEFAULT 1,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (app_id, item)
		)`,
	}

	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			return fmt.Errorf("exec migration: %w", err)
		}
	}

	// Upgrade legacy user_secrets → vault_items schema.
	// IMPORTANT: must run before upgradeTEEInstancesSchema which references vault_items.
	if err := s.upgradeVaultItemsSchema(); err != nil {
		return err
	}

	// Upgrade legacy tee_instances schema (from older versions) to include:
	//   - UNIQUE(public_key)
	//   - bound_attestation_app_id column
	//   - FOREIGN KEY(bound_app_id, bound_item) -> vault_items(app_id, item)
	if err := s.upgradeTEEInstancesSchema(); err != nil {
		return err
	}

	return nil
}

// upgradeVaultItemsSchema migrates from the old user_secrets/bound_user_id schema
// to the new vault_items/bound_item schema. Skips if user_secrets table does not exist.
func (s *Store) upgradeVaultItemsSchema() error {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'user_secrets'`,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("check user_secrets existence: %w", err)
	}
	if count == 0 {
		return nil // fresh DB with vault_items, nothing to migrate
	}

	// Pin a single connection so PRAGMA foreign_keys=OFF and the transaction
	// are guaranteed to execute on the same connection. SQLite PRAGMAs are
	// per-connection; without pinning, the pool could dispatch them to
	// different connections.
	ctx := context.Background()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("pin connection for migration: %w", err)
	}
	defer conn.Close()

	// Must disable FK checks before table recreation.
	// Use defer to guarantee re-enabling on any exit path.
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
		return fmt.Errorf("disable foreign keys for migration: %w", err)
	}
	defer conn.ExecContext(ctx, "PRAGMA foreign_keys=ON")

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin vault_items migration: %w", err)
	}
	defer tx.Rollback()

	// Migrate user_secrets → vault_items (table already created by DDL above)
	if _, err := tx.Exec(
		`INSERT OR IGNORE INTO vault_items (app_id, item, secret_encrypted, created_at, updated_at)
		 SELECT app_id, user_id, secret_encrypted, created_at, updated_at FROM user_secrets`,
	); err != nil {
		return fmt.Errorf("copy user_secrets to vault_items: %w", err)
	}
	if _, err := tx.Exec(`DROP TABLE user_secrets`); err != nil {
		return fmt.Errorf("drop user_secrets: %w", err)
	}

	// Migrate tee_instances: bound_user_id → bound_item
	var teeTableSQL string
	if err := tx.QueryRow(
		`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'tee_instances'`,
	).Scan(&teeTableSQL); err == nil {
		schema := strings.ToLower(strings.Join(strings.Fields(teeTableSQL), " "))
		if strings.Contains(schema, "bound_user_id") {
			if _, err := tx.Exec(`CREATE TABLE tee_instances_new (
				fid TEXT PRIMARY KEY,
				public_key BLOB NOT NULL UNIQUE,
				bound_app_id TEXT NOT NULL,
				bound_attestation_app_id TEXT NOT NULL,
				bound_item TEXT NOT NULL,
				label TEXT NOT NULL DEFAULT '',
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				last_used_at DATETIME,
				FOREIGN KEY (bound_app_id, bound_item) REFERENCES vault_items(app_id, item)
			)`); err != nil {
				return fmt.Errorf("create tee_instances_new: %w", err)
			}
			if _, err := tx.Exec(`INSERT INTO tee_instances_new
				(fid, public_key, bound_app_id, bound_attestation_app_id, bound_item, label, created_at, last_used_at)
				SELECT fid, public_key, bound_app_id,
					COALESCE(bound_attestation_app_id, ''),
					bound_user_id, label, created_at, last_used_at
				FROM tee_instances`); err != nil {
				return fmt.Errorf("copy tee_instances data: %w", err)
			}
			if _, err := tx.Exec(`DROP TABLE tee_instances`); err != nil {
				return fmt.Errorf("drop old tee_instances: %w", err)
			}
			if _, err := tx.Exec(`ALTER TABLE tee_instances_new RENAME TO tee_instances`); err != nil {
				return fmt.Errorf("rename tee_instances_new: %w", err)
			}
		}
	}

	// Migrate debug_policies: user_id → item
	var debugTableSQL string
	if err := tx.QueryRow(
		`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'debug_policies'`,
	).Scan(&debugTableSQL); err == nil {
		schema := strings.ToLower(strings.Join(strings.Fields(debugTableSQL), " "))
		if strings.Contains(schema, "user_id") {
			if _, err := tx.Exec(`CREATE TABLE debug_policies_new (
				app_id TEXT NOT NULL,
				item TEXT NOT NULL,
				allow_read_debug INTEGER NOT NULL DEFAULT 1,
				updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY (app_id, item)
			)`); err != nil {
				return fmt.Errorf("create debug_policies_new: %w", err)
			}
			if _, err := tx.Exec(`INSERT INTO debug_policies_new (app_id, item, allow_read_debug, updated_at)
				SELECT app_id, user_id, allow_read_debug, updated_at FROM debug_policies`); err != nil {
				return fmt.Errorf("copy debug_policies data: %w", err)
			}
			if _, err := tx.Exec(`DROP TABLE debug_policies`); err != nil {
				return fmt.Errorf("drop old debug_policies: %w", err)
			}
			if _, err := tx.Exec(`ALTER TABLE debug_policies_new RENAME TO debug_policies`); err != nil {
				return fmt.Errorf("rename debug_policies_new: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit vault_items migration: %w", err)
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
	hasCompositeFK := strings.Contains(schema, "foreign key (bound_app_id, bound_item) references vault_items(app_id, item)")

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
		bound_item TEXT NOT NULL,
		label TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		last_used_at DATETIME,
		FOREIGN KEY (bound_app_id, bound_item) REFERENCES vault_items(app_id, item)
	)`); err != nil {
		return fmt.Errorf("create tee_instances_new: %w", err)
	}

	if _, err := tx.Exec(`INSERT INTO tee_instances_new
		(fid, public_key, bound_app_id, bound_attestation_app_id, bound_item, label, created_at, last_used_at)
		SELECT fid, public_key, bound_app_id, COALESCE(bound_attestation_app_id, ''), bound_item, label, created_at, last_used_at
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
