package health

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/mydisha/keirouter/backend/internal/store"
)

// ProviderTelemetryEvent captures one completed provider attempt for health
// aggregation. It mirrors the spec telemetry event; sensitive request/response
// bodies are never included.
type ProviderTelemetryEvent struct {
	Timestamp         time.Time
	Provider          string
	ProviderAccountID string
	Model             string
	Capability        string

	Status string // "success" | "failed"
	// HTTPStatus is the upstream HTTP status code (0 when not applicable).
	HTTPStatus int
	LatencyMs  int
	TTFTMs     int

	InputTokens  int
	OutputTokens int
	CostMicroUSD int64

	ErrorType    ProviderErrorType
	ErrorMessage string

	ChainID            string
	FallbackTriggered  bool
	FallbackToProvider string
	FallbackToModel    string
	FinalFailure       bool
}

// LatencyThresholds maps a capability to its p95 latency threshold in ms.
type LatencyThresholds map[string]int

// DefaultLatencyThresholds returns the spec's default per-capability p95
// thresholds used when none are configured.
func DefaultLatencyThresholds() LatencyThresholds {
	return LatencyThresholds{
		"chat_completions": 10_000,
		"embeddings":       5_000,
		"image_generation": 60_000,
		"audio":            60_000,
		"search":           15_000,
	}
}

// ThresholdFor resolves the p95 latency threshold for a capability, falling
// back to the chat default when unknown.
func (lt LatencyThresholds) ThresholdFor(capability string) int {
	if v, ok := lt[capability]; ok && v > 0 {
		return v
	}
	if v, ok := lt["chat_completions"]; ok && v > 0 {
		return v
	}
	return 10_000
}

// Config controls the health telemetry service.
type Config struct {
	// Enabled gates telemetry recording. When false, Record is a no-op.
	Enabled bool
	// QueueSize is the async event channel capacity. Full queues drop events.
	QueueSize int
	// CurrentFlushInterval is how often provider_health_current is recomputed.
	CurrentFlushInterval time.Duration
	// SnapshotInterval is how often 1-minute snapshot rows are written.
	SnapshotInterval time.Duration
	// RollingWindow is the lookback for current-state aggregation.
	RollingWindow time.Duration
	// MaxSamplesPerBucket caps latency/TTFT samples kept per minute bucket.
	MaxSamplesPerBucket int
	// LatencyThresholds per capability (ms).
	LatencyThresholds LatencyThresholds
}

// Service records provider telemetry events asynchronously and aggregates them
// into provider_health_current (fast dashboard load) and
// provider_health_snapshots (historical charts).
//
// All recording is best-effort: a full queue or a DB write failure logs a
// warning and continues. The gateway request path never blocks on telemetry.
type Service struct {
	cfg  Config
	log  *slog.Logger
	repo *store.ProviderHealthRepo

	ch      chan ProviderTelemetryEvent
	once    sync.Once
	done    chan struct{}
	started bool

	mu      sync.Mutex // guards states + chains maps
	states  map[string]*keyState
	chains  map[string]*chainState // key = chain_id
	dropped atomic.Uint64
}

// chainState rolls up telemetry across all providers in one chain so the
// dashboard can show fallback rate and final-failure count per chain.
type chainState struct {
	chainID     string
	buckets     map[int64]*chainBucket // key = unix minute
	lastUpdated time.Time
}

type chainBucket struct {
	minute     time.Time
	requests   int64
	successes  int64
	failures   int64
	fallbacks  int64
	finalFails int64
}

type keyState struct {
	provider            string
	account             string
	model               string
	capability          string
	buckets             map[int64]*minuteBucket // key = unix minute
	consecutiveFailures int
	lastSuccess         *time.Time
	lastFailure         *time.Time
}

type minuteBucket struct {
	minute              time.Time
	revision            uint64
	snapshottedRevision uint64
	requests            int64
	successes           int64
	failures            int64
	fallbacks           int64
	finalFails          int64
	inputTokens         int64
	outputTokens        int64
	costMicros          int64
	latencies           []int
	ttfts               []int
	errCounts           map[ProviderErrorType]int64
}

