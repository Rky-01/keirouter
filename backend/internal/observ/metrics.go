// Package observ provides KeiRouter's native observability: Prometheus metrics
// covering request volume, latency, token usage, cost, fallbacks, and cache
// hits. Metrics are exposed at /metrics for scraping by Prometheus/Grafana.
//
// Keeping instrumentation in one place lets the pipeline and gateway record
// signals through a single typed API rather than scattering metric names.
package observ

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds the registered Prometheus collectors.
type Metrics struct {
	registry *prometheus.Registry

	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	TokensTotal     *prometheus.CounterVec
	CostMicros      *prometheus.CounterVec
	Fallbacks       *prometheus.CounterVec
	CacheHits       prometheus.Counter
	CacheMisses     prometheus.Counter
	UpstreamErrors  *prometheus.CounterVec
}

// New builds a Metrics instance backed by a dedicated registry. Using a private
// registry (rather than the global default) keeps tests isolated and avoids
// duplicate-registration panics.
func New() *Metrics {
	reg := prometheus.NewRegistry()
	factory := promauto.With(reg)

	m := &Metrics{
		registry: reg,
		RequestsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "keirouter_requests_total",
			Help: "Total chat requests, labeled by provider, model, and outcome.",
		}, []string{"provider", "model", "outcome"}),
		RequestDuration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "keirouter_request_duration_seconds",
			Help:    "Upstream request latency in seconds.",
			Buckets: prometheus.ExponentialBuckets(0.05, 2, 12), // 50ms .. ~100s
		}, []string{"provider", "model"}),
		TokensTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "keirouter_tokens_total",
			Help: "Total tokens processed, labeled by provider, model, and kind.",
		}, []string{"provider", "model", "kind"}),
		CostMicros: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "keirouter_cost_micros_total",
			Help: "Estimated cost in micros of USD, labeled by provider and model.",
		}, []string{"provider", "model"}),
		Fallbacks: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "keirouter_fallbacks_total",
			Help: "Fallback events, labeled by the error kind that triggered them.",
		}, []string{"kind"}),
		CacheHits: factory.NewCounter(prometheus.CounterOpts{
			Name: "keirouter_cache_hits_total",
			Help: "Semantic cache hits.",
		}),
		CacheMisses: factory.NewCounter(prometheus.CounterOpts{
			Name: "keirouter_cache_misses_total",
			Help: "Semantic cache misses.",
		}),
		UpstreamErrors: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "keirouter_upstream_errors_total",
			Help: "Upstream errors, labeled by provider and error kind.",
		}, []string{"provider", "kind"}),
	}
	return m
}

// Registry exposes the underlying Prometheus registry for the /metrics handler.
func (m *Metrics) Registry() *prometheus.Registry { return m.registry }

// RecordRequest records a completed (or failed) request's outcome, latency,
// tokens, and cost in one call.
func (m *Metrics) RecordRequest(provider, model, outcome string, seconds float64,
	promptTokens, completionTokens, cachedTokens int, costMicros int64) {
	m.RequestsTotal.WithLabelValues(provider, model, outcome).Inc()
	m.RequestDuration.WithLabelValues(provider, model).Observe(seconds)
	if promptTokens > 0 {
		m.TokensTotal.WithLabelValues(provider, model, "prompt").Add(float64(promptTokens))
	}
	if completionTokens > 0 {
		m.TokensTotal.WithLabelValues(provider, model, "completion").Add(float64(completionTokens))
	}
	if cachedTokens > 0 {
		m.TokensTotal.WithLabelValues(provider, model, "cached").Add(float64(cachedTokens))
	}
	if costMicros > 0 {
		m.CostMicros.WithLabelValues(provider, model).Add(float64(costMicros))
	}
}

// RecordFallback notes that a fallback occurred due to the given error kind.
func (m *Metrics) RecordFallback(kind string) {
	m.Fallbacks.WithLabelValues(kind).Inc()
}

// RecordUpstreamError counts an upstream error by provider and kind.
func (m *Metrics) RecordUpstreamError(provider, kind string) {
	m.UpstreamErrors.WithLabelValues(provider, kind).Inc()
}

// RecordCache notes a cache hit or miss.
func (m *Metrics) RecordCache(hit bool) {
	if hit {
		m.CacheHits.Inc()
	} else {
		m.CacheMisses.Inc()
	}
}