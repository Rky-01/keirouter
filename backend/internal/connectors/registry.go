package connectors

import (
	"fmt"

	"github.com/mydisha/keirouter/backend/internal/core"
)

// Registry resolves connectors by provider id. It is built once at startup from
// the provider catalog and is read-only thereafter, so it is safe for
// concurrent use without locking.
type Registry struct {
	byID map[string]core.Connector
}

// NewRegistry builds a registry from the given connectors.
func NewRegistry(conns ...core.Connector) *Registry {
	m := make(map[string]core.Connector, len(conns))
	for _, c := range conns {
		m[c.ID()] = c
	}
	return &Registry{byID: m}
}

// Get returns the connector for a provider id.
func (r *Registry) Get(provider string) (core.Connector, error) {
	c, ok := r.byID[provider]
	if !ok {
		return nil, fmt.Errorf("connectors: no connector for provider %q", provider)
	}
	return c, nil
}

// Has reports whether a provider is registered.
func (r *Registry) Has(provider string) bool {
	_, ok := r.byID[provider]
	return ok
}

// Providers returns the registered provider ids.
func (r *Registry) Providers() []string {
	out := make([]string, 0, len(r.byID))
	for id := range r.byID {
		out = append(out, id)
	}
	return out
}

// DefaultRegistry builds the built-in connector set from the provider catalog.
// Each entry maps a provider id to its dialect and default endpoint.
func DefaultRegistry() *Registry {
	var conns []core.Connector
	for _, p := range Catalog() {
		switch p.Dialect {
		case core.DialectAnthropic:
			conns = append(conns, NewAnthropic(p.ID, p.BaseURL))
		default:
			conns = append(conns, NewOpenAICompatible(p.ID, p.BaseURL))
		}
	}
	return NewRegistry(conns...)
}