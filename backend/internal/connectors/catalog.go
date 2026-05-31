package connectors

import "github.com/mydisha/keirouter/backend/internal/core"

// ProviderSpec describes a built-in provider: its id, the wire dialect it
// speaks, its default endpoint, and pricing for cost metering.
type ProviderSpec struct {
	ID          string
	DisplayName string
	Dialect     core.Dialect
	BaseURL     string
	// AuthKind is the default authentication mechanism (api_key, oauth, none).
	AuthKind string
	// Pricing (USD per million tokens) used for cost estimation. Zero means
	// free or unknown.
	InputPerM  float64
	OutputPerM float64
}

// Catalog returns the built-in provider specs. This is intentionally a curated
// starter set covering the major OpenAI-compatible and Anthropic-native
// endpoints; more providers (OAuth, media, reverse-engineered) are layered on
// in later phases.
func Catalog() []ProviderSpec {
	return []ProviderSpec{
		{
			ID: "openai", DisplayName: "OpenAI", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.openai.com/v1", AuthKind: "api_key",
			InputPerM: 2.5, OutputPerM: 10,
		},
		{
			ID: "anthropic", DisplayName: "Anthropic", Dialect: core.DialectAnthropic,
			BaseURL: "https://api.anthropic.com/v1", AuthKind: "api_key",
			InputPerM: 3, OutputPerM: 15,
		},
		{
			ID: "deepseek", DisplayName: "DeepSeek", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.deepseek.com/v1", AuthKind: "api_key",
			InputPerM: 0.27, OutputPerM: 1.1,
		},
		{
			ID: "groq", DisplayName: "Groq", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.groq.com/openai/v1", AuthKind: "api_key",
			InputPerM: 0.59, OutputPerM: 0.79,
		},
		{
			ID: "glm", DisplayName: "GLM (Zhipu)", Dialect: core.DialectOpenAI,
			BaseURL: "https://open.bigmodel.cn/api/paas/v4", AuthKind: "api_key",
			InputPerM: 0.6, OutputPerM: 0.6,
		},
		{
			ID: "minimax", DisplayName: "MiniMax", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.minimax.io/v1", AuthKind: "api_key",
			InputPerM: 0.2, OutputPerM: 1.1,
		},
		{
			ID: "openrouter", DisplayName: "OpenRouter", Dialect: core.DialectOpenAI,
			BaseURL: "https://openrouter.ai/api/v1", AuthKind: "api_key",
		},
		{
			ID: "together", DisplayName: "Together AI", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.together.xyz/v1", AuthKind: "api_key",
		},
		{
			// A generic OpenAI-compatible endpoint configured entirely via the
			// account's base URL. Lets users point at any compatible gateway.
			ID: "custom-openai", DisplayName: "Custom (OpenAI-compatible)", Dialect: core.DialectOpenAI,
			BaseURL: "", AuthKind: "api_key",
		},
		{
			ID: "custom-anthropic", DisplayName: "Custom (Anthropic-compatible)", Dialect: core.DialectAnthropic,
			BaseURL: "", AuthKind: "api_key",
		},
	}
}

// SpecByID returns the catalog spec for a provider id, or false if unknown.
func SpecByID(id string) (ProviderSpec, bool) {
	for _, p := range Catalog() {
		if p.ID == id {
			return p, true
		}
	}
	return ProviderSpec{}, false
}