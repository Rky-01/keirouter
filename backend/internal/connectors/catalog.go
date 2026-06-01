package connectors

import "github.com/mydisha/keirouter/backend/internal/core"

// ProviderSpec describes a built-in provider: its id, the wire dialect it
// speaks, its default endpoint, the service kinds it serves, and the metadata
// the dashboard renders (display name, alias, brand color, auth modes, etc.).
type ProviderSpec struct {
	ID          string
	DisplayName string
	// Alias is the short code accepted in model strings (e.g. "ds" -> deepseek).
	Alias   string
	Dialect core.Dialect
	BaseURL string
	// AuthKind is the default authentication mechanism (api_key, oauth, none).
	AuthKind string
	// AuthModes lists every supported auth mechanism (a provider may offer both
	// OAuth and API key). Defaults to [AuthKind] when empty.
	AuthModes []string
	// ServiceKinds enumerates the capabilities this provider serves. Empty means
	// LLM-only (the conservative default).
	ServiceKinds []core.ServiceKind
	// Color is the brand color used by the dashboard (hex).
	Color string
	// Website is the provider's marketing/console URL.
	Website string
	// APIKeyURL is where a user mints an API key for this provider.
	APIKeyURL string
	// Deprecated marks providers that carry usage risk (unofficial OAuth, etc.).
	Deprecated bool
	// Hidden hides the provider from the default dashboard listing.
	Hidden bool
	// Notice is a short human-readable note shown in the dashboard.
	Notice string
	// Pricing (USD per million tokens) used for cost estimation. Zero means
	// free or unknown.
	InputPerM  float64
	OutputPerM float64
}

// llm is shorthand for the default LLM-only service kind slice.
func llm(extra ...core.ServiceKind) []core.ServiceKind {
	return append([]core.ServiceKind{core.ServiceLLM}, extra...)
}

// Catalog returns the built-in provider specs. It mirrors the upstream 9router
// catalog: free OAuth providers, free-tier providers, OAuth providers, the
// large API-key provider set, and dedicated media providers (image, STT, TTS,
// web search, web fetch, embeddings).
//
// Dialect determines which connector backs a provider. Providers whose upstream
// dialect KeiRouter does not yet drive natively (kiro, cursor, antigravity,
// gemini-cli, vertex, commandcode, web-cookie) are present for discovery and
// account management but only become routable once their connector lands.
func Catalog() []ProviderSpec {
	return append(append(append(append(
		freeProviders(),
		freeTierProviders()...),
		oauthProviders()...),
		apiKeyProviders()...),
		mediaProviders()...)
}

// freeProviders are subscription/OAuth-session providers (use at your own risk).
func freeProviders() []ProviderSpec {
	risk := "Uses a subscription/OAuth session not licensed for proxy use. Account may be restricted. Use at your own risk."
	return []ProviderSpec{
		{ID: "kiro", DisplayName: "Kiro AI", Alias: "kr", Dialect: core.DialectKiro,
			BaseURL: "https://codewhisperer.us-east-1.amazonaws.com/generateAssistantResponse",
			AuthKind: "oauth", AuthModes: []string{"oauth"}, ServiceKinds: llm(),
			Color: "#FF6B35", Website: "https://kiro.dev", Deprecated: true, Notice: risk},
		{ID: "gemini-cli", DisplayName: "Gemini CLI", Alias: "gc", Dialect: core.DialectGeminiCLI,
			BaseURL: "https://cloudcode-pa.googleapis.com/v1internal",
			AuthKind: "oauth", AuthModes: []string{"oauth"}, ServiceKinds: llm(),
			Color: "#4285F4", Website: "https://github.com/google-gemini/gemini-cli", Deprecated: true, Notice: risk},
		{ID: "qoder", DisplayName: "Qoder", Alias: "qd", Dialect: core.DialectOpenAI,
			BaseURL: "https://api3.qoder.sh/algo/api/v2/service/pro/sse/agent_chat_generation",
			AuthKind: "oauth", AuthModes: []string{"oauth"}, ServiceKinds: llm(),
			Color: "#EC4899", Website: "https://qoder.com", Deprecated: true, Notice: risk},
		{ID: "opencode", DisplayName: "OpenCode Free", Alias: "oc", Dialect: core.DialectOpenAI,
			BaseURL: "https://opencode.ai", AuthKind: "none", AuthModes: []string{"none"}, ServiceKinds: llm(),
			Color: "#E87040", Website: "https://opencode.ai"},
	}
}

