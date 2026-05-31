// Package store is KeiRouter's persistence layer. It opens a database
// connection (SQLite by default, Postgres optionally), applies embedded
// migrations on startup, and exposes typed repositories for each domain.
//
// The same SQL runs against both engines wherever possible; dialect-specific
// statements are selected via the Dialect value. This keeps the local
// single-binary experience zero-config while letting team/VPS deployments point
// at Postgres without code changes.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver registered as "pgx"
	_ "modernc.org/sqlite"             // pure-Go SQLite driver, no CGO

	"github.com/mydisha/keirouter/backend/internal/config"
)

// Dialect identifies the active SQL engine.
type Dialect string

const (
	DialectSQLite   Dialect = "sqlite"
	DialectPostgres Dialect = "postgres"
)

// DB wraps the sql.DB handle together with its dialect so repositories can
// adapt placeholder syntax and engine-specific statements.
type DB struct {
	sql     *sql.DB
	dialect Dialect
}

// Open connects using the given database configuration and verifies the
// connection. The caller is responsible for calling Close.
func Open(ctx context.Context, cfg config.DatabaseConfig, dataDir string) (*DB, error) {
	dialect := Dialect(cfg.Driver)

	var (
		driverName string
		dsn        string
	)
	switch dialect {
	case DialectSQLite:
		driverName = "sqlite"
		dsn = sqliteDSN(cfg, dataDir)
	case DialectPostgres:
		driverName = "pgx"
		dsn = cfg.DSN
	default:
		return nil, fmt.Errorf("store: unsupported driver %q", cfg.Driver)
	}

	sqlDB, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", dialect, err)
	}

	// SQLite is single-writer; cap connections to avoid SQLITE_BUSY. Postgres
	// uses the configured pool.
	if dialect == DialectSQLite {
		sqlDB.SetMaxOpenConns(1)
	} else {
		if cfg.MaxOpenConns > 0 {
			sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
		}
		if cfg.MaxIdleConns > 0 {
			sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
		}
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("store: ping %s: %w", dialect, err)
	}

	db := &DB{sql: sqlDB, dialect: dialect}
	if dialect == DialectSQLite {
		if err := db.applySQLitePragmas(ctx); err != nil {
			_ = sqlDB.Close()
			return nil, err
		}
	}
	return db, nil
}

// sqliteDSN builds the SQLite connection string. An empty DSN defaults to a
// file under the data directory; ":memory:" is honored for tests.
func sqliteDSN(cfg config.DatabaseConfig, dataDir string) string {
	path := cfg.DSN
	if path == "" {
		path = dataDir + "/keirouter.db"
	}
	if path == ":memory:" {
		// Shared-cache in-memory keeps the schema visible across the pool.
		return "file::memory:?cache=shared"
	}
	// WAL + busy timeout improve concurrent read/write behavior.
	return "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)"
}

func (db *DB) applySQLitePragmas(ctx context.Context) error {
	pragmas := []string{
		"PRAGMA foreign_keys = ON;",
		"PRAGMA journal_mode = WAL;",
		"PRAGMA synchronous = NORMAL;",
	}
	for _, p := range pragmas {
		if _, err := db.sql.ExecContext(ctx, p); err != nil {
			return fmt.Errorf("store: pragma %q: %w", p, err)
		}
	}
	return nil
}

// SQL exposes the underlying handle for repositories within this package.
func (db *DB) SQL() *sql.DB { return db.sql }

// Dialect reports the active engine.
func (db *DB) Dialect() Dialect { return db.dialect }

// Close releases the connection pool.
func (db *DB) Close() error { return db.sql.Close() }

// rebind converts '?' placeholders to the engine's native form. SQLite accepts
// '?' directly; Postgres needs $1, $2, ... positional placeholders.
func (db *DB) rebind(query string) string {
	if db.dialect != DialectPostgres {
		return query
	}
	var b []byte
	n := 0
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			n++
			b = append(b, '$')
			b = append(b, []byte(fmt.Sprintf("%d", n))...)
			continue
		}
		b = append(b, query[i])
	}
	return string(b)
}