// New builds a health telemetry Service. The caller must call Start to launch
// the background aggregator and Close on shutdown.
func New(cfg Config, log *slog.Logger, repo *store.ProviderHealthRepo) *Service {
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 5000
	}
	if cfg.CurrentFlushInterval <= 0 {
		cfg.CurrentFlushInterval = 30 * time.Second
	}
	if cfg.SnapshotInterval <= 0 {
		cfg.SnapshotInterval = 60 * time.Second
	}
	if cfg.RollingWindow <= 0 {
		cfg.RollingWindow = 15 * time.Minute
	}
	if cfg.MaxSamplesPerBucket <= 0 {
		cfg.MaxSamplesPerBucket = 500
	}
	if cfg.LatencyThresholds == nil {
		cfg.LatencyThresholds = DefaultLatencyThresholds()
	}
	if log == nil {
		log = slog.Default()
	}
	return &Service{
		cfg:    cfg,
		log:    log,
		repo:   repo,
		ch:     make(chan ProviderTelemetryEvent, cfg.QueueSize),
		done:   make(chan struct{}),
		states: make(map[string]*keyState),
		chains: make(map[string]*chainState),
	}
}

// Record enqueues a telemetry event. Non-blocking: when the queue is full the
// event is dropped with a debug log so the request path is never delayed.
func (s *Service) Record(ev ProviderTelemetryEvent) {
	if s == nil || !s.cfg.Enabled {
		return
	}
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now()
	}
	select {
	case s.ch <- ev:
	default:
		s.dropped.Add(1)
		s.log.Debug("health telemetry queue full; dropping event",
			"provider", ev.Provider, "model", ev.Model)
	}
}

// DroppedEvents reports telemetry events lost because the non-blocking queue
// was full. Surfacing this value lets dashboards qualify health coverage.
func (s *Service) DroppedEvents() uint64 {
	if s == nil {
		return 0
	}
	return s.dropped.Load()
}

// RollingWindow reports the configured lookback represented by
// provider_health_current. Dashboards must expose this actual collector window
// instead of implying that an arbitrary requested range was applied.
func (s *Service) RollingWindow() time.Duration {
	if s == nil {
		return 0
	}
	return s.cfg.RollingWindow
}

// ChainStat is the rolled-up health of one routing chain over the rolling
// window, used by the chain-impact view.
type ChainStat struct {
	ChainID          string
	Requests         int64
	Successes        int64
	Failures         int64
	Fallbacks        int64
	FinalFailures    int64
	FallbackRate     float64 // 0-1
	FinalFailureRate float64 // 0-1
	LastUpdated      time.Time
}

// ChainStats returns rolled-up per-chain stats over the rolling window. Chains
// with no recent telemetry are omitted.
func (s *Service) ChainStats() []ChainStat {
	if s == nil {
		return nil
	}
	windowStart := time.Now().UTC().Add(-s.cfg.RollingWindow).Truncate(time.Minute)
	s.mu.Lock()
	out := make([]ChainStat, 0, len(s.chains))
	for id, cs := range s.chains {
		var agg ChainStat
		agg.ChainID = id
		for m, b := range cs.buckets {
			if b.minute.Before(windowStart) {
				continue
			}
			_ = m
			agg.Requests += b.requests
			agg.Successes += b.successes
			agg.Failures += b.failures
			agg.Fallbacks += b.fallbacks
			agg.FinalFailures += b.finalFails
		}
		if agg.Requests > 0 {
			agg.FallbackRate = float64(agg.Fallbacks) / float64(agg.Requests)
			agg.FinalFailureRate = float64(agg.FinalFailures) / float64(agg.Requests)
		}
		agg.LastUpdated = cs.lastUpdated
		if agg.Requests > 0 || agg.Fallbacks > 0 {
			out = append(out, agg)
		}
	}
	s.mu.Unlock()
	return out
}