// freeTierProviders offer a free tier but may require an account or API key.
func freeTierProviders() []ProviderSpec {
	return []ProviderSpec{
		{ID: "openrouter", DisplayName: "OpenRouter", Alias: "openrouter", Dialect: core.DialectOpenAI,
			BaseURL: "https://openrouter.ai/api/v1", AuthKind: "api_key",
			ServiceKinds: llm(core.ServiceEmbedding), Color: "#F97316",
			Website: "https://openrouter.ai", APIKeyURL: "https://openrouter.ai/settings/keys",
			Notice: "Free tier: 27+ free models, no credit card, 200 req/day."},
		{ID: "nvidia", DisplayName: "NVIDIA NIM", Alias: "nvidia", Dialect: core.DialectOpenAI,
			BaseURL: "https://integrate.api.nvidia.com/v1", AuthKind: "api_key",
			ServiceKinds: llm(core.ServiceTTS, core.ServiceEmbedding), Color: "#76B900",
			Website: "https://developer.nvidia.com/nim", APIKeyURL: "https://build.nvidia.com/settings/api-keys"},
		{ID: "ollama", DisplayName: "Ollama Cloud", Alias: "ollama", Dialect: core.DialectOllama,
			BaseURL: "https://ollama.com", AuthKind: "api_key", ServiceKinds: llm(),
			Color: "#111111", Website: "https://ollama.com", APIKeyURL: "https://ollama.com/settings/keys"},
		{ID: "ollama-local", DisplayName: "Ollama Local", Alias: "ollama-local", Dialect: core.DialectOllama,
			BaseURL: "http://localhost:11434", AuthKind: "none", AuthModes: []string{"none"}, ServiceKinds: llm(),
			Color: "#111111", Website: "https://ollama.com"},
		{ID: "gemini", DisplayName: "Gemini", Alias: "gemini", Dialect: core.DialectGemini,
			BaseURL: "https://generativelanguage.googleapis.com/v1beta", AuthKind: "api_key",
			ServiceKinds: llm(core.ServiceEmbedding, core.ServiceImage, core.ServiceSearch, core.ServiceTTS, core.ServiceSTT),
			Color: "#4285F4", Website: "https://ai.google.dev", APIKeyURL: "https://aistudio.google.com/app/apikey"},
		{ID: "vertex", DisplayName: "Vertex AI", Alias: "vx", Dialect: core.DialectVertex,
			BaseURL: "https://aiplatform.googleapis.com", AuthKind: "oauth", AuthModes: []string{"oauth"}, ServiceKinds: llm(),
			Color: "#4285F4", Website: "https://cloud.google.com/vertex-ai", Hidden: true},
		{ID: "cloudflare-ai", DisplayName: "Cloudflare", Alias: "cf", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.cloudflare.com/client/v4/accounts/{accountId}/ai/v1", AuthKind: "api_key",
			ServiceKinds: llm(core.ServiceImage), Color: "#F38020",
			Website: "https://developers.cloudflare.com/workers-ai/", APIKeyURL: "https://dash.cloudflare.com/profile/api-tokens"},
		{ID: "byteplus", DisplayName: "BytePlus ModelArk", Alias: "bpm", Dialect: core.DialectOpenAI,
			BaseURL: "https://ark.ap-southeast.bytepluses.com/api/coding/v3", AuthKind: "api_key", ServiceKinds: llm(),
			Color: "#2563EB", Website: "https://console.byteplus.com/ark"},
	}
}

