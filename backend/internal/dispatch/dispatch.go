// Package dispatch selects which provider account serves a request and, on
// failure, advances through fallback candidates.
//
// A Target names a provider+model. The dispatcher resolves the live accounts
// for a target's provider, skips accounts on cooldown or lacking the required
// capabilities, and tries them in priority order. When every account for a
// target is exhausted, it advances to the next target in the chain. Errors that
// are not fallbackable (e.g. a malformed request) short-circuit immediately.
package dispatch

import (
	"context"
	"fmt"
	"time"

	"github.com/mydisha/keirouter/backend/internal/capability"
	"github.com/mydisha/keirouter/backend/internal/core"
	"github.com/mydisha/keirouter/backend/internal/store"
	"github.com/mydisha/keirouter/backend/internal/vault"
)

// Target is one candidate in a fallback chain.
type Target struct {
	Provider string
	Model    string
}

// Attempt describes a single resolved try: the connector, credentials, and the
// account it came from. The pipeline executes attempts via the connector.
type Attempt struct {
	Target  Target
	Conn    core.Connector
	Creds   core.Credentials
	Account store.Account
}

// ConnectorSource resolves a connector by provider id.
type ConnectorSource interface {
	Get(provider string) (core.Connector, error)
}

// Dispatcher walks fallback chains, yielding resolved attempts.
type Dispatcher struct {
	conns    ConnectorSource
	accounts *store.AccountRepo
	vault    *vault.Vault
	// defaultCooldown is applied to an account when an error carries no
	// upstream-specified Retry-After.
	defaultCooldown time.Duration
}

// New builds a Dispatcher.
func New(conns ConnectorSource, accounts *store.AccountRepo, v *vault.Vault) *Dispatcher {
	return &Dispatcher{
		conns:           conns,
		accounts:        accounts,
		vault:           v,
		defaultCooldown: 60 * time.Second,
	}
}

// Plan resolves the ordered list of attempts for a chain of targets, scoped to
// a tenant and constrained to the given required capabilities. It returns an
// error only when no attempt could be resolved at all (no usable account for
// any target); otherwise the pipeline tries attempts in order.
func (d *Dispatcher) Plan(ctx context.Context, tenantID string, targets []Target, required core.CapabilitySet) ([]Attempt, error) {
	now := time.Now()
	var attempts []Attempt
	var lastReason string

	for _, target := range targets {
		// Capability guard: never fall back to a model that cannot honor the
		// request. This prevents silent quality downgrades.
		if !capability.Supports(target.Model, required) {
			lastReason = fmt.Sprintf("model %q lacks required capabilities", target.Model)
			continue
		}

		conn, err := d.conns.Get(target.Provider)
		if err != nil {
			lastReason = err.Error()
			continue
		}

		accs, err := d.accounts.ListByProvider(ctx, tenantID, target.Provider)
		if err != nil {
			return nil, fmt.Errorf("dispatch: list accounts for %s: %w", target.Provider, err)
		}
		if len(accs) == 0 {
			lastReason = fmt.Sprintf("no accounts configured for provider %q", target.Provider)
			continue
		}

		for _, acc := range accs {
			if acc.CooldownUntil != nil && acc.CooldownUntil.After(now) {
				lastReason = fmt.Sprintf("account %s on cooldown", acc.ID)
				continue
			}
			creds, err := d.vault.Open(acc)
			if err != nil {
				lastReason = err.Error()
				continue
			}
			attempts = append(attempts, Attempt{
				Target:  target,
				Conn:    conn,
				Creds:   creds,
				Account: acc,
			})
		}
	}

	if len(attempts) == 0 {
		if lastReason == "" {
			lastReason = "no usable targets in chain"
		}
		return nil, &core.ProviderError{Kind: core.ErrInternal, Message: "dispatch: " + lastReason}
	}
	return attempts, nil
}

// NoteFailure applies a cooldown to an account based on a provider error, so
// subsequent Plan calls skip it while it recovers. Rate-limit and quota errors
// honor the upstream Retry-After when present.
func (d *Dispatcher) NoteFailure(ctx context.Context, accountID string, err *core.ProviderError) {
	if err == nil {
		return
	}
	var cooldown time.Duration
	switch err.Kind {
	case core.ErrRateLimit:
		cooldown = d.defaultCooldown
		if err.RetryAfter > 0 {
			cooldown = err.RetryAfter
		}
	case core.ErrQuotaExhausted:
		cooldown = 30 * time.Minute
		if err.RetryAfter > 0 {
			cooldown = err.RetryAfter
		}
	case core.ErrAuth:
		cooldown = 5 * time.Minute
	default:
		return // transient/upstream errors don't warrant a persisted cooldown
	}
	_ = d.accounts.SetCooldown(ctx, accountID, time.Now().Add(cooldown))
}

// TargetsFromChain flattens a stored chain into ordered targets.
func TargetsFromChain(chain store.Chain) []Target {
	out := make([]Target, 0, len(chain.Steps))
	for _, s := range chain.Steps {
		out = append(out, Target{Provider: s.Provider, Model: s.Model})
	}
	return out
}