// Start launches the drain + aggregator goroutine once.
func (s *Service) Start(ctx context.Context) {
	if s == nil || !s.cfg.Enabled {
		return
	}
	s.once.Do(func() {
		s.started = true
		go s.run(ctx)
	})
}

// Close stops the service and waits for the aggregator to exit. No-op when the
// service was never started or is disabled.
func (s *Service) Close(timeout time.Duration) {
	if s == nil {
		return
	}
	if !s.cfg.Enabled || !s.started {
		return
	}
	close(s.ch)
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-s.done:
	case <-timer.C:
	}
}

func (s *Service) run(ctx context.Context) {
	defer close(s.done)
	currentTick := time.NewTicker(s.cfg.CurrentFlushInterval)
	snapTick := time.NewTicker(s.cfg.SnapshotInterval)
	defer currentTick.Stop()
	defer snapTick.Stop()

	for {
		select {
		case <-ctx.Done():
			s.flushCurrent(context.Background())
			s.flushSnapshots(context.Background(), true)
			return
		case ev, ok := <-s.ch:
			if !ok {
				s.flushCurrent(context.Background())
				s.flushSnapshots(context.Background(), true)
				return
			}
			s.ingest(ev)
		case <-currentTick.C:
			s.flushCurrent(context.Background())
		case <-snapTick.C:
			s.flushSnapshots(context.Background(), false)
		}
	}
}

func (s *Service) ingest(ev ProviderTelemetryEvent) {
	min := ev.Timestamp.UTC().Truncate(time.Minute)
	s.mu.Lock()

	// A final-failure marker exists only to close the chain-level request. The
	// concrete provider attempt was already recorded immediately before it;
	// aggregating the marker again would double-count failed requests and create
	// a second account-less provider row.
	if ev.FinalFailure && ev.Provider != "" {
		key := store.HealthKey(ev.Provider, ev.ProviderAccountID, ev.Model, ev.Capability)
		if ks, ok := s.states[key]; ok {
			if bucket, ok := ks.buckets[min.Unix()]; ok {
				bucket.finalFails++
				bucket.revision++
			}
		}
	}
	if !ev.FinalFailure && ev.Provider != "" {
		key := store.HealthKey(ev.Provider, ev.ProviderAccountID, ev.Model, ev.Capability)
		ks, ok := s.states[key]
		if !ok {
			ks = &keyState{
				provider:   ev.Provider,
				account:    ev.ProviderAccountID,
				model:      ev.Model,
				capability: ev.Capability,
				buckets:    make(map[int64]*minuteBucket),
			}
			s.states[key] = ks
		}
		bucket, ok := ks.buckets[min.Unix()]
		if !ok {
			bucket = &minuteBucket{minute: min, errCounts: make(map[ProviderErrorType]int64)}
			ks.buckets[min.Unix()] = bucket
		}
		bucket.requests++
		bucket.inputTokens += int64(ev.InputTokens)
		bucket.outputTokens += int64(ev.OutputTokens)
		bucket.costMicros += ev.CostMicroUSD
		if ev.Status == "success" {
			bucket.successes++
			ks.consecutiveFailures = 0
			now := ev.Timestamp
			ks.lastSuccess = &now
		} else {
			bucket.failures++
			bucket.errCounts[ev.ErrorType]++
			ks.consecutiveFailures++
			now := ev.Timestamp
			ks.lastFailure = &now
		}
		if ev.FallbackTriggered && ev.Status == "failed" {
			bucket.fallbacks++
		}
		if ev.LatencyMs > 0 && len(bucket.latencies) < s.cfg.MaxSamplesPerBucket {
			bucket.latencies = append(bucket.latencies, ev.LatencyMs)
		}
		if ev.TTFTMs > 0 && len(bucket.ttfts) < s.cfg.MaxSamplesPerBucket {
			bucket.ttfts = append(bucket.ttfts, ev.TTFTMs)
		}
		bucket.revision++
	}

	// Chain-level rollup: only terminal events (success or FinalFailure)
	// count as requests. Non-terminal per-attempt failures are ignored here.
	if ev.ChainID != "" {
		cs, ok := s.chains[ev.ChainID]
		if !ok {
			cs = &chainState{chainID: ev.ChainID, buckets: make(map[int64]*chainBucket)}
			s.chains[ev.ChainID] = cs
		}
		cb, ok := cs.buckets[min.Unix()]
		if !ok {
			cb = &chainBucket{minute: min}
			cs.buckets[min.Unix()] = cb
		}
		if ev.Status == "success" || ev.FinalFailure {
			cb.requests++
			if ev.Status == "success" {
				cb.successes++
			} else {
				cb.failures++
				cb.finalFails++
			}
			if ev.FallbackTriggered {
				cb.fallbacks++
			}
		}
		cs.lastUpdated = ev.Timestamp
	}
	s.mu.Unlock()
}

