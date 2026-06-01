// Package gateway is the HTTP edge of KeiRouter. It authenticates inbound
// requests, parses them with the client's dialect codec, resolves a routing
// chain, runs the pipeline, and renders the response (unary or streaming) back
// in the client's dialect.
package gateway

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/mydisha/keirouter/backend/internal/auth"
	"github.com/mydisha/keirouter/backend/internal/clitools"
	"github.com/mydisha/keirouter/backend/internal/config"
	"github.com/mydisha/keirouter/backend/internal/identity"
	"github.com/mydisha/keirouter/backend/internal/oauth"
	"github.com/mydisha/keirouter/backend/internal/observ"
	"github.com/mydisha/keirouter/backend/internal/pipeline"
	"github.com/mydisha/keirouter/backend/internal/store"
	"github.com/mydisha/keirouter/backend/internal/transform"
	"github.com/mydisha/keirouter/backend/internal/vault"
)

// Server holds the gateway's dependencies and HTTP routes.
type Server struct {
	cfg      config.Config
	log      *slog.Logger
	identity *identity.Service
	auth     *auth.Service
	pipeline *pipeline.Pipeline
	chains   *store.ChainRepo
	aliases  *store.AliasRepo
	accounts *store.AccountRepo
	budgets  *store.BudgetRepo
	usage    *store.UsageRepo
	settings *store.SettingsRepo
	vault    *vault.Vault
	codecs   *transform.Registry
	metrics  *observ.Metrics
	cliTools *clitools.Registry
	cliToolHome string
	oauthSessions *oauth.SessionStore
	router   chi.Router
}

// Deps bundles the gateway's collaborators.
type Deps struct {
	Config   config.Config
	Logger   *slog.Logger
	Identity *identity.Service
	Auth     *auth.Service
	Pipeline *pipeline.Pipeline
	Chains   *store.ChainRepo
	Aliases  *store.AliasRepo
	Accounts *store.AccountRepo
	Budgets  *store.BudgetRepo
	Usage    *store.UsageRepo
	Settings *store.SettingsRepo
	Vault    *vault.Vault
	Codecs   *transform.Registry
	Metrics  *observ.Metrics
	CLITools *clitools.Registry
	CLITHome string
}

// New builds a gateway Server and wires its routes.
func New(d Deps) *Server {
	log := d.Logger
	if log == nil {
		log = slog.Default()
	}
	cliTools := d.CLITools
	if cliTools == nil {
		cliTools = clitools.NewRegistry()
	}
	cliToolHome := d.CLITHome
	if cliToolHome == "" {
		cliToolHome, _ = os.UserHomeDir()
	}
	s := &Server{
		cfg:      d.Config,
		log:      log,
		identity: d.Identity,
		auth:     d.Auth,
		pipeline: d.Pipeline,
		chains:   d.Chains,
		aliases:  d.Aliases,
		accounts: d.Accounts,
		budgets:  d.Budgets,
		usage:    d.Usage,
		settings: d.Settings,
		vault:    d.Vault,
		codecs:   d.Codecs,
		metrics:  d.Metrics,
		cliTools: cliTools,
		cliToolHome: cliToolHome,
		oauthSessions: oauth.NewSessionStore(),
	}
	s.router = s.routes()
	return s
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler { return s.router }

func (s *Server) routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: s.cfg.Server.CORSOrigins,
		AllowedMethods: []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Authorization", "Content-Type", "x-api-key"},
	}))

	// Health check (unauthenticated).
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Prometheus metrics endpoint. Loopback-guarded by default to avoid leaking
	// operational telemetry; expose deliberately behind a scraper's network.
	if s.metrics != nil {
		r.Group(func(r chi.Router) {
			r.Use(s.loopbackOnly)
			r.Handle("/metrics", promhttp.HandlerFor(s.metrics.Registry(), promhttp.HandlerOpts{}))
		})
	}

	// OpenAI-compatible API surface (authenticated).
	r.Group(func(r chi.Router) {
		r.Use(s.authMiddleware)
		r.Post("/v1/chat/completions", s.handleOpenAIChat)
		r.Post("/v1/messages", s.handleAnthropicMessages)

		// OpenAI Responses API surface (Codex and Responses-native clients).
		r.Post("/v1/responses", s.handleOpenAIResponses)

		// Gemini-native generateContent endpoint. The model + action are in the
		// URL path ({model}:generateContent), matching Google's SDK clients.
		r.Post("/v1beta/models/{modelAction}", s.handleGeminiGenerate)

		// Multi-capability endpoints (Phase 2).
		r.Post("/v1/embeddings", s.handleEmbeddings)
		r.Post("/v1/images/generations", s.handleImageGeneration)
		r.Post("/v1/audio/speech", s.handleAudioSpeech)
		r.Post("/v1/audio/transcriptions", s.handleAudioTranscription)
		r.Post("/v1/search", s.handleWebSearch)
		r.Post("/v1/web/fetch", s.handleWebFetch)

		// Model discovery.
		r.Get("/v1/models", s.handleListModels)
		r.Get("/v1/models/info", s.handleModelInfo)
		r.Get("/v1/models/{kind}", s.handleListModelsByKind)
	})

	// Dashboard auth endpoints (login/logout/status) are loopback-guarded but
	// do not require a session — they are how a session is obtained.
	r.Route("/api/auth", func(r chi.Router) {
		r.Use(s.loopbackOnly)
		s.mountAuth(r)
		r.Group(func(pr chi.Router) {
			pr.Use(s.sessionMiddleware)
			s.mountAuthenticatedAuth(pr)
		})
	})

	// Dashboard admin API. Guarded by loopback access AND a valid dashboard
	// session, so provider credentials and routing config are never exposed to
	// an unauthenticated caller, even on localhost.
	r.Route("/api", func(r chi.Router) {
		r.Use(s.loopbackOnly)
		r.Use(s.sessionMiddleware)
		s.mountAdmin(r)
	})

	return r
}

// ---- HTTP helpers -----------------------------------------------------------

// writeJSON writes a JSON response with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes an OpenAI-style error envelope.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    errorType(status),
		},
	})
}

func errorType(status int) string {
	switch {
	case status == http.StatusUnauthorized:
		return "authentication_error"
	case status == http.StatusTooManyRequests:
		return "rate_limit_error"
	case status >= 400 && status < 500:
		return "invalid_request_error"
	default:
		return "api_error"
	}
}

// tenantOf returns the tenant id for an authenticated key, defaulting to the
// implicit local tenant when unset.
func tenantOf(key store.APIKey) string {
	if key.TenantID != "" {
		return key.TenantID
	}
	return store.DefaultTenantID
}