package gateway

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/mydisha/keirouter/backend/internal/connectors"
	"github.com/mydisha/keirouter/backend/internal/store"
	"github.com/mydisha/keirouter/backend/internal/vault"
)

// mountAdmin registers the dashboard admin endpoints on the given router. These
// manage API keys, provider accounts, routing chains, budgets, and usage.
func (s *Server) mountAdmin(r chi.Router) {
	r.Get("/providers", s.adminListProviders)

	r.Get("/keys", s.adminListKeys)
	r.Post("/keys", s.adminCreateKey)
	r.Delete("/keys/{id}", s.adminDeleteKey)

	r.Get("/accounts", s.adminListAccounts)
	r.Post("/accounts", s.adminCreateAccount)
	r.Delete("/accounts/{id}", s.adminDeleteAccount)

	r.Get("/chains", s.adminListChains)
	r.Post("/chains", s.adminCreateChain)
	r.Delete("/chains/{id}", s.adminDeleteChain)

	r.Get("/budgets", s.adminListBudgets)
	r.Post("/budgets", s.adminCreateBudget)
	r.Delete("/budgets/{id}", s.adminDeleteBudget)

	r.Get("/usage", s.adminUsageSummary)
}

const adminTenant = store.DefaultTenantID

// ---- providers --------------------------------------------------------------

