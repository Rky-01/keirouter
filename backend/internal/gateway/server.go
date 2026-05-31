// Package gateway is the HTTP edge of KeiRouter. It authenticates inbound
// requests, parses them with the client's dialect codec, resolves a routing
// chain, runs the pipeline, and renders the response (unary or streaming) back
// in the client's dialect.
package gateway

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/mydisha/keirouter/backend/internal/config"
	"github.com/mydisha/keirouter/backend/internal/identity"
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
	pipeline *pipeline.Pipeline
	chains   *store.ChainRepo
	accounts *store.AccountRepo
	budgets  *store.BudgetRepo
	usage    *store.UsageRepo
	vault    *vault.Vault
	codecs   *transform.Registry
	router   chi.Router
}

// Deps bundles the gateway's collaborators.
type Deps struct {
	Config   config.Config
	Logger   *slog.Logger
	Identity *identity.Service
	Pipeline *pipeline.Pipeline
	Chains   *store.ChainRepo
	Accounts *store.AccountRepo
	Budgets  *store.BudgetRepo
	Usage    *store.UsageRepo
	Vault    *vault.Vault
	Codecs   *transform.Registry
}

// New builds a gateway Server and wires its routes.
func New(d Deps) *Server {
	log := d.Logger
	if log == nil {
		log = slog.Default()
	}
	s := &Server{
		cfg:      d.Config,
		log:      log,
		identity: d.Identity,
		pipeline: d.Pipeline,
		chains:   d.Chains,
		accounts: d.Accounts,
		budgets:  d.Budgets,
		usage:    d.Usage,
		vault:    d.Vault,
		codecs:   d.Codecs,
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

	// OpenAI-compatible API surface (authenticated).
	r.Group(func(r chi.Router) {
		r.Use(s.authMiddleware)
		r.Post("/v1/chat/completions", s.handleOpenAIChat)
		r.Post("/v1/messages", s.handleAnthropicMessages)
		r.Get("/v1/models", s.handleListModels)
	})

	// Dashboard admin API. In local single-user mode this is guarded by the
	// loopback-only check rather than per-user auth, matching the local proxy
	// model. SECURITY: when exposing KeiRouter beyond localhost, place this
	// behind dashboard session auth or a reverse proxy with access control.
	r.Route("/api", func(r chi.Router) {
		r.Use(s.loopbackOnly)
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

// modelInfo is a single entry in the /v1/models listing.
type modelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

// handleListModels reports the chains configured for the tenant as virtual
// models the client can target, plus a hint about the provider/model form.
func (s *Server) handleListModels(w http.ResponseWriter, r *http.Request) {
	key, _ := authedKey(r.Context())
	tenantID := tenantOf(key)

	chains, err := s.chains.ListByTenant(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list models")
		return
	}

	data := make([]modelInfo, 0, len(chains))
	for _, c := range chains {
		data = append(data, modelInfo{ID: c.Name, Object: "model", OwnedBy: "keirouter"})
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
}

// tenantOf returns the tenant id for an authenticated key, defaulting to the
// implicit local tenant when unset.
func tenantOf(key store.APIKey) string {
	if key.TenantID != "" {
		return key.TenantID
	}
	return store.DefaultTenantID
}