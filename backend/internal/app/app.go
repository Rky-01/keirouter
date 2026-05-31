// Package app wires KeiRouter's components into a runnable application: it
// opens the store, runs migrations, loads or generates the master key, and
// constructs the gateway with its full dependency graph.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/mydisha/keirouter/backend/internal/auth"
	"github.com/mydisha/keirouter/backend/internal/budget"
	"github.com/mydisha/keirouter/backend/internal/cache"
	"github.com/mydisha/keirouter/backend/internal/config"
	"github.com/mydisha/keirouter/backend/internal/connectors"
	"github.com/mydisha/keirouter/backend/internal/crypto"
	"github.com/mydisha/keirouter/backend/internal/dispatch"
	"github.com/mydisha/keirouter/backend/internal/gateway"
	"github.com/mydisha/keirouter/backend/internal/identity"
	"github.com/mydisha/keirouter/backend/internal/meter"
	"github.com/mydisha/keirouter/backend/internal/observ"
	"github.com/mydisha/keirouter/backend/internal/pipeline"
	"github.com/mydisha/keirouter/backend/internal/slimmer"
	"github.com/mydisha/keirouter/backend/internal/store"
	"github.com/mydisha/keirouter/backend/internal/transform"
	"github.com/mydisha/keirouter/backend/internal/vault"
)

// App is the assembled application, ready to serve.
type App struct {
	cfg    config.Config
	log    *slog.Logger
	db     *store.DB
	server *http.Server
}

// Build constructs the application from configuration. It opens the database,
// applies migrations, initializes the crypto root, and wires the gateway.
func Build(ctx context.Context, cfg config.Config, log *slog.Logger) (*App, error) {
	dataDir, err := resolveDataDir(cfg)
	if err != nil {
		return nil, err
	}

	db, err := store.Open(ctx, cfg.Database, dataDir)
	if err != nil {
		return nil, err
	}
	if err := db.Migrate(ctx); err != nil {
		return nil, fmt.Errorf("app: migrate: %w", err)
	}
	if err := db.Tenants().EnsureDefault(ctx); err != nil {
		return nil, fmt.Errorf("app: ensure default tenant: %w", err)
	}

	masterKey, err := loadOrCreateMasterKey(cfg, dataDir, log)
	if err != nil {
		return nil, err
	}
	sealer, err := crypto.NewSealer(masterKey)
	if err != nil {
		return nil, fmt.Errorf("app: build sealer: %w", err)
	}

	// Construct services.
	v := vault.New(sealer)
	connRegistry := connectors.DefaultRegistry()
	codecs := transform.DefaultRegistry()
	idSvc := identity.New(db.APIKeys())

	authSvc := auth.New(db.Settings(), cfg.Security.JWTSecret, cfg.Security.SessionTTL)
	seeded, err := authSvc.EnsureDefaults(ctx)
	if err != nil {
		return nil, fmt.Errorf("app: init auth: %w", err)
	}
	if seeded {
		log.Warn("seeded default dashboard password",
			"password", auth.DefaultPassword,
			"note", "change it on first login via the onboarding flow")
	}

	pricing := buildPricing()
	mtr := meter.New(db.Usage(), pricing)
	bud := budget.New(db.Budgets(), db.Usage())
	disp := dispatch.New(connRegistry, db.Accounts(), v)
	slim := slimmer.Default()
	metrics := observ.New()

	// Semantic cache. Defaults to a local in-memory store keyed by a
	// deterministic hash embedder (exact-prompt caching). A provider-backed
	// embedder can be substituted for true near-match semantic caching.
	semanticCache := cache.New(cache.Config{
		Enabled:             cfg.Cache.Enabled,
		SimilarityThreshold: cfg.Cache.SimilarityThreshold,
		TTL:                 cfg.Cache.TTL,
		MaxEntries:          10000,
	}, nil)
	embedder := cache.NewHashEmbedder(32)

	pipe := pipeline.New(pipeline.Deps{
		Dispatcher: disp,
		Meter:      mtr,
		Budget:     bud,
		Slimmer:    slim,
		Metrics:    metrics,
		Cache:      semanticCache,
		Embedder:   embedder,
		Logger:     log,
	})

	gw := gateway.New(gateway.Deps{
		Config:   cfg,
		Logger:   log,
		Identity: idSvc,
		Auth:     authSvc,
		Pipeline: pipe,
		Chains:   db.Chains(),
		Accounts: db.Accounts(),
		Budgets:  db.Budgets(),
		Usage:    db.Usage(),
		Vault:    v,
		Codecs:   codecs,
		Metrics:  metrics,
	})

	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           gw.Handler(),
		ReadHeaderTimeout: 15 * time.Second,
		// No WriteTimeout: streaming responses are long-lived; the stall
		// timeout is enforced per-stream inside the connectors instead.
	}

	return &App{cfg: cfg, log: log, db: db, server: srv}, nil
}

// Run starts the HTTP server and blocks until ctx is cancelled, then shuts down
// gracefully.
func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		a.log.Info("KeiRouter listening", "addr", a.cfg.Addr(), "db", a.cfg.Database.Driver)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		a.log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := a.server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return a.db.Close()
	case err := <-errCh:
		return err
	}
}

// resolveDataDir returns the configured data directory, creating it if needed.
func resolveDataDir(cfg config.Config) (string, error) {
	dir := cfg.Data.Dir
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("app: resolve home dir: %w", err)
		}
		dir = filepath.Join(home, ".keirouter")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("app: create data dir %q: %w", dir, err)
	}
	return dir, nil
}

// loadOrCreateMasterKey resolves the envelope-encryption root key. Precedence:
// explicit config/env value, then a persisted key file, then a freshly
// generated key written to disk with 0600 permissions.
func loadOrCreateMasterKey(cfg config.Config, dataDir string, log *slog.Logger) ([]byte, error) {
	if cfg.Security.MasterKey != "" {
		key, err := crypto.DecodeMasterKey(cfg.Security.MasterKey)
		if err != nil {
			return nil, fmt.Errorf("app: invalid configured master key: %w", err)
		}
		return key, nil
	}

	keyPath := filepath.Join(dataDir, "master.key")
	if data, err := os.ReadFile(keyPath); err == nil {
		key, derr := crypto.DecodeMasterKey(string(data))
		if derr != nil {
			return nil, fmt.Errorf("app: corrupt master key file %q: %w", keyPath, derr)
		}
		return key, nil
	}

	key, err := crypto.GenerateMasterKey()
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(keyPath, []byte(crypto.EncodeMasterKey(key)), 0o600); err != nil {
		return nil, fmt.Errorf("app: persist master key: %w", err)
	}
	log.Warn("generated new master key", "path", keyPath,
		"note", "back this up; losing it makes stored credentials unrecoverable")
	return key, nil
}

// buildPricing projects the connector catalog into a meter pricing table.
func buildPricing() map[string]meter.Price {
	specs := connectors.Catalog()
	prices := make([]meter.SpecPrice, 0, len(specs))
	for _, s := range specs {
		prices = append(prices, meter.SpecPrice{ID: s.ID, InputPerM: s.InputPerM, OutputPerM: s.OutputPerM})
	}
	return meter.PricingFromCatalog(prices)
}