func (s *Server) adminListProviders(w http.ResponseWriter, _ *http.Request) {
	specs := connectors.Catalog()
	out := make([]map[string]any, 0, len(specs))
	for _, p := range specs {
		out = append(out, map[string]any{
			"id":           p.ID,
			"display_name": p.DisplayName,
			"dialect":      p.Dialect,
			"auth_kind":    p.AuthKind,
			"input_per_m":  p.InputPerM,
			"output_per_m": p.OutputPerM,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": out})
}

// ---- API keys ---------------------------------------------------------------

func (s *Server) adminListKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := s.identity.List(r.Context(), adminTenant)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		out = append(out, map[string]any{
			"id": k.ID, "name": k.Name, "display": k.Display,
			"disabled": k.Disabled, "created_at": k.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"keys": out})
}

func (s *Server) adminCreateKey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name      string `json:"name"`
		ProjectID string `json:"project_id"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	issued, err := s.identity.Create(r.Context(), adminTenant, body.ProjectID, body.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Plaintext is returned exactly once.
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": issued.Record.ID, "name": issued.Record.Name,
		"key": issued.Plaintext, "display": issued.Record.Display,
	})
}

func (s *Server) adminDeleteKey(w http.ResponseWriter, r *http.Request) {
	if err := s.identity.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- accounts ---------------------------------------------------------------

func (s *Server) adminListAccounts(w http.ResponseWriter, r *http.Request) {
	accs, err := s.accounts.ListByTenant(r.Context(), adminTenant)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]map[string]any, 0, len(accs))
	for _, a := range accs {
		// Never expose secret material.
		out = append(out, map[string]any{
			"id": a.ID, "provider": a.Provider, "label": a.Label,
			"auth_kind": a.AuthKind, "priority": a.Priority,
			"disabled": a.Disabled, "created_at": a.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"accounts": out})
}

func (s *Server) adminCreateAccount(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Provider string `json:"provider"`
		Label    string `json:"label"`
		APIKey   string `json:"api_key"`
		BaseURL  string `json:"base_url"`
		Priority int    `json:"priority"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Provider == "" {
		writeError(w, http.StatusBadRequest, "provider is required")
		return
	}
	if _, ok := connectors.SpecByID(body.Provider); !ok {
		writeError(w, http.StatusBadRequest, "unknown provider: "+body.Provider)
		return
	}
	if s.vault == nil {
		writeError(w, http.StatusInternalServerError, "vault not configured")
		return
	}

	now := time.Now()
	acc := store.Account{
		ID:        uuid.NewString(),
		TenantID:  adminTenant,
		Provider:  body.Provider,
		Label:     body.Label,
		AuthKind:  store.AuthAPIKey,
		Priority:  defaultInt(body.Priority, 100),
		CreatedAt: now,
		UpdatedAt: now,
	}
	meta := map[string]string{}
	if body.BaseURL != "" {
		meta["base_url"] = body.BaseURL
	}
	if err := s.vault.Seal(&acc, vault.NewSecret{APIKey: body.APIKey, Metadata: meta}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.accounts.Create(r.Context(), acc); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": acc.ID, "provider": acc.Provider, "label": acc.Label})
}

func (s *Server) adminDeleteAccount(w http.ResponseWriter, r *http.Request) {
	if err := s.accounts.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- chains -----------------------------------------------------------------

func (s *Server) adminListChains(w http.ResponseWriter, r *http.Request) {
	chains, err := s.chains.ListByTenant(r.Context(), adminTenant)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]map[string]any, 0, len(chains))
	for _, c := range chains {
		steps := make([]map[string]any, 0, len(c.Steps))
		for _, st := range c.Steps {
			steps = append(steps, map[string]any{"provider": st.Provider, "model": st.Model, "position": st.Position})
		}
		out = append(out, map[string]any{
			"id": c.ID, "name": c.Name, "strategy": c.Strategy, "steps": steps,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"chains": out})
}

func (s *Server) adminCreateChain(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string `json:"name"`
		Strategy string `json:"strategy"`
		Steps    []struct {
			Provider string `json:"provider"`
			Model    string `json:"model"`
		} `json:"steps"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Name == "" || len(body.Steps) == 0 {
		writeError(w, http.StatusBadRequest, "name and at least one step are required")
		return
	}

	now := time.Now()
	chain := store.Chain{
		ID:        uuid.NewString(),
		TenantID:  adminTenant,
		Name:      body.Name,
		Strategy:  defaultStr(body.Strategy, "priority"),
		CreatedAt: now,
		UpdatedAt: now,
	}
	for i, st := range body.Steps {
		if _, ok := connectors.SpecByID(st.Provider); !ok {
			writeError(w, http.StatusBadRequest, "unknown provider in step: "+st.Provider)
			return
		}
		chain.Steps = append(chain.Steps, store.ChainStep{
			ID: uuid.NewString(), ChainID: chain.ID, Position: i,
			Provider: st.Provider, Model: st.Model, CreatedAt: now,
		})
	}
	if err := s.chains.Create(r.Context(), chain); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": chain.ID, "name": chain.Name})
}

func (s *Server) adminDeleteChain(w http.ResponseWriter, r *http.Request) {
	if err := s.chains.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- budgets ----------------------------------------------------------------

func (s *Server) adminListBudgets(w http.ResponseWriter, r *http.Request) {
	budgets, err := s.budgets.ListByTenant(r.Context(), adminTenant)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]map[string]any, 0, len(budgets))
	for _, b := range budgets {
		out = append(out, map[string]any{
			"id": b.ID, "scope_kind": b.ScopeKind, "scope_id": b.ScopeID,
			"limit_micros": b.LimitMicros, "period": b.Period,
			"alert_pct": b.AlertPct, "hard_cutoff": b.HardCutoff,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"budgets": out})
}

func (s *Server) adminCreateBudget(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ScopeKind   string `json:"scope_kind"`
		ScopeID     string `json:"scope_id"`
		LimitUSD    float64 `json:"limit_usd"`
		Period      string `json:"period"`
		AlertPct    int    `json:"alert_pct"`
		HardCutoff  *bool  `json:"hard_cutoff"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.LimitUSD <= 0 {
		writeError(w, http.StatusBadRequest, "limit_usd must be positive")
		return
	}
	hardCutoff := true
	if body.HardCutoff != nil {
		hardCutoff = *body.HardCutoff
	}

	now := time.Now()
	b := store.Budget{
		ID:          uuid.NewString(),
		TenantID:    adminTenant,
		ScopeKind:   store.BudgetScope(defaultStr(body.ScopeKind, string(store.ScopeTenant))),
		ScopeID:     defaultStr(body.ScopeID, adminTenant),
		LimitMicros: int64(body.LimitUSD * 1_000_000),
		Period:      defaultStr(body.Period, "monthly"),
		AlertPct:    defaultInt(body.AlertPct, 80),
		HardCutoff:  hardCutoff,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.budgets.Create(r.Context(), b); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": b.ID})
}

func (s *Server) adminDeleteBudget(w http.ResponseWriter, r *http.Request) {
	if err := s.budgets.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- usage ------------------------------------------------------------------

func (s *Server) adminUsageSummary(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	since := time.Now().AddDate(0, 0, -30)
	switch period {
	case "today":
		now := time.Now()
		since = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "week":
		since = time.Now().AddDate(0, 0, -7)
	case "month", "":
		since = time.Now().AddDate(0, -1, 0)
	}

	sum, err := s.usage.Summarize(r.Context(), adminTenant, since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"total_requests":    sum.TotalRequests,
		"prompt_tokens":     sum.PromptTokens,
		"completion_tokens": sum.CompletionTokens,
		"cached_tokens":     sum.CachedTokens,
		"cost_usd":          float64(sum.CostMicros) / 1_000_000,
		"cache_hits":        sum.CacheHits,
		"since":             since,
	})
}

// ---- helpers ----------------------------------------------------------------

// decodeJSON decodes a request body into v, writing a 400 on failure. It
// returns false when the caller should stop.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		if errors.Is(err, http.ErrBodyReadAfterClose) {
			writeError(w, http.StatusBadRequest, "empty body")
			return false
		}
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return false
	}
	return true
}

func defaultStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func defaultInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}