// oauthProviders authenticate via OAuth (some also accept an API key).
func oauthProviders() []ProviderSpec {
	risk := "Uses a subscription/OAuth session not licensed for proxy use. Account may be restricted. Use at your own risk."
	return []ProviderSpec{
		{ID: "claude", DisplayName: "Claude Code", Alias: "cc", Dialect: core.DialectAnthropic,
			BaseURL: "https://api.anthropic.com/v1", AuthKind: "oauth", AuthModes: []string{"oauth"}, ServiceKinds: llm(),
			Color: "#D97757", Website: "https://claude.ai", Deprecated: true, Notice: risk},
		{ID: "antigravity", DisplayName: "Antigravity", Alias: "ag", Dialect: core.DialectAntigravity,
			BaseURL: "https://daily-cloudcode-pa.googleapis.com", AuthKind: "oauth", AuthModes: []string{"oauth"}, ServiceKinds: llm(),
			Color: "#F59E0B", Website: "https://antigravity.google", Deprecated: true, Notice: risk},
		{ID: "codex", DisplayName: "OpenAI Codex", Alias: "cx", Dialect: core.DialectOpenAIResponses,
			BaseURL: "https://chatgpt.com/backend-api/codex/responses", AuthKind: "oauth", AuthModes: []string{"oauth"},
			ServiceKinds: llm(core.ServiceImage), Color: "#3B82F6", Website: "https://chatgpt.com/codex", Deprecated: true, Notice: risk},
		{ID: "github", DisplayName: "GitHub Copilot", Alias: "gh", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.githubcopilot.com", AuthKind: "oauth", AuthModes: []string{"oauth"},
			ServiceKinds: llm(core.ServiceEmbedding), Color: "#333333", Website: "https://github.com/features/copilot", Deprecated: true, Notice: risk},
		{ID: "cursor", DisplayName: "Cursor IDE", Alias: "cu", Dialect: core.DialectCursor,
			BaseURL: "https://api2.cursor.sh", AuthKind: "oauth", AuthModes: []string{"oauth"}, ServiceKinds: llm(),
			Color: "#00D4AA", Website: "https://cursor.com"},
		{ID: "kilocode", DisplayName: "Kilo Code", Alias: "kc", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.kilo.ai/api/openrouter", AuthKind: "oauth", AuthModes: []string{"oauth"}, ServiceKinds: llm(),
			Color: "#FF6B35", Website: "https://kilocode.ai"},
		{ID: "cline", DisplayName: "Cline", Alias: "cl", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.cline.bot/api/v1", AuthKind: "oauth", AuthModes: []string{"oauth"}, ServiceKinds: llm(),
			Color: "#5B9BD5", Website: "https://cline.bot"},
		{ID: "qwen", DisplayName: "Qwen Code", Alias: "qwen", Dialect: core.DialectOpenAI,
			BaseURL: "https://portal.qwen.ai/v1/chat/completions", AuthKind: "oauth", AuthModes: []string{"oauth"}, ServiceKinds: llm(),
			Color: "#615CED", Website: "https://chat.qwen.ai", Deprecated: true, Notice: risk},
		{ID: "iflow", DisplayName: "iFlow", Alias: "iflow", Dialect: core.DialectOpenAI,
			BaseURL: "https://apis.iflow.cn/v1/chat/completions", AuthKind: "oauth", AuthModes: []string{"oauth", "api_key"}, ServiceKinds: llm(),
			Color: "#2563EB", Website: "https://iflow.cn", Deprecated: true, Notice: risk},
	}
}

