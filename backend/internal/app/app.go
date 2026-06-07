// Package app wires KeiRouter's components into a runnable application: it
// opens the store, runs migrations, loads or generates the master key, and
// constructs the gateway with its full dependency graph.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
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
	"github.com/mydisha/keirouter/backend/internal/oauth"
	"github.com/mydisha/keirouter/backend/internal/observ"
	"github.com/mydisha/keirouter/backend/internal/pipeline"
	"github.com/mydisha/keirouter/backend/internal/slimmer"
	"github.com/mydisha/keirouter/backend/internal/store"
	"github.com/mydisha/keirouter/backend/internal/transform"
	"github.com/mydisha/keirouter/backend/internal/tunnel/cloudflare"
	"github.com/mydisha/keirouter/backend/internal/tunnel/tailscale"
	"github.com/mydisha/keirouter/backend/internal/update"
	"github.com/mydisha/keirouter/backend/internal/usagehub"
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
func Build(ctx context.Context, cfg config.Config, log *slog.Logger, version string) (*App, error) {
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
	modelPrices := buildModelPrices()
	mtr := meter.New(db.Usage(), pricing, modelPrices)
	uh := usagehub.New()
	mtr.SetHub(uh)
	bud := budget.New(db.Budgets(), db.Usage())
	disp := dispatch.New(connRegistry, db.Accounts(), v)
	// Refresh expiring OAuth access tokens just-in-time before each upstream
	// call, persisting the rotated tokens.
	tokenRefresher := oauth.NewTokenManager(v, db.Accounts())
	disp.SetTokenRefresher(tokenRefresher)
	// Resolve proxy pool bindings for accounts that have one.
	disp.SetPoolSource(db.ProxyPools())
	// Model-level cooldowns and round-robin chain rotation.
	disp.SetRoutingSource(db.Routing())
	slim := slimmer.Default()
	metrics := observ.New()

	// Semantic cache. Defaults to a local in-memory store keyed by a
	// deterministic hash embedder (exact-prompt caching). When configured with
	// backend=redis + embedding_provider=api, uses Redis for persistence and
	// an embedding API for true semantic near-match caching.
	var cacheStore cache.Store
	if cfg.Cache.Backend == "redis" && cfg.Cache.RedisURL != "" {
		rs, err := cache.NewRedisStore(cache.RedisStoreConfig{
			URL:    cfg.Cache.RedisURL,
			TTL:    cfg.Cache.TTL,
			Logger: log,
		})
		if err != nil {
			return nil, fmt.Errorf("app: redis cache store: %w", err)
		}
		cacheStore = rs
		log.Info("semantic cache: redis backend", "url", cfg.Cache.RedisURL)
	}

	var embedder cache.Embedder
	if cfg.Cache.EmbeddingProvider == "api" && cfg.Cache.EmbeddingAPIKey != "" {
		embedder = cache.NewAPIEmbedder(cache.APIEmbedderConfig{
			BaseURL: cfg.Cache.EmbeddingAPIURL,
			APIKey:  cfg.Cache.EmbeddingAPIKey,
			Model:   cfg.Cache.EmbeddingModel,
			Dims:    cfg.Cache.EmbeddingDims,
		})
		log.Info("semantic cache: API embedder", "model", cfg.Cache.EmbeddingModel)
	} else {
		embedder = cache.NewHashEmbedder(32)
		if cfg.Cache.Enabled {
			log.Info("semantic cache: hash embedder (exact-match only)")
		}
	}

	semanticCache := cache.New(cache.Config{
		Enabled:             cfg.Cache.Enabled,
		SimilarityThreshold: cfg.Cache.SimilarityThreshold,
		TTL:                 cfg.Cache.TTL,
		MaxEntries:          10000,
	}, cacheStore)

	// Timeout notifier: atomic cache of timeout values that can be updated
	// at runtime from the dashboard settings without restarting.
	timeoutNotifier := gateway.NewTimeoutNotifier(
		cfg.Server.StreamStallTimeout,
		30*time.Second, // ResponseHeaderTimeout fallback (from transport)
		cfg.Server.RequestTimeout,
	)

	pipe := pipeline.New(pipeline.Deps{
		Dispatcher:         disp,
		Meter:              mtr,
		Budget:             bud,
		Slimmer:            slim,
		Metrics:            metrics,
		Cache:              semanticCache,
		Embedder:           embedder,
		Logger:             log,
		RequestTimeout:     cfg.Server.RequestTimeout,
		StreamStallTimeout: cfg.Server.StreamStallTimeout,
		TimeoutReader:      timeoutNotifier,
	})

	// Resolve frontend dist directory. Check common install locations, then cwd.
	frontendDir := resolveFrontendDir()
	if frontendDir == "" {
		log.Warn("dashboard assets not found",
			"note", "API routes will still work, but the web dashboard will not be served")
	}

	// Tunnel managers for Cloudflare quick tunnel and Tailscale funnel.
	cfManager := cloudflare.NewManager(dataDir, cfg.Server.Port, log)
	tsManager := tailscale.NewManager(dataDir, cfg.Server.Port, log)

	gw := gateway.New(gateway.Deps{
		Config:      cfg,
		Logger:      log,
		Version:     version,
		Updates:     update.NewChecker(version, ""),
		DB:          db,
		Identity:    idSvc,
		Auth:        authSvc,
		Pipeline:    pipe,
		Conns:       connRegistry,
		Chains:      db.Chains(),
		Aliases:     db.Aliases(),
		Accounts:    db.Accounts(),
		Pools:       db.ProxyPools(),
		Budgets:     db.Budgets(),
		Usage:       db.Usage(),
		Settings:    db.Settings(),
		Vault:       v,
		Codecs:      codecs,
		Metrics:     metrics,
		FrontendDir: frontendDir,
		DataDir:     dataDir,
		CfManager:   cfManager,
		TsManager:   tsManager,
		UsageHub:       uh,
		TimeoutNotifier: timeoutNotifier,
		Refresher:       tokenRefresher,
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

	listeners, err := a.loopbackListeners()
	if err != nil {
		return err
	}

	for _, ln := range listeners {
		ln := ln
		go func() {
			a.log.Info("KeiRouter listening", "addr", ln.Addr().String(), "db", a.cfg.Database.Driver)
			if serveErr := a.server.Serve(ln); serveErr != nil && serveErr != http.ErrServerClosed {
				errCh <- serveErr
			}
		}()
	}

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

// loopbackListeners opens the TCP listeners the server serves on. When the
// configured host is a loopback name ("localhost"), it binds both 127.0.0.1
// and ::1 so callbacks resolve regardless of whether the OS prefers IPv4 or
// IPv6. This matters on Windows, where "localhost" usually resolves to ::1
// first — a single 127.0.0.1 listener would leave OAuth callbacks to
// http://localhost hanging. For any explicit address, a single listener on
// that address is used.
func (a *App) loopbackListeners() ([]net.Listener, error) {
	host := a.cfg.Server.Host
	port := a.cfg.Server.Port

	if host == "localhost" {
		var lns []net.Listener
		for _, ip := range []string{"127.0.0.1", "::1"} {
			addr := net.JoinHostPort(ip, fmt.Sprintf("%d", port))
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				// ::1 may be unavailable on hosts with IPv6 disabled. Tolerate
				// that as long as at least one listener came up.
				if ip == "::1" && len(lns) > 0 {
					a.log.Warn("IPv6 loopback unavailable; serving IPv4 only", "err", err)
					continue
				}
				for _, opened := range lns {
					_ = opened.Close()
				}
				return nil, fmt.Errorf("app: listen %s: %w", addr, err)
			}
			lns = append(lns, ln)
		}
		return lns, nil
	}

	ln, err := net.Listen("tcp", a.cfg.Addr())
	if err != nil {
		return nil, fmt.Errorf("app: listen %s: %w", a.cfg.Addr(), err)
	}
	return []net.Listener{ln}, nil
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

// resolveFrontendDir locates the frontend dist directory. The installer and
// Docker image put assets under /usr/local/share/keirouter, while development
// builds keep them in frontend/dist.
func resolveFrontendDir() string {
	candidates := []string{
		os.Getenv("KEIROUTER_FRONTEND_DIR"),
		"/usr/local/share/keirouter/frontend/dist",
		"/usr/share/keirouter/frontend/dist",
		"/opt/keirouter/frontend/dist",
		"/usr/local/frontend/dist",
		"frontend/dist",
		"../frontend/dist",
		"../../frontend/dist",
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".keirouter", "frontend", "dist"))
	}
	// Also try relative to the executable.
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, "frontend", "dist"),
			filepath.Join(dir, "..", "share", "keirouter", "frontend", "dist"),
			filepath.Join(dir, "..", "frontend", "dist"),
		)
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	return ""
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

// buildModelPrices builds the per-model pricing table from the connector model prices.
func buildModelPrices() map[string]meter.Price {
	out := make(map[string]meter.Price)
	for _, mp := range connectors.ModelPricingTable() {
		out[mp.Provider+"/"+mp.Model] = meter.Price{
			InputPerM:       mp.InputPerM,
			OutputPerM:      mp.OutputPerM,
			CachedInputPerM: mp.CachedInputPerM,
			CacheWritePerM:  mp.CacheWritePerM,
			ReasoningPerM:   mp.ReasoningPerM,
		}
	}
	return out
}