// flushCurrent recomputes provider_health_current for every active key from its
// rolling window of minute buckets.
func (s *Service) flushCurrent(ctx context.Context) {
	if s.repo == nil {
		return
	}
	s.mu.Lock()
	now := time.Now().UTC()
	windowStart := now.Add(-s.cfg.RollingWindow).Truncate(time.Minute)
	keys := make([]*keyState, 0, len(s.states))
	for _, ks := range s.states {
		keys = append(keys, ks)
	}
	s.mu.Unlock()
	if err := s.repo.DeleteStaleTrafficCurrent(ctx, windowStart); err != nil {
		s.log.Warn("stale health current cleanup failed", "err", err)
	}

	for _, ks := range keys {
		agg := s.aggregate(ks, windowStart, now)
		if agg.requests == 0 {
			// Leave a concurrently refreshed probe-only row intact, but remove the
			// expired traffic aggregate so the API cannot label it as current.
			if err := s.repo.DeleteTrafficCurrent(ctx, ks.provider, ks.account, ks.model,
				ks.capability, now.Add(-time.Second)); err != nil {
				s.log.Warn("expired health current cleanup failed", "err", err,
					"provider", ks.provider, "model", ks.model)
			}
			continue
		}
		s.upsertCurrent(ctx, ks, agg, now)
	}
}

// flushSnapshots persists completed minute buckets while retaining them through
// the rolling window used by provider_health_current. A revision marker avoids
// rewriting unchanged buckets; late events make a retained bucket dirty and
// replace the same persisted snapshot on the next flush.
func (s *Service) flushSnapshots(ctx context.Context, flushAll bool) {
	if s.repo == nil {
		return
	}
	s.mu.Lock()
	now := time.Now().UTC()
	currentMinute := now.Truncate(time.Minute).Unix()
	windowStart := now.Add(-s.cfg.RollingWindow).Truncate(time.Minute)
	type pending struct {
		ks       *keyState
		minute   int64
		original *minuteBucket
		bucket   *minuteBucket
		revision uint64
	}
	var toWrite []pending
	for _, ks := range s.states {
		for m, b := range ks.buckets {
			completed := m < currentMinute || flushAll
			if completed && b.revision != b.snapshottedRevision {
				toWrite = append(toWrite, pending{
					ks: ks, minute: m, original: b, bucket: cloneMinuteBucket(b), revision: b.revision,
				})
			}
			if b.minute.Before(windowStart) && b.revision == b.snapshottedRevision {
				delete(ks.buckets, m)
			}
		}
	}
	s.mu.Unlock()

	for _, p := range toWrite {
		if err := s.writeSnapshot(ctx, p.ks, p.bucket); err != nil {
			s.log.Warn("health snapshot write failed", "err", err,
				"provider", p.ks.provider, "model", p.ks.model)
			continue
		}
		s.mu.Lock()
		// A late event may have changed the bucket during the DB write. Mark only
		// the exact revision persisted so that a newer revision is retried.
		if cur, ok := p.ks.buckets[p.minute]; ok && cur == p.original && cur.revision == p.revision {
			cur.snapshottedRevision = p.revision
			if cur.minute.Before(windowStart) {
				delete(p.ks.buckets, p.minute)
			}
		}
		s.mu.Unlock()
	}
}

