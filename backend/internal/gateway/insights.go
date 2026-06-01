package gateway

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/mydisha/keirouter/backend/internal/connectors"
	"github.com/mydisha/keirouter/backend/internal/store"
)

// sinceForPeriod maps a dashboard period query value to a lower-bound time.
// Mirrors adminUsageSummary's windows: today / week / month (default).
func sinceForPeriod(period string) time.Time {
	now := time.Now()
	switch period {
	case "today":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "week":
		return now.AddDate(0, 0, -7)
	case "month", "":
		return now.AddDate(0, -1, 0)
	default:
		return now.AddDate(0, 0, -30)
	}
}

// ---- usage insights ---------------------------------------------------------

// adminUsageInsights returns the rich payload that powers the Usage page: the
// per-provider routing breakdown, a bucketed activity-over-time series, recent
// activity rows, and headline metrics (success rate, average latency).
func (s *Server) adminUsageInsights(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	since := sinceForPeriod(period)
	ctx := r.Context()

	sum, err := s.usage.Summarize(ctx, adminTenant, since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	breakdown, err := s.usage.Breakdown(ctx, adminTenant, since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	recent, err := s.usage.Recent(ctx, adminTenant, 8)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	timeline, err := s.usage.Timeline(ctx, adminTenant, since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Per-provider breakdown with shares of total request volume. Decorate each
	// with the provider's display name/color from the connector catalog.
	providers := make([]map[string]any, 0, len(breakdown))
	for _, p := range breakdown {
		share := 0.0
		if sum.TotalRequests > 0 {
			share = float64(p.TotalRequests) / float64(sum.TotalRequests) * 100
		}
		display, color, icon := p.Provider, "", ""
		if spec, ok := connectors.SpecByID(p.Provider); ok {
			display = spec.DisplayName
			color = spec.Color
			icon = "/providers/" + spec.ID + ".png"
		}
		providers = append(providers, map[string]any{
			"provider":          p.Provider,
			"display_name":      display,
			"color":             color,
			"icon":              icon,
			"total_requests":    p.TotalRequests,
			"prompt_tokens":     p.PromptTokens,
			"completion_tokens": p.CompletionTokens,
			"cost_usd":          float64(p.CostMicros) / 1_000_000,
			"share_pct":         share,
		})
	}

	// Recent activity rows.
	recentRows := make([]map[string]any, 0, len(recent))
	for _, rec := range recent {
		recentRows = append(recentRows, map[string]any{
			"id":         rec.ID,
			"provider":   rec.Provider,
			"model":      rec.Model,
			"tokens":     rec.PromptTokens + rec.CompletionTokens,
			"cost_usd":   float64(rec.CostMicros) / 1_000_000,
			"cache_hit":  rec.CacheHit,
			"latency_ms": rec.LatencyMS,
			"created_at": rec.CreatedAt,
		})
	}

	// Bucket the timeline into 24 even slots across the window for the sparkline.
	buckets := bucketTimeline(timeline, since, time.Now(), 24)
	busiestIdx, busiestCount := 0, int64(0)
	for i, b := range buckets {
		if b.count > busiestCount {
			busiestCount, busiestIdx = b.count, i
		}
	}
	series := make([]map[string]any, 0, len(buckets))
	for _, b := range buckets {
		series = append(series, map[string]any{"label": b.label, "count": b.count})
	}

	// Success rate + average latency, derived from the recent window. The meter
	// records 0 latency when a request never reached an upstream, so those rows
	// are treated as failures for the headline success-rate metric.
	var withLatency, latencySum int64
	for _, rec := range recent {
		if rec.LatencyMS > 0 {
			withLatency++
			latencySum += int64(rec.LatencyMS)
		}
	}
	successRate := 100.0
	avgLatency := 0
	if len(recent) > 0 {
		successRate = float64(withLatency) / float64(len(recent)) * 100
		if withLatency > 0 {
			avgLatency = int(latencySum / withLatency)
		}
	}

	busiest := ""
	if busiestCount > 0 && busiestIdx < len(buckets) {
		busiest = buckets[busiestIdx].label
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"summary": map[string]any{
			"total_requests":    sum.TotalRequests,
			"prompt_tokens":     sum.PromptTokens,
			"completion_tokens": sum.CompletionTokens,
			"cached_tokens":     sum.CachedTokens,
			"cost_usd":          float64(sum.CostMicros) / 1_000_000,
			"cache_hits":        sum.CacheHits,
			"success_rate":      successRate,
			"avg_latency_ms":    avgLatency,
			"since":             since,
		},
		"providers": providers,
		"recent":    recentRows,
		"series":    series,
		"busiest":   busiest,
	})
}

type timeBucket struct {
	label string
	count int64
}

// bucketTimeline distributes time points into n even buckets between from and
// to, labelling each bucket with its start time (HH:MM).
func bucketTimeline(points []store.TimePoint, from, to time.Time, n int) []timeBucket {
	if n <= 0 {
		n = 24
	}
	buckets := make([]timeBucket, n)
	span := to.Sub(from)
	if span <= 0 {
		span = time.Hour
	}
	slot := span / time.Duration(n)
	if slot <= 0 {
		slot = time.Minute
	}
	for i := 0; i < n; i++ {
		buckets[i].label = from.Add(time.Duration(i) * slot).Format("15:04")
	}
	for _, p := range points {
		idx := int(p.CreatedAt.Sub(from) / slot)
		if idx < 0 {
			idx = 0
		}
		if idx >= n {
			idx = n - 1
		}
		buckets[idx].count++
	}
	return buckets
}

// ---- quota tracker ----------------------------------------------------------

// adminQuotaUsage returns per-account usage so the Quota Tracker can show how
// much each connected account has consumed in the period.
func (s *Server) adminQuotaUsage(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	since := sinceForPeriod(period)
	ctx := r.Context()

	accs, err := s.accounts.ListByTenant(ctx, adminTenant)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	byAcct, err := s.usage.ByAccount(ctx, adminTenant, since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	usageByID := make(map[string]store.AccountUsage, len(byAcct))
	for _, u := range byAcct {
		usageByID[u.AccountID] = u
	}

	out := make([]map[string]any, 0, len(accs))
	for _, a := range accs {
		u := usageByID[a.ID]
		status := "active"
		if a.Disabled {
			status = "paused"
		} else if a.CooldownUntil != nil && a.CooldownUntil.After(time.Now()) {
			status = "needs_attention"
		}
		display := a.Provider
		if spec, ok := connectors.SpecByID(a.Provider); ok {
			display = spec.DisplayName
		}
		out = append(out, map[string]any{
			"id":                a.ID,
			"provider":          a.Provider,
			"provider_name":     display,
			"label":             a.Label,
			"auth_kind":         a.AuthKind,
			"priority":          a.Priority,
			"status":            status,
			"total_requests":    u.TotalRequests,
			"prompt_tokens":     u.PromptTokens,
			"completion_tokens": u.CompletionTokens,
			"cached_tokens":     u.CachedTokens,
			"cost_usd":          float64(u.CostMicros) / 1_000_000,
			"updated_at":        a.UpdatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"accounts": out, "since": since})
}

// ---- console log ------------------------------------------------------------

// adminConsoleLog returns the most recent metered requests as a chronological
// console feed. It is the data source for the Console Log page.
func (s *Server) adminConsoleLog(w http.ResponseWriter, r *http.Request) {
	recent, err := s.usage.Recent(r.Context(), adminTenant, 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	rows := make([]map[string]any, 0, len(recent))
	for _, rec := range recent {
		level := "info"
		if rec.LatencyMS == 0 {
			level = "error"
		} else if rec.LatencyMS > 8000 {
			level = "warn"
		}
		rows = append(rows, map[string]any{
			"id":         rec.ID,
			"level":      level,
			"provider":   rec.Provider,
			"model":      rec.Model,
			"tokens":     rec.PromptTokens + rec.CompletionTokens,
			"cost_usd":   float64(rec.CostMicros) / 1_000_000,
			"cache_hit":  rec.CacheHit,
			"latency_ms": rec.LatencyMS,
			"created_at": rec.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": rows})
}

// ---- proxy pools ------------------------------------------------------------

// proxyPoolsKey is the settings key under which proxy pool definitions are
// stored as a JSON array. Proxy pools route upstream traffic through egress
// proxies, mirroring 9router's Proxy Pools feature.
const proxyPoolsKey = "proxy_pools"

// proxyPool is one named pool of egress proxy URLs.
type proxyPool struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Proxies   []string `json:"proxies"`
	Enabled   bool     `json:"enabled"`
	CreatedAt string   `json:"created_at"`
}

func (s *Server) loadProxyPools(r *http.Request) []proxyPool {
	pools := []proxyPool{}
	if s.settings == nil {
		return pools
	}
	raw, err := s.settings.Get(r.Context(), proxyPoolsKey)
	if err != nil || raw == "" {
		return pools
	}
	_ = json.Unmarshal([]byte(raw), &pools)
	return pools
}

func (s *Server) saveProxyPools(r *http.Request, pools []proxyPool) error {
	raw, err := json.Marshal(pools)
	if err != nil {
		return err
	}
	return s.settings.Set(r.Context(), proxyPoolsKey, string(raw))
}

func (s *Server) adminListProxyPools(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"pools": s.loadProxyPools(r)})
}

func (s *Server) adminCreateProxyPool(w http.ResponseWriter, r *http.Request) {
	if s.settings == nil {
		writeError(w, http.StatusInternalServerError, "settings store not configured")
		return
	}
	var body struct {
		Name    string   `json:"name"`
		Proxies []string `json:"proxies"`
		Enabled *bool    `json:"enabled"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	cleaned := make([]string, 0, len(body.Proxies))
	for _, p := range body.Proxies {
		if p != "" {
			cleaned = append(cleaned, p)
		}
	}
	pool := proxyPool{
		ID:        uuid.NewString(),
		Name:      body.Name,
		Proxies:   cleaned,
		Enabled:   enabled,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	pools := append(s.loadProxyPools(r), pool)
	if err := s.saveProxyPools(r, pools); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, pool)
}

func (s *Server) adminDeleteProxyPool(w http.ResponseWriter, r *http.Request) {
	if s.settings == nil {
		writeError(w, http.StatusInternalServerError, "settings store not configured")
		return
	}
	id := chi.URLParam(r, "id")
	pools := s.loadProxyPools(r)
	out := pools[:0]
	for _, p := range pools {
		if p.ID != id {
			out = append(out, p)
		}
	}
	if err := s.saveProxyPools(r, out); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- skills -----------------------------------------------------------------

// skillsKey is the settings key under which skill toggles are stored. Skills
// are reusable system-prompt augmentations the gateway can apply, mirroring
// 9router's Skills feature.
const skillsKey = "skills"

type skill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"created_at"`
}

func (s *Server) loadSkills(r *http.Request) []skill {
	skills := []skill{}
	if s.settings == nil {
		return skills
	}
	raw, err := s.settings.Get(r.Context(), skillsKey)
	if err != nil || raw == "" {
		return skills
	}
	_ = json.Unmarshal([]byte(raw), &skills)
	return skills
}

func (s *Server) saveSkills(r *http.Request, skills []skill) error {
	raw, err := json.Marshal(skills)
	if err != nil {
		return err
	}
	return s.settings.Set(r.Context(), skillsKey, string(raw))
}

func (s *Server) adminListSkills(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"skills": s.loadSkills(r)})
}

func (s *Server) adminCreateSkill(w http.ResponseWriter, r *http.Request) {
	if s.settings == nil {
		writeError(w, http.StatusInternalServerError, "settings store not configured")
		return
	}
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Prompt      string `json:"prompt"`
		Enabled     *bool  `json:"enabled"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	sk := skill{
		ID:          uuid.NewString(),
		Name:        body.Name,
		Description: body.Description,
		Prompt:      body.Prompt,
		Enabled:     enabled,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	skills := append(s.loadSkills(r), sk)
	if err := s.saveSkills(r, skills); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, sk)
}

func (s *Server) adminUpdateSkill(w http.ResponseWriter, r *http.Request) {
	if s.settings == nil {
		writeError(w, http.StatusInternalServerError, "settings store not configured")
		return
	}
	id := chi.URLParam(r, "id")
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	skills := s.loadSkills(r)
	found := false
	for i := range skills {
		if skills[i].ID == id {
			if body.Enabled != nil {
				skills[i].Enabled = *body.Enabled
			}
			found = true
			break
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "skill not found")
		return
	}
	if err := s.saveSkills(r, skills); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) adminDeleteSkill(w http.ResponseWriter, r *http.Request) {
	if s.settings == nil {
		writeError(w, http.StatusInternalServerError, "settings store not configured")
		return
	}
	id := chi.URLParam(r, "id")
	skills := s.loadSkills(r)
	out := skills[:0]
	for _, sk := range skills {
		if sk.ID != id {
			out = append(out, sk)
		}
	}
	if err := s.saveSkills(r, out); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
