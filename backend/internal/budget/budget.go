// Package budget enforces spend limits before a request is dispatched.
//
// Budgets are defined per scope (tenant, project, or API key) with a hard cap
// in micros of USD over a rolling period. Before each request the engine sums
// the scope's spend in the current period; if a hard-cutoff budget is already
// at or over its limit, the request is rejected with ErrBudgetBlocked (which is
// not fallbackable). Alert thresholds let the UI warn before the cutoff.
package budget

import (
	"context"
	"fmt"
	"time"

	"github.com/mydisha/keirouter/backend/internal/core"
	"github.com/mydisha/keirouter/backend/internal/store"
)

// Engine evaluates budgets against recorded usage.
type Engine struct {
	budgets *store.BudgetRepo
	usage   *store.UsageRepo
}

// New builds a budget Engine.
func New(budgets *store.BudgetRepo, usage *store.UsageRepo) *Engine {
	return &Engine{budgets: budgets, usage: usage}
}

// Decision is the outcome of a budget check.
type Decision struct {
	// Allowed is false when a hard-cutoff budget is exhausted.
	Allowed bool
	// Blocking is the budget that caused a block (nil when allowed).
	Blocking *store.Budget
	// Alerts lists budgets that have crossed their alert threshold but not their
	// hard limit. Informational; does not block.
	Alerts []store.Budget
}

// Scope identifies the request's billing scopes to check.
type Scope struct {
	TenantID  string
	ProjectID string
	APIKeyID  string
}

// Check evaluates all budgets applicable to a scope and reports whether the
// request may proceed. It checks key, project, and tenant budgets; the first
// exhausted hard-cutoff budget blocks the request.
func (e *Engine) Check(ctx context.Context, scope Scope) (Decision, error) {
	dec := Decision{Allowed: true}

	checks := []struct {
		kind store.BudgetScope
		id   string
	}{
		{store.ScopeAPIKey, scope.APIKeyID},
		{store.ScopeProject, scope.ProjectID},
		{store.ScopeTenant, scope.TenantID},
	}

	for _, c := range checks {
		if c.id == "" {
			continue
		}
		budgets, err := e.budgets.ListByScope(ctx, c.kind, c.id)
		if err != nil {
			return Decision{}, fmt.Errorf("budget: list %s budgets: %w", c.kind, err)
		}
		for _, b := range budgets {
			spent, err := e.usage.SpendSince(ctx, b.ScopeKind, b.ScopeID, periodStart(b.Period, time.Now()))
			if err != nil {
				return Decision{}, fmt.Errorf("budget: spend lookup: %w", err)
			}
			switch {
			case spent >= b.LimitMicros && b.HardCutoff:
				dec.Allowed = false
				blocking := b
				dec.Blocking = &blocking
				return dec, nil
			case b.AlertPct > 0 && spent*100 >= b.LimitMicros*int64(b.AlertPct):
				dec.Alerts = append(dec.Alerts, b)
			}
		}
	}
	return dec, nil
}

// CheckOrError is a convenience wrapper that returns a ProviderError when the
// request is blocked, suitable for direct use in the pipeline.
func (e *Engine) CheckOrError(ctx context.Context, scope Scope) error {
	dec, err := e.Check(ctx, scope)
	if err != nil {
		return err
	}
	if !dec.Allowed {
		return &core.ProviderError{
			Kind:    core.ErrBudgetBlocked,
			Message: fmt.Sprintf("budget %q exhausted for %s", dec.Blocking.ID, dec.Blocking.ScopeKind),
		}
	}
	return nil
}

// periodStart returns the start of the current budget window for a period.
func periodStart(period string, now time.Time) time.Time {
	now = now.UTC()
	switch period {
	case "daily":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	case "weekly":
		// ISO-ish: start of the week on Monday.
		offset := (int(now.Weekday()) + 6) % 7
		d := now.AddDate(0, 0, -offset)
		return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
	case "monthly":
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	default: // "total"
		return time.Time{}
	}
}