func cloneMinuteBucket(source *minuteBucket) *minuteBucket {
	clone := *source
	clone.latencies = append([]int(nil), source.latencies...)
	clone.ttfts = append([]int(nil), source.ttfts...)
	clone.errCounts = make(map[ProviderErrorType]int64, len(source.errCounts))
	for errorType, count := range source.errCounts {
		clone.errCounts[errorType] = count
	}
	return &clone
}

type windowAgg struct {
	requests, successes, failures int64
	fallbacks, finalFails         int64
	inputTokens, outputTokens     int64
	costMicros                    int64
	consecutiveFailures           int
	lastSuccess, lastFailure      *time.Time
	errCounts                     map[ProviderErrorType]int64
	latencies                     []int
	ttfts                         []int
}

func (s *Service) aggregate(ks *keyState, windowStart, _ time.Time) windowAgg {
	s.mu.Lock()
	defer s.mu.Unlock()
	var a windowAgg
	a.errCounts = make(map[ProviderErrorType]int64)
	for _, b := range ks.buckets {
		if b.minute.Before(windowStart) {
			continue
		}
		a.requests += b.requests
		a.successes += b.successes
		a.failures += b.failures
		a.fallbacks += b.fallbacks
		a.finalFails += b.finalFails
		a.inputTokens += b.inputTokens
		a.outputTokens += b.outputTokens
		a.costMicros += b.costMicros
		for t, c := range b.errCounts {
			a.errCounts[t] += c
		}
		a.latencies = append(a.latencies, b.latencies...)
		a.ttfts = append(a.ttfts, b.ttfts...)
	}
	a.consecutiveFailures = ks.consecutiveFailures
	a.lastSuccess = ks.lastSuccess
	a.lastFailure = ks.lastFailure
	return a
}

func (s *Service) upsertCurrent(ctx context.Context, ks *keyState, a windowAgg, now time.Time) {
	if s.repo == nil {
		return
	}
	successRate := 0.0
	if a.requests > 0 {
		successRate = float64(a.successes) / float64(a.requests)
	}
	errorRate := 0.0
	if a.requests > 0 {
		errorRate = float64(a.failures) / float64(a.requests)
	}
	dominant := dominantErrorType(a.errCounts)
	p95Lat := percentileOf(a.latencies, 95)
	p95TTFT := percentileOf(a.ttfts, 95)
	threshold := s.cfg.LatencyThresholds.ThresholdFor(ks.capability)

	score := ComputeScore(ScoreInput{
		SuccessRate:         successRate,
		P95LatencyMs:        p95Lat,
		LatencyThresholdMs:  threshold,
		DominantErrorType:   dominant,
		ConsecutiveFailures: a.consecutiveFailures,
	})
	status := StatusFromScore(score, a.requests > 0, false)
	mainIssue := MainIssue(dominant, p95Lat, threshold, a.fallbacks)
	var recommendation string
	if mainIssue != "" {
		recommendation = RecommendationForIssue(mainIssue)
	}

	cur := store.ProviderHealthCurrent{
		ID:                  uuid.NewString(),
		Provider:            ks.provider,
		ProviderAccountID:   ks.account,
		Model:               ks.model,
		Capability:          ks.capability,
		HealthStatus:        status,
		HealthScore:         score,
		SuccessRate:         successRate,
		ErrorRate:           errorRate,
		RequestCount:        a.requests,
		FallbackCount:       a.fallbacks,
		LatencyP95Ms:        intPtr(p95Lat),
		TTFTP95Ms:           intPtr(p95TTFT),
		ConsecutiveFailures: a.consecutiveFailures,
		LastSuccessAt:       a.lastSuccess,
		LastFailureAt:       a.lastFailure,
		LastUpdatedAt:       now,
	}
	if mainIssue != "" {
		cur.MainIssue = &mainIssue
	}
	if recommendation != "" {
		cur.Recommendation = &recommendation
	}
	if err := s.repo.UpsertCurrent(ctx, cur); err != nil {
		s.log.Warn("health current upsert failed", "err", err,
			"provider", ks.provider, "model", ks.model)
	}
}