// apiKeyProviders is the large API-key-authenticated provider set.
func apiKeyProviders() []ProviderSpec {
	return []ProviderSpec{
		{ID: "openai", DisplayName: "OpenAI", Alias: "openai", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.openai.com/v1", AuthKind: "api_key",
			ServiceKinds: llm(core.ServiceEmbedding, core.ServiceTTS, core.ServiceSTT, core.ServiceImage, core.ServiceSearch),
			Color: "#10A37F", Website: "https://platform.openai.com", APIKeyURL: "https://platform.openai.com/api-keys",
			InputPerM: 2.5, OutputPerM: 10},
		{ID: "anthropic", DisplayName: "Anthropic", Alias: "anthropic", Dialect: core.DialectAnthropic,
			BaseURL: "https://api.anthropic.com/v1", AuthKind: "api_key", ServiceKinds: llm(),
			Color: "#D97757", Website: "https://console.anthropic.com", APIKeyURL: "https://console.anthropic.com/settings/keys",
			InputPerM: 3, OutputPerM: 15},
		{ID: "deepseek", DisplayName: "DeepSeek", Alias: "ds", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.deepseek.com", AuthKind: "api_key", ServiceKinds: llm(),
			Color: "#4D6BFE", Website: "https://deepseek.com", APIKeyURL: "https://platform.deepseek.com/api_keys",
			InputPerM: 0.27, OutputPerM: 1.1},
		{ID: "glm", DisplayName: "GLM Coding", Alias: "glm", Dialect: core.DialectAnthropic,
			BaseURL: "https://api.z.ai/api/anthropic/v1", AuthKind: "api_key", ServiceKinds: llm(),
			Color: "#2563EB", Website: "https://open.bigmodel.cn", APIKeyURL: "https://open.bigmodel.cn/usercenter/apikeys",
			InputPerM: 0.6, OutputPerM: 0.6},
		{ID: "glm-cn", DisplayName: "GLM (China)", Alias: "glm-cn", Dialect: core.DialectOpenAI,
			BaseURL: "https://open.bigmodel.cn/api/coding/paas/v4", AuthKind: "api_key", ServiceKinds: llm(),
			Color: "#DC2626", Website: "https://open.bigmodel.cn", APIKeyURL: "https://open.bigmodel.cn/usercenter/apikeys",
			InputPerM: 0.6, OutputPerM: 0.6},
		{ID: "kimi", DisplayName: "Kimi", Alias: "kimi", Dialect: core.DialectAnthropic,
			BaseURL: "https://api.kimi.com/coding/v1", AuthKind: "api_key", ServiceKinds: llm(core.ServiceSearch),
			Color: "#1E3A8A", Website: "https://kimi.moonshot.cn", APIKeyURL: "https://platform.moonshot.ai/console/api-keys"},
		{ID: "minimax", DisplayName: "MiniMax Coding", Alias: "minimax", Dialect: core.DialectAnthropic,
			BaseURL: "https://api.minimax.io/anthropic/v1", AuthKind: "api_key",
			ServiceKinds: llm(core.ServiceImage, core.ServiceSearch, core.ServiceTTS), Color: "#7C3AED",
			Website: "https://www.minimaxi.com", APIKeyURL: "https://platform.minimaxi.com/user-center/basic-information/interface-key",
			InputPerM: 0.2, OutputPerM: 1.1},
		{ID: "minimax-cn", DisplayName: "MiniMax (China)", Alias: "minimax-cn", Dialect: core.DialectAnthropic,
			BaseURL: "https://api.minimaxi.com/anthropic/v1", AuthKind: "api_key", ServiceKinds: llm(core.ServiceTTS),
			Color: "#DC2626", Website: "https://www.minimaxi.com"},
		{ID: "alicode", DisplayName: "Alibaba", Alias: "alicode", Dialect: core.DialectOpenAI,
			BaseURL: "https://coding.dashscope.aliyuncs.com/v1", AuthKind: "api_key", ServiceKinds: llm(),
			Color: "#FF6A00", Website: "https://bailian.console.aliyun.com"},
		{ID: "alicode-intl", DisplayName: "Alibaba Intl", Alias: "alicode-intl", Dialect: core.DialectOpenAI,
			BaseURL: "https://coding-intl.dashscope.aliyuncs.com/v1", AuthKind: "api_key", ServiceKinds: llm(),
			Color: "#FF6A00", Website: "https://modelstudio.console.alibabacloud.com"},
		{ID: "xiaomi-mimo", DisplayName: "Xiaomi MiMo", Alias: "mimo", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.xiaomimimo.com/v1", AuthKind: "api_key", ServiceKinds: llm(),
			Color: "#FF6900", Website: "https://xiaomimimo.com"},
		{ID: "volcengine-ark", DisplayName: "Volcengine Ark", Alias: "ark", Dialect: core.DialectOpenAI,
			BaseURL: "https://ark.cn-beijing.volces.com/api/coding/v3", AuthKind: "api_key", ServiceKinds: llm(),
			Color: "#1677FF", Website: "https://ark.cn-beijing.volces.com"},
		{ID: "vercel-ai-gateway", DisplayName: "Vercel AI Gateway", Alias: "vercel", Dialect: core.DialectOpenAI,
			BaseURL: "https://ai-gateway.vercel.sh/v1", AuthKind: "api_key", ServiceKinds: llm(),
			Color: "#111827", Website: "https://vercel.com/ai-gateway"},
		{ID: "azure", DisplayName: "Azure OpenAI", Alias: "azure", Dialect: core.DialectOpenAI,
			BaseURL: "", AuthKind: "api_key", ServiceKinds: llm(), Color: "#0078D4",
			Website: "https://azure.microsoft.com/en-us/products/ai-services/openai-service"},
		{ID: "commandcode", DisplayName: "Command Code", Alias: "cmc", Dialect: core.DialectCommandCode,
			BaseURL: "https://api.commandcode.ai/alpha/generate", AuthKind: "api_key", ServiceKinds: llm(),
			Color: "#000000", Website: "https://commandcode.ai", Hidden: true},
		{ID: "groq", DisplayName: "Groq", Alias: "groq", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.groq.com/openai/v1", AuthKind: "api_key", ServiceKinds: llm(core.ServiceSTT),
			Color: "#F55036", Website: "https://groq.com", APIKeyURL: "https://console.groq.com/keys",
			InputPerM: 0.59, OutputPerM: 0.79},
		{ID: "xai", DisplayName: "xAI (Grok)", Alias: "xai", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.x.ai/v1", AuthKind: "api_key", AuthModes: []string{"oauth", "api_key"},
			ServiceKinds: llm(core.ServiceSearch, core.ServiceImage), Color: "#1DA1F2", Website: "https://x.ai",
			APIKeyURL: "https://console.x.ai"},
		{ID: "mistral", DisplayName: "Mistral", Alias: "mistral", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.mistral.ai/v1", AuthKind: "api_key", ServiceKinds: llm(core.ServiceEmbedding),
			Color: "#FF7000", Website: "https://mistral.ai", APIKeyURL: "https://console.mistral.ai/api-keys"},
		{ID: "perplexity", DisplayName: "Perplexity", Alias: "pplx", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.perplexity.ai", AuthKind: "api_key", ServiceKinds: llm(core.ServiceSearch),
			Color: "#20808D", Website: "https://www.perplexity.ai", APIKeyURL: "https://www.perplexity.ai/settings/api"},
		{ID: "together", DisplayName: "Together AI", Alias: "together", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.together.xyz/v1", AuthKind: "api_key", ServiceKinds: llm(core.ServiceEmbedding),
			Color: "#0F6FFF", Website: "https://www.together.ai", APIKeyURL: "https://api.together.xyz/settings/api-keys"},
		{ID: "fireworks", DisplayName: "Fireworks AI", Alias: "fireworks", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.fireworks.ai/inference/v1", AuthKind: "api_key", ServiceKinds: llm(core.ServiceEmbedding),
			Color: "#7B2EF2", Website: "https://fireworks.ai", APIKeyURL: "https://fireworks.ai/account/api-keys"},
		{ID: "cerebras", DisplayName: "Cerebras", Alias: "cerebras", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.cerebras.ai/v1", AuthKind: "api_key", ServiceKinds: llm(),
			Color: "#FF4F00", Website: "https://www.cerebras.ai", APIKeyURL: "https://cloud.cerebras.ai/platform"},
		{ID: "cohere", DisplayName: "Cohere", Alias: "cohere", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.cohere.ai/v1", AuthKind: "api_key", ServiceKinds: llm(),
			Color: "#39594D", Website: "https://cohere.com", APIKeyURL: "https://dashboard.cohere.com/api-keys"},
		{ID: "nebius", DisplayName: "Nebius AI", Alias: "nebius", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.studio.nebius.ai/v1", AuthKind: "api_key", ServiceKinds: llm(core.ServiceEmbedding),
			Color: "#6C5CE7", Website: "https://nebius.com", APIKeyURL: "https://studio.nebius.com/settings/api-keys"},
		{ID: "siliconflow", DisplayName: "SiliconFlow", Alias: "siliconflow", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.siliconflow.cn/v1", AuthKind: "api_key", ServiceKinds: llm(),
			Color: "#5B6EF5", Website: "https://cloud.siliconflow.com", APIKeyURL: "https://cloud.siliconflow.com/account/ak"},
		{ID: "hyperbolic", DisplayName: "Hyperbolic", Alias: "hyp", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.hyperbolic.xyz/v1", AuthKind: "api_key", ServiceKinds: llm(core.ServiceTTS),
			Color: "#00D4FF", Website: "https://hyperbolic.xyz", APIKeyURL: "https://app.hyperbolic.xyz/settings"},
		{ID: "blackbox", DisplayName: "Blackbox AI", Alias: "bb", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.blackbox.ai", AuthKind: "api_key", ServiceKinds: llm(),
			Color: "#5B5FEF", Website: "https://blackbox.ai", APIKeyURL: "https://www.blackbox.ai/api-management"},
		{ID: "chutes", DisplayName: "Chutes AI", Alias: "ch", Dialect: core.DialectOpenAI,
			BaseURL: "https://llm.chutes.ai/v1", AuthKind: "api_key", ServiceKinds: llm(),
			Color: "#111111", Website: "https://chutes.ai", APIKeyURL: "https://chutes.ai/app/api"},
		{ID: "opencode-go", DisplayName: "OpenCode Go", Alias: "ocg", Dialect: core.DialectOpenAI,
			BaseURL: "https://opencode.ai/zen/go/v1", AuthKind: "api_key", ServiceKinds: llm(),
			Color: "#E87040", Website: "https://opencode.ai/auth"},
		// Generic compatible endpoints configured entirely via the account base URL.
		{ID: "custom-openai", DisplayName: "Custom (OpenAI-compatible)", Alias: "custom-openai", Dialect: core.DialectOpenAI,
			BaseURL: "", AuthKind: "api_key", ServiceKinds: llm()},
		{ID: "custom-anthropic", DisplayName: "Custom (Anthropic-compatible)", Alias: "custom-anthropic", Dialect: core.DialectAnthropic,
			BaseURL: "", AuthKind: "api_key", ServiceKinds: llm()},
	}
}

