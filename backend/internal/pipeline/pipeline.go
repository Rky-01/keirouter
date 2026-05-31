// Package pipeline orchestrates the request lifecycle: it applies token-saving
// transforms, enforces budgets, selects an account via the dispatcher, executes
// the upstream call with fallback, and records usage.
//
// It operates entirely on canonical core types; dialect translation happens at
// the gateway edge, before and after the pipeline. Both unary and streaming
// paths share the same candidate-selection and fallback logic.
package pipeline

import (
	"context"
	"log/slog"
	"time"

	"github.com/mydisha/keirouter/backend/internal/budget"
	"github.com/mydisha/keirouter/backend/internal/capability"
	"github.com/mydisha/keirouter/backend/internal/core"
	"github.com/mydisha/keirouter/backend/internal/dispatch"
	"github.com/mydisha/keirouter/backend/internal/meter"
	"github.com/mydisha/keirouter/backend/internal/slimmer"
	"github.com/mydisha/keirouter/backend/internal/terse"
)

// Pipeline wires the request-processing stages together.
type Pipeline struct {
	dispatcher *dispatch.Dispatcher
	meter      *meter.Meter
	budget     *budget.Engine
	slimmer    *slimmer.Engine
	log        *slog.Logger
}

// Deps bundles the pipeline's collaborators.
type Deps struct {
	Dispatcher *dispatch.Dispatcher
	Meter      *meter.Meter
	Budget     *budget.Engine
	Slimmer    *slimmer.Engine
	Logger     *slog.Logger
}

// New builds a Pipeline.
func New(d Deps) *Pipeline {
	log := d.Logger
	if log == nil {
		log = slog.Default()
	}
	return &Pipeline{
		dispatcher: d.Dispatcher,
		meter:      d.Meter,
		budget:     d.Budget,
		slimmer:    d.Slimmer,
		log:        log,
	}
}

// Options carries per-request routing and token-saving settings resolved from
// the API key, request, and global config.
type Options struct {
	// Targets is the ordered fallback chain (provider+model candidates).
	Targets []dispatch.Target
	// Slimmer / Terse control token-saving transforms.
	Slimmer slimmer.Config
	Terse   terse.Config
}

// Result reports the outcome of a unary request for metering and audit.
type Result struct {
	Response   *core.ChatResponse
	Provider   string
	Model      string
	AccountID  string
	CostMicros int64
	Latency    time.Duration
	SlimStats  *slimmer.Stats
}

// Chat runs a non-streaming request through the full pipeline with fallback.
func (p *Pipeline) Chat(ctx context.Context, req *core.ChatRequest, opts Options) (*Result, error) {
	if err := p.preflight(ctx, req, opts); err != nil {
		return nil, err
	}
	slimStats := p.applyTokenSaving(req, opts)

	required := capability.Required(req)
	attempts, err := p.dispatcher.Plan(ctx, req.Metadata.TenantID, opts.Targets, required)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for _, attempt := range attempts {
		started := time.Now()
		attemptReq := cloneForAttempt(req, attempt.Target.Model)

		resp, callErr := attempt.Conn.Chat(ctx, attemptReq, attempt.Creds)
		latency := time.Since(started)

		if callErr != nil {
			pe := core.AsProviderError(callErr)
			lastErr = pe
			p.dispatcher.NoteFailure(ctx, attempt.Account.ID, pe)
			if !pe.Fallbackable() {
				return nil, pe
			}
			p.log.Warn("chat attempt failed, falling back",
				"provider", attempt.Target.Provider, "model", attempt.Target.Model, "kind", pe.Kind)
			continue
		}

		cost := p.record(ctx, req.Metadata, attempt, resp.Usage, false, latency)
		return &Result{
			Response:   resp,
			Provider:   attempt.Target.Provider,
			Model:      attempt.Target.Model,
			AccountID:  attempt.Account.ID,
			CostMicros: cost,
			Latency:    latency,
			SlimStats:  slimStats,
		}, nil
	}

	if lastErr == nil {
		lastErr = &core.ProviderError{Kind: core.ErrInternal, Message: "pipeline: no attempts executed"}
	}
	return nil, lastErr
}

// StreamResult is delivered before chunks start flowing, identifying the chosen
// target so the gateway can set response headers.
type StreamResult struct {
	Chunks    <-chan core.StreamChunk
	Provider  string
	Model     string
	AccountID string
}

// Stream runs a streaming request with fallback. Fallback applies only to the
// connection-establishment phase; once the first attempt's channel is returned,
// errors surface as ChunkError on that channel. Usage metering happens in a
// goroutine that observes the final usage chunk.
func (p *Pipeline) Stream(ctx context.Context, req *core.ChatRequest, opts Options) (*StreamResult, error) {
	if err := p.preflight(ctx, req, opts); err != nil {
		return nil, err
	}
	p.applyTokenSaving(req, opts)

	required := capability.Required(req)
	attempts, err := p.dispatcher.Plan(ctx, req.Metadata.TenantID, opts.Targets, required)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for _, attempt := range attempts {
		attemptReq := cloneForAttempt(req, attempt.Target.Model)
		started := time.Now()

		upstream, callErr := attempt.Conn.Stream(ctx, attemptReq, attempt.Creds)
		if callErr != nil {
			pe := core.AsProviderError(callErr)
			lastErr = pe
			p.dispatcher.NoteFailure(ctx, attempt.Account.ID, pe)
			if !pe.Fallbackable() {
				return nil, pe
			}
			continue
		}

		// Tee the upstream channel so we can meter terminal usage without
		// blocking the client consumer.
		out := make(chan core.StreamChunk, 16)
		meta := req.Metadata
		acc := attempt
		go p.pumpStream(ctx, upstream, out, meta, acc, started)

		return &StreamResult{
			Chunks:    out,
			Provider:  attempt.Target.Provider,
			Model:     attempt.Target.Model,
			AccountID: attempt.Account.ID,
		}, nil
	}

	if lastErr == nil {
		lastErr = &core.ProviderError{Kind: core.ErrInternal, Message: "pipeline: no attempts executed"}
	}
	return nil, lastErr
}

// pumpStream forwards chunks to the client channel while capturing usage for
// metering when the stream completes.
func (p *Pipeline) pumpStream(ctx context.Context, in <-chan core.StreamChunk, out chan<- core.StreamChunk,
	meta core.RequestMetadata, attempt dispatch.Attempt, started time.Time) {
	defer close(out)
	var usage core.Usage
	for chunk := range in {
		if chunk.Type == core.ChunkUsage && chunk.Usage != nil {
			usage = *chunk.Usage
		}
		select {
		case out <- chunk:
		case <-ctx.Done():
			return
		}
	}
	p.record(ctx, meta, attempt, usage, false, time.Since(started))
}

// preflight runs validation and the budget guard before any upstream call.
func (p *Pipeline) preflight(ctx context.Context, req *core.ChatRequest, opts Options) error {
	if len(opts.Targets) == 0 {
		return &core.ProviderError{Kind: core.ErrBadRequest, Message: "no routing targets resolved for model"}
	}
	if p.budget != nil {
		scope := budget.Scope{
			TenantID:  req.Metadata.TenantID,
			ProjectID: req.Metadata.ProjectID,
			APIKeyID:  req.Metadata.APIKeyID,
		}
		if err := p.budget.CheckOrError(ctx, scope); err != nil {
			return err
		}
	}
	return nil
}

// applyTokenSaving runs the slimmer and terse transforms in place.
func (p *Pipeline) applyTokenSaving(req *core.ChatRequest, opts Options) *slimmer.Stats {
	var stats *slimmer.Stats
	if p.slimmer != nil && opts.Slimmer.Enabled {
		stats = p.slimmer.Compress(req, opts.Slimmer)
		if stats != nil {
			p.log.Debug("slimmer compressed request", "saved_bytes", stats.Saved(), "hits", len(stats.Hits))
		}
	}
	terse.Apply(req, opts.Terse)
	return stats
}

// record meters a completed attempt; failures to record are logged, not fatal.
func (p *Pipeline) record(ctx context.Context, meta core.RequestMetadata, attempt dispatch.Attempt,
	usage core.Usage, cacheHit bool, latency time.Duration) int64 {
	if p.meter == nil {
		return 0
	}
	cost, err := p.meter.Record(ctx, meter.Event{
		TenantID:  meta.TenantID,
		ProjectID: meta.ProjectID,
		APIKeyID:  meta.APIKeyID,
		Provider:  attempt.Target.Provider,
		Model:     attempt.Target.Model,
		AccountID: attempt.Account.ID,
		Usage:     usage,
		CacheHit:  cacheHit,
		Latency:   latency,
	})
	if err != nil {
		p.log.Error("failed to record usage", "err", err)
	}
	return cost
}

// cloneForAttempt produces a shallow copy of the request with the candidate's
// model id, so each fallback attempt targets the right model without mutating
// the shared request.
func cloneForAttempt(req *core.ChatRequest, model string) *core.ChatRequest {
	clone := *req
	clone.Model = model
	return &clone
}
