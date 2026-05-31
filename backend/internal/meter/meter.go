// Package meter records per-request usage and computes cost.
//
// Cost is stored in micros (millionths of a USD) as an integer to avoid
// floating-point drift in budget accounting. The pricing table maps a provider
// to its per-million-token rates; unknown providers cost zero (treated as free
// for display purposes, consistent with KeiRouter's savings-tracker model).
package meter

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/mydisha/keirouter/backend/internal/core"
	"github.com/mydisha/keirouter/backend/internal/store"
)

// Price holds per-million-token rates in USD.
type Price struct {
	InputPerM  float64
	OutputPerM float64
}

// Meter records usage rows and computes cost from a pricing table.
type Meter struct {
	usage   *store.UsageRepo
	pricing map[string]Price
}

// New builds a Meter backed by a usage repo and a provider pricing table.
func New(usage *store.UsageRepo, pricing map[string]Price) *Meter {
	if pricing == nil {
		pricing = map[string]Price{}
	}
	return &Meter{usage: usage, pricing: pricing}
}

// Event captures the facts about one completed (or cached) request.
type Event struct {
	TenantID  string
	ProjectID string
	APIKeyID  string
	Provider  string
	Model     string
	AccountID string
	Usage     core.Usage
	CacheHit  bool
	Latency   time.Duration
}

// CostMicros returns the cost of a usage event in micros of USD. Cached
// requests cost nothing (the whole point of the cache).
func (m *Meter) CostMicros(provider string, u core.Usage, cacheHit bool) int64 {
	if cacheHit {
		return 0
	}
	p, ok := m.pricing[provider]
	if !ok {
		return 0
	}
	// micros = tokens / 1e6 * pricePerM * 1e6 = tokens * pricePerM.
	inputCost := float64(u.PromptTokens) * p.InputPerM
	outputCost := float64(u.CompletionTokens) * p.OutputPerM
	return int64(inputCost + outputCost)
}

// Record persists a usage row for an event and returns the computed cost.
func (m *Meter) Record(ctx context.Context, ev Event) (int64, error) {
	cost := m.CostMicros(ev.Provider, ev.Usage, ev.CacheHit)
	rec := store.UsageRecord{
		ID:               uuid.NewString(),
		TenantID:         ev.TenantID,
		ProjectID:        ev.ProjectID,
		APIKeyID:         ev.APIKeyID,
		Provider:         ev.Provider,
		Model:            ev.Model,
		AccountID:        ev.AccountID,
		PromptTokens:     ev.Usage.PromptTokens,
		CompletionTokens: ev.Usage.CompletionTokens,
		CachedTokens:     ev.Usage.CachedTokens,
		CostMicros:       cost,
		CacheHit:         ev.CacheHit,
		LatencyMS:        int(ev.Latency.Milliseconds()),
		CreatedAt:        time.Now(),
	}
	if err := m.usage.Record(ctx, rec); err != nil {
		return cost, err
	}
	return cost, nil
}

// PricingFromCatalog builds a pricing table from provider specs (provider id ->
// rates). Helper for wiring from the connectors catalog.
func PricingFromCatalog(specs []SpecPrice) map[string]Price {
	out := make(map[string]Price, len(specs))
	for _, s := range specs {
		out[s.ID] = Price{InputPerM: s.InputPerM, OutputPerM: s.OutputPerM}
	}
	return out
}

// SpecPrice is the minimal pricing projection of a provider spec, kept here to
// avoid a hard dependency on the connectors package.
type SpecPrice struct {
	ID         string
	InputPerM  float64
	OutputPerM float64
}