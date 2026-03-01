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
	// Check if we need to upgrade from the old schema (v1).
	if err := s.upgradeToSchemaV2(); err != nil {
		return err
	}

	migrations := []string{
		`CREATE TABLE IF NOT EXISTS vaults (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS vault_items (
			rowid INTEGER PRIMARY KEY AUTOINCREMENT,
			vault_id TEXT NOT NULL,
			item_name TEXT NOT NULL,
			section TEXT NOT NULL DEFAULT '',
			value TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(vault_id, section, item_name),
			FOREIGN KEY (vault_id) REFERENCES vaults(id)
		)`,
		`CREATE TABLE IF NOT EXISTS tee_instances (
			fid TEXT PRIMARY KEY,
			label TEXT NOT NULL DEFAULT '',
			public_key BLOB NOT NULL UNIQUE,
			dstack_app_id TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_used_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS vault_instance_access (
			vault_id TEXT NOT NULL,
			fid TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (vault_id, fid),
			FOREIGN KEY (vault_id) REFERENCES vaults(id),
			FOREIGN KEY (fid) REFERENCES tee_instances(fid)
		)`,
		`CREATE TABLE IF NOT EXISTS debug_policies (
			vault_id TEXT NOT NULL,
			fid TEXT NOT NULL,
			allow_read INTEGER NOT NULL DEFAULT 1,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (vault_id, fid),
			FOREIGN KEY (vault_id) REFERENCES vaults(id),
			FOREIGN KEY (fid) REFERENCES tee_instances(fid)
		)`,
	}

	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			return fmt.Errorf("exec migration: %w", err)
		}
	}

	return nil
}

// upgradeToSchemaV2 detects the old v1 schema (apps table) and migrates data
// to the new vault-centric schema. Skips if the apps table does not exist.
func (s *Store) upgradeToSchemaV2() error {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'apps'`,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("check apps table existence: %w", err)
	}
	if count == 0 {
		return nil // fresh DB or already migrated
	}

	// Also handle even older schema (user_secrets)
	var hasUserSecrets int
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'user_secrets'`,
	).Scan(&hasUserSecrets); err != nil {
		return fmt.Errorf("check user_secrets table existence: %w", err)
	}

	// Pin a single connection so PRAGMA foreign_keys=OFF and the transaction
	// are guaranteed to execute on the same connection.
	ctx := context.Background()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("pin connection for v2 migration: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
		return fmt.Errorf("disable foreign keys for v2 migration: %w", err)
	}
	defer conn.ExecContext(ctx, "PRAGMA foreign_keys=ON")

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin v2 migration: %w", err)
	}
	defer tx.Rollback()

	// 1. Create new vaults table
	if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS vaults (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("create vaults table: %w", err)
	}

	// 2. Copy apps → vaults (id=app_id, name, created_at)
	if _, err := tx.Exec(
		`INSERT OR IGNORE INTO vaults (id, name, created_at)
		 SELECT app_id, name, created_at FROM apps`,
	); err != nil {
		return fmt.Errorf("copy apps to vaults: %w", err)
	}

	// 3. Create new tee_instances table
	if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS tee_instances_v2 (
		fid TEXT PRIMARY KEY,
		label TEXT NOT NULL DEFAULT '',
		public_key BLOB NOT NULL UNIQUE,
		dstack_app_id TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		last_used_at DATETIME
	)`); err != nil {
		return fmt.Errorf("create tee_instances_v2: %w", err)
	}

	// 4. Copy tee_instances → new schema
	// Check if old tee_instances exists and has bound_attestation_app_id
	var teeCount int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'tee_instances'`).Scan(&teeCount); err != nil {
		return fmt.Errorf("check tee_instances table existence: %w", err)
	}
	if teeCount > 0 {
		var teeTableSQL string
		if err := tx.QueryRow(
			`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'tee_instances'`,
		).Scan(&teeTableSQL); err == nil {
			schema := strings.ToLower(strings.Join(strings.Fields(teeTableSQL), " "))
			hasAttestationID := strings.Contains(schema, "bound_attestation_app_id")

			if hasAttestationID {
				if _, err := tx.Exec(`INSERT OR IGNORE INTO tee_instances_v2
					(fid, label, public_key, dstack_app_id, created_at, last_used_at)
					SELECT fid, label, public_key, bound_attestation_app_id, created_at, last_used_at
					FROM tee_instances`); err != nil {
					return fmt.Errorf("copy tee_instances to v2: %w", err)
				}
			} else {
				// Very old schema without bound_attestation_app_id
				if _, err := tx.Exec(`INSERT OR IGNORE INTO tee_instances_v2
					(fid, label, public_key, dstack_app_id, created_at, last_used_at)
					SELECT fid, label, public_key, '', created_at, last_used_at
					FROM tee_instances`); err != nil {
					return fmt.Errorf("copy old tee_instances to v2: %w", err)
				}
			}
		}
	}

	// 5. Create vault_instance_access from old bound_app_id relationships
	if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS vault_instance_access (
		vault_id TEXT NOT NULL,
		fid TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (vault_id, fid),
		FOREIGN KEY (vault_id) REFERENCES vaults(id),
		FOREIGN KEY (fid) REFERENCES tee_instances_v2(fid)
	)`); err != nil {
		return fmt.Errorf("create vault_instance_access: %w", err)
	}

	if teeCount > 0 {
		var teeTableSQL string
		if err := tx.QueryRow(
			`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'tee_instances'`,
		).Scan(&teeTableSQL); err == nil {
			schema := strings.ToLower(strings.Join(strings.Fields(teeTableSQL), " "))
			if strings.Contains(schema, "bound_app_id") {
				if _, err := tx.Exec(`INSERT OR IGNORE INTO vault_instance_access (vault_id, fid)
					SELECT DISTINCT bound_app_id, fid FROM tee_instances
					WHERE bound_app_id IN (SELECT id FROM vaults)`); err != nil {
					return fmt.Errorf("create vault_instance_access entries: %w", err)
				}
			}
		}
	}

	// 6. Drop old tables
	for _, table := range []string{"tee_instances", "vault_items", "apps", "debug_policies"} {
		if _, err := tx.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", table)); err != nil {
			return fmt.Errorf("drop old %s: %w", table, err)
		}
	}

	// Also drop user_secrets if it exists
	if hasUserSecrets > 0 {
		if _, err := tx.Exec(`DROP TABLE IF EXISTS user_secrets`); err != nil {
			return fmt.Errorf("drop user_secrets: %w", err)
		}
	}

	// 7. Rename tee_instances_v2 → tee_instances
	if _, err := tx.Exec(`ALTER TABLE tee_instances_v2 RENAME TO tee_instances`); err != nil {
		return fmt.Errorf("rename tee_instances_v2: %w", err)
	}

	// 8. Recreate vault_instance_access with correct FK to renamed tee_instances
	// (SQLite FKs reference the table name at creation time, and the rename keeps them valid)

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit v2 migration: %w", err)
	}

	return nil
}
