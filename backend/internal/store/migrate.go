package store

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// migrationsTable tracks which migrations have been applied.
const migrationsTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    TEXT PRIMARY KEY,
    applied_at TEXT NOT NULL
);`

// Migrate applies every embedded migration that has not yet run, in filename
// order. Each migration runs inside a transaction so a failure leaves the
// schema untouched. It is safe to call on every startup.
func (db *DB) Migrate(ctx context.Context) error {
	unlock, err := db.lockMigrations(ctx)
	if err != nil {
		return err
	}
	defer unlock()

	if _, err := db.sql.ExecContext(ctx, migrationsTable); err != nil {
		return fmt.Errorf("store: create migrations table: %w", err)
	}

	applied, err := db.appliedVersions(ctx)
	if err != nil {
		return err
	}

	files, err := migrationFiles()
	if err != nil {
		return err
	}

	for _, f := range files {
		if !db.migrationApplies(f) {
			continue
		}
		version := migrationVersion(f)
		if _, done := applied[version]; done {
			continue
		}
		body, err := migrationsFS.ReadFile("migrations/" + f)
		if err != nil {
			return fmt.Errorf("store: read migration %s: %w", f, err)
		}
		if err := db.runMigration(ctx, version, string(body)); err != nil {
			return fmt.Errorf("store: apply migration %s: %w", f, err)
		}
	}
	return nil
}

const migrationLockID int64 = 0x4b4549524f555445 // "KEIROUTE"

// lockMigrations serializes schema changes across application replicas. The
// advisory lock is held by a dedicated PostgreSQL connection for the complete
// migration run; SQLite already serializes DDL through its file lock.
func (db *DB) lockMigrations(ctx context.Context) (func(), error) {
	if db.dialect != DialectPostgres {
		return func() {}, nil
	}
	conn, err := db.sql.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: acquire migration connection: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", migrationLockID); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("store: acquire migration lock: %w", err)
	}
	return func() {
		_, _ = conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", migrationLockID)
		_ = conn.Close()
	}, nil
}

func (db *DB) migrationApplies(filename string) bool {
	switch {
	case strings.HasSuffix(filename, ".postgres.sql"):
		return db.dialect == DialectPostgres
	case strings.HasSuffix(filename, ".sqlite.sql"):
		return db.dialect == DialectSQLite
	default:
		return true
	}
}

func migrationVersion(filename string) string {
	version := strings.TrimSuffix(filename, ".sql")
	version = strings.TrimSuffix(version, ".postgres")
	version = strings.TrimSuffix(version, ".sqlite")
	return version
}

func (db *DB) runMigration(ctx context.Context, version, body string) error {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, stmt := range splitStatements(body) {
		if strings.TrimSpace(stmt) == "" {
			continue
		}

		const savepoint = "keirouter_migration_statement"
		if db.dialect == DialectPostgres {
			if _, err := tx.ExecContext(ctx, "SAVEPOINT "+savepoint); err != nil {
				return fmt.Errorf("create migration savepoint: %w", err)
			}
		}

		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			// PostgreSQL marks the transaction aborted after any statement error.
			// Restore the statement savepoint before tolerating a duplicate column.
			if isAddColumnAlreadyExists(stmt, err) {
				if db.dialect == DialectPostgres {
					if _, rollbackErr := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+savepoint); rollbackErr != nil {
						return fmt.Errorf("restore migration savepoint: %w", rollbackErr)
					}
					if _, releaseErr := tx.ExecContext(ctx, "RELEASE SAVEPOINT "+savepoint); releaseErr != nil {
						return fmt.Errorf("release migration savepoint: %w", releaseErr)
					}
				}
				continue
			}
			return fmt.Errorf("statement failed: %w\n%s", err, stmt)
		}
		if db.dialect == DialectPostgres {
			if _, err := tx.ExecContext(ctx, "RELEASE SAVEPOINT "+savepoint); err != nil {
				return fmt.Errorf("release migration savepoint: %w", err)
			}
		}
	}

	insert := db.rebind("INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)")
	if _, err := tx.ExecContext(ctx, insert, version, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return err
	}
	return tx.Commit()
}

// isAddColumnAlreadyExists reports whether err is the "column already exists"
// error produced by attempting an "ALTER TABLE ... ADD COLUMN ..." for a column
// that is already present. It is scoped to ADD COLUMN statements so genuine
// errors on other statements are never swallowed. The message substrings cover
// both engines: modernc SQLite emits "duplicate column name: <col>" and
// Postgres emits "column \"<col>\" of relation \"<table>\" already exists".
func isAddColumnAlreadyExists(stmt string, err error) bool {
	if err == nil {
		return false
	}
	if !strings.Contains(strings.ToUpper(stmt), "ADD COLUMN") {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate column name") ||
		strings.Contains(msg, "already exists")
}

func (db *DB) appliedVersions(ctx context.Context) (map[string]struct{}, error) {
	rows, err := db.sql.QueryContext(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("store: read applied migrations: %w", err)
	}
	defer rows.Close()

	applied := map[string]struct{}{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = struct{}{}
	}
	return applied, rows.Err()
}

func migrationFiles() ([]string, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("store: list migrations: %w", err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	return files, nil
}

// splitStatements breaks a SQL file into individual statements on semicolons.
//
// Comments are stripped from the whole body *before* splitting, because a "--"
// line comment may itself contain a semicolon ("...stored; plaintext is shown")
// which would otherwise split a statement incorrectly. The schema avoids
// semicolons inside string literals, so splitting the comment-free SQL on ";"
// is sufficient and keeps the runner dependency-free.
func splitStatements(body string) []string {
	clean := stripSQLComments(body)
	parts := strings.Split(clean, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return out
}

// stripSQLComments removes whole-line "--" comments. It intentionally only
// handles line comments (the schema uses no inline trailing comments), which is
// enough to keep semicolons inside prose from corrupting statement splitting.
func stripSQLComments(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}
