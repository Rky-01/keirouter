// Package config loads KeiRouter configuration from defaults, an optional YAML
// file, and environment variables (in increasing order of precedence).
//
// Env vars are prefixed KEIROUTER_ and use double underscores for nesting, e.g.
// KEIROUTER_SERVER__PORT=8080 sets server.port.
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

// Config is the fully-resolved application configuration.
type Config struct {
	Server   ServerConfig   `koanf:"server"`
	Database DatabaseConfig `koanf:"database"`
	Security SecurityConfig `koanf:"security"`
	Cache    CacheConfig    `koanf:"cache"`
	Log      LogConfig      `koanf:"log"`
	Data     DataConfig     `koanf:"data"`
}

// ServerConfig controls the HTTP listener.
type ServerConfig struct {
	Host string `koanf:"host"`
	Port int    `koanf:"port"`
	// StreamStallTimeout aborts a stream that produces no bytes for this long.
	StreamStallTimeout time.Duration `koanf:"stream_stall_timeout"`
	// RequestTimeout bounds non-streaming upstream calls.
	RequestTimeout time.Duration `koanf:"request_timeout"`
	// CORSOrigins lists allowed dashboard origins ("*" permitted for local).
	CORSOrigins []string `koanf:"cors_origins"`
}

// DatabaseConfig selects and configures the persistence backend.
type DatabaseConfig struct {
	// Driver is "sqlite" (default, zero-config) or "postgres".
	Driver string `koanf:"driver"`
	// DSN is the connection string. Empty with sqlite => <data_dir>/keirouter.db.
	DSN string `koanf:"dsn"`
	// MaxOpenConns/MaxIdleConns tune the pool (postgres-relevant).
	MaxOpenConns int `koanf:"max_open_conns"`
	MaxIdleConns int `koanf:"max_idle_conns"`
}

// SecurityConfig holds auth and encryption settings.
type SecurityConfig struct {
	// MasterKey is the base64 root key for envelope encryption of credentials.
	// If empty, KeiRouter generates one on first run and persists it to
	// <data_dir>/master.key (0600). Provide via env/KMS in production.
	MasterKey string `koanf:"master_key"`
	// JWTSecret signs dashboard session tokens. Auto-generated if empty.
	JWTSecret string `koanf:"jwt_secret"`
	// DashboardPasswordHash is the argon2id hash of the dashboard password.
	DashboardPasswordHash string `koanf:"dashboard_password_hash"`
	// SessionTTL is how long a dashboard session stays valid.
	SessionTTL time.Duration `koanf:"session_ttl"`
	// BindLoopbackOnly rejects non-loopback dashboard/API access when true.
	BindLoopbackOnly bool `koanf:"bind_loopback_only"`
}

// CacheConfig configures the semantic response cache (Pro feature).
type CacheConfig struct {
	Enabled bool `koanf:"enabled"`
	// Backend is "memory" (local HNSW) or "redis".
	Backend string `koanf:"backend"`
	// RedisURL is used when Backend == "redis".
	RedisURL string `koanf:"redis_url"`
	// SimilarityThreshold in [0,1]; a candidate hit must score >= this.
	SimilarityThreshold float64 `koanf:"similarity_threshold"`
	// TTL bounds cached entry lifetime.
	TTL time.Duration `koanf:"ttl"`
}

// LogConfig controls structured logging.
type LogConfig struct {
	// Level: debug, info, warn, error.
	Level string `koanf:"level"`
	// Format: json or text.
	Format string `koanf:"format"`
}

// DataConfig controls on-disk state location.
type DataConfig struct {
	// Dir is the root data directory. Empty => OS-specific default.
	Dir string `koanf:"dir"`
}

// Default returns the baseline configuration applied before file/env overrides.
func Default() Config {
	return Config{
		Server: ServerConfig{
			Host:               "127.0.0.1",
			Port:               20180,
			StreamStallTimeout: 30 * time.Second,
			RequestTimeout:     5 * time.Minute,
			CORSOrigins:        []string{"*"},
		},
		Database: DatabaseConfig{
			Driver:       "sqlite",
			MaxOpenConns: 1, // sqlite: serialize writers to avoid SQLITE_BUSY
			MaxIdleConns: 1,
		},
		Security: SecurityConfig{
			SessionTTL:       24 * time.Hour,
			BindLoopbackOnly: true,
		},
		Cache: CacheConfig{
			Enabled:             false,
			Backend:             "memory",
			SimilarityThreshold: 0.95,
			TTL:                 time.Hour,
		},
		Log: LogConfig{Level: "info", Format: "text"},
	}
}

// Load resolves configuration. filePath may be empty to skip file loading.
func Load(filePath string) (Config, error) {
	k := koanf.New(".")

	// 1. Defaults.
	if err := k.Load(structs.Provider(Default(), "koanf"), nil); err != nil {
		return Config{}, fmt.Errorf("load defaults: %w", err)
	}

	// 2. Optional YAML file.
	if filePath != "" {
		if err := k.Load(file.Provider(filePath), yaml.Parser()); err != nil {
			return Config{}, fmt.Errorf("load config file %q: %w", filePath, err)
		}
	}

	// 3. Environment overrides: KEIROUTER_SERVER__PORT -> server.port.
	envProvider := env.Provider("KEIROUTER_", ".", func(s string) string {
		s = strings.TrimPrefix(s, "KEIROUTER_")
		s = strings.ReplaceAll(s, "__", ".")
		return strings.ToLower(s)
	})
	if err := k.Load(envProvider, nil); err != nil {
		return Config{}, fmt.Errorf("load env: %w", err)
	}

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	switch c.Database.Driver {
	case "sqlite", "postgres":
	default:
		return fmt.Errorf("database.driver must be sqlite or postgres, got %q", c.Database.Driver)
	}
	if c.Database.Driver == "postgres" && c.Database.DSN == "" {
		return fmt.Errorf("database.dsn is required when driver=postgres")
	}
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port out of range: %d", c.Server.Port)
	}
	return nil
}

// Addr returns the host:port the server binds to.
func (c Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}