func (s *Service) writeSnapshot(ctx context.Context, ks *keyState, b *minuteBucket) error {
	successRate := 0.0
	if b.requests > 0 {
		successRate = float64(b.successes) / float64(b.requests)
	}
	dominant := dominantErrorType(b.errCounts)
	p50Lat := percentileOf(b.latencies, 50)
	p95Lat := percentileOf(b.latencies, 95)
	p99Lat := percentileOf(b.latencies, 99)
	p50TTFT := percentileOf(b.ttfts, 50)
	p95TTFT := percentileOf(b.ttfts, 95)
	p99TTFT := percentileOf(b.ttfts, 99)
	threshold := s.cfg.LatencyThresholds.ThresholdFor(ks.capability)

	score := ComputeScore(ScoreInput{
		SuccessRate:        successRate,
		P95LatencyMs:       p95Lat,
		LatencyThresholdMs: threshold,
		DominantErrorType:  dominant,
	})
	status := StatusFromScore(score, b.requests > 0, false)
	mainIssue := MainIssue(dominant, p95Lat, threshold, b.fallbacks)

	snap := store.ProviderHealthSnapshot{
		ID:                  uuid.NewString(),
		BucketStart:         b.minute,
		BucketSizeSeconds:   60,
		Provider:            ks.provider,
		ProviderAccountID:   ks.account,
		Model:               ks.model,
		Capability:          ks.capability,
		RequestCount:        b.requests,
		SuccessCount:        b.successes,
		FailureCount:        b.failures,
		FallbackCount:       b.fallbacks,
		FinalFailureCount:   b.finalFails,
		InputTokens:         b.inputTokens,
		OutputTokens:        b.outputTokens,
		EstimatedCostMicros: b.costMicros,
		LatencyP50Ms:        intPtr(p50Lat),
		LatencyP95Ms:        intPtr(p95Lat),
		LatencyP99Ms:        intPtr(p99Lat),
		TTFTP50Ms:           intPtr(p50TTFT),
		TTFTP95Ms:           intPtr(p95TTFT),
		TTFTP99Ms:           intPtr(p99TTFT),
		RateLimitedCount:    b.errCounts[ProviderErrorRateLimited],
		AuthErrorCount:      b.errCounts[ProviderErrorAuth],
		QuotaExceededCount:  b.errCounts[ProviderErrorQuotaExceeded],
		TimeoutCount:        b.errCounts[ProviderErrorTimeout],
		Provider5xxCount:    b.errCounts[ProviderErrorProvider5xx],
		BadRequestCount:     b.errCounts[ProviderErrorBadRequest],
		NetworkErrorCount:   b.errCounts[ProviderErrorNetwork],
		UnsupportedCount:    b.errCounts[ProviderErrorUnsupported],
		UnknownErrorCount:   b.errCounts[ProviderErrorUnknown],
		HealthScore:         score,
		HealthStatus:        status,
		CreatedAt:           time.Now(),
	}
	if mainIssue != "" {
		snap.MainIssue = &mainIssue
	}
	return s.repo.InsertSnapshot(ctx, snap)
}

func dominantErrorType(counts map[ProviderErrorType]int64) ProviderErrorType {
	var best ProviderErrorType
	var bestCount int64
	for t, c := range counts {
		if t == ProviderErrorNone {
			continue
		}
		if c > bestCount {
			best = t
			bestCount = c
		}
	}
	if bestCount == 0 {
		return ProviderErrorNone
	}
	return best
}

func percentileOf(samples []int, q float64) int {
	if len(samples) == 0 {
		return 0
	}
	return Percentile(SortInts(samples), q)
}

func intPtr(v int) *int {
	if v == 0 {
		return nil
	}
	return &v
}