// mediaProviders serve non-LLM kinds: embeddings, image, STT, TTS, web search,
// and web fetch. They are driven by the dedicated media connectors.
func mediaProviders() []ProviderSpec {
	return []ProviderSpec{
		// Speech-to-text.
		{ID: "deepgram", DisplayName: "Deepgram", Alias: "dg", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.deepgram.com/v1", AuthKind: "api_key", ServiceKinds: []core.ServiceKind{core.ServiceSTT, core.ServiceTTS},
			Color: "#13EF93", Website: "https://deepgram.com", APIKeyURL: "https://console.deepgram.com/api-keys"},
		{ID: "assemblyai", DisplayName: "AssemblyAI", Alias: "aai", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.assemblyai.com/v2", AuthKind: "api_key", ServiceKinds: []core.ServiceKind{core.ServiceSTT},
			Color: "#0062FF", Website: "https://assemblyai.com", APIKeyURL: "https://www.assemblyai.com/app/api-keys"},
		// Text-to-speech.
		{ID: "elevenlabs", DisplayName: "ElevenLabs", Alias: "el", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.elevenlabs.io/v1", AuthKind: "api_key", ServiceKinds: []core.ServiceKind{core.ServiceTTS},
			Color: "#6C47FF", Website: "https://elevenlabs.io", APIKeyURL: "https://elevenlabs.io/app/settings/api-keys"},
		{ID: "inworld", DisplayName: "Inworld TTS", Alias: "inworld", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.inworld.ai/tts/v1", AuthKind: "api_key", ServiceKinds: []core.ServiceKind{core.ServiceTTS},
			Color: "#FF6B6B", Website: "https://inworld.ai", APIKeyURL: "https://platform.inworld.ai/api-keys"},
		// Image generation.
		{ID: "nanobanana", DisplayName: "NanoBanana API", Alias: "nb", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.nanobananaapi.ai/v1", AuthKind: "api_key", ServiceKinds: []core.ServiceKind{core.ServiceImage},
			Color: "#FFD700", Website: "https://nanobananaapi.ai", APIKeyURL: "https://nanobananaapi.ai/dashboard"},
		{ID: "fal-ai", DisplayName: "Fal.ai", Alias: "fal", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.fal.ai/v1", AuthKind: "api_key", ServiceKinds: []core.ServiceKind{core.ServiceImage},
			Color: "#2563EB", Website: "https://fal.ai", APIKeyURL: "https://fal.ai/dashboard/keys"},
		{ID: "stability-ai", DisplayName: "Stability AI", Alias: "stability", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.stability.ai", AuthKind: "api_key", ServiceKinds: []core.ServiceKind{core.ServiceImage},
			Color: "#8B5CF6", Website: "https://stability.ai", APIKeyURL: "https://platform.stability.ai/account/keys"},
		{ID: "black-forest-labs", DisplayName: "Black Forest Labs", Alias: "bfl", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.bfl.ai/v1", AuthKind: "api_key", ServiceKinds: []core.ServiceKind{core.ServiceImage},
			Color: "#111827", Website: "https://blackforestlabs.ai"},
		// Embeddings.
		{ID: "voyage-ai", DisplayName: "Voyage AI", Alias: "voyage", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.voyageai.com/v1", AuthKind: "api_key", ServiceKinds: []core.ServiceKind{core.ServiceEmbedding},
			Color: "#0EA5E9", Website: "https://www.voyageai.com", APIKeyURL: "https://dash.voyageai.com/api-keys"},
		{ID: "jina-ai", DisplayName: "Jina AI", Alias: "jina", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.jina.ai/v1", AuthKind: "api_key", ServiceKinds: []core.ServiceKind{core.ServiceEmbedding},
			Color: "#2563EB", Website: "https://jina.ai", APIKeyURL: "https://jina.ai/?sui=apikey"},
		// Web search.
		{ID: "tavily", DisplayName: "Tavily", Alias: "tavily", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.tavily.com", AuthKind: "api_key", ServiceKinds: []core.ServiceKind{core.ServiceSearch, core.ServiceFetch},
			Color: "#5B21B6", Website: "https://tavily.com", APIKeyURL: "https://app.tavily.com/home"},
		{ID: "brave-search", DisplayName: "Brave Search", Alias: "brave", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.search.brave.com/res/v1", AuthKind: "api_key", ServiceKinds: []core.ServiceKind{core.ServiceSearch},
			Color: "#FB542B", Website: "https://brave.com/search/api", APIKeyURL: "https://api-dashboard.search.brave.com/app/keys"},
		{ID: "serper", DisplayName: "Serper", Alias: "serper", Dialect: core.DialectOpenAI,
			BaseURL: "https://google.serper.dev", AuthKind: "api_key", ServiceKinds: []core.ServiceKind{core.ServiceSearch},
			Color: "#4F46E5", Website: "https://serper.dev", APIKeyURL: "https://serper.dev/api-key"},
		{ID: "exa", DisplayName: "Exa", Alias: "exa", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.exa.ai", AuthKind: "api_key", ServiceKinds: []core.ServiceKind{core.ServiceSearch, core.ServiceFetch},
			Color: "#2563EB", Website: "https://exa.ai", APIKeyURL: "https://dashboard.exa.ai/api-keys"},
		{ID: "searxng", DisplayName: "SearXNG", Alias: "searxng", Dialect: core.DialectOpenAI,
			BaseURL: "http://localhost:8888", AuthKind: "none", AuthModes: []string{"none"}, ServiceKinds: []core.ServiceKind{core.ServiceSearch},
			Color: "#3B82F6", Website: "https://docs.searxng.org"},
		// Web fetch.
		{ID: "firecrawl", DisplayName: "Firecrawl", Alias: "firecrawl", Dialect: core.DialectOpenAI,
			BaseURL: "https://api.firecrawl.dev/v1", AuthKind: "api_key", ServiceKinds: []core.ServiceKind{core.ServiceFetch},
			Color: "#F59E0B", Website: "https://firecrawl.dev", APIKeyURL: "https://www.firecrawl.dev/app/api-keys"},
		{ID: "jina-reader", DisplayName: "Jina Reader", Alias: "jina-reader", Dialect: core.DialectOpenAI,
			BaseURL: "https://r.jina.ai", AuthKind: "api_key", ServiceKinds: []core.ServiceKind{core.ServiceFetch},
			Color: "#111111", Website: "https://jina.ai/reader", APIKeyURL: "https://jina.ai/?sui=apikey"},
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

// SpecByAlias resolves a provider by its short alias or id.
func SpecByAlias(aliasOrID string) (ProviderSpec, bool) {
	for _, p := range Catalog() {
		if p.ID == aliasOrID || p.Alias == aliasOrID {
			return p, true
		}
	}
	return ProviderSpec{}, false
}

// SpecsByKind returns the catalog specs that serve a given service kind,
// excluding hidden providers.
func SpecsByKind(kind core.ServiceKind) []ProviderSpec {
	var out []ProviderSpec
	for _, p := range Catalog() {
		if p.Hidden {
			continue
		}
		if core.HasServiceKind(p.ServiceKinds, kind) {
			out = append(out, p)
		}
	}
	return out
}

// AuthModesOf returns the auth modes for a spec, defaulting to [AuthKind].
func (p ProviderSpec) AuthModesOf() []string {
	if len(p.AuthModes) > 0 {
		return p.AuthModes
	}
	if p.AuthKind != "" {
		return []string{p.AuthKind}
	}
	return []string{"api_key"}
}