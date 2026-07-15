package connectors

// currentOfficialModelPrices contains rates verified against vendor pricing
// pages in July 2026. These entries intentionally come last in the catalog so
// they override compatibility values retained in model_prices.go.
func currentOfficialModelPrices() []ModelPrice {
	const (
		openAIURL     = "https://platform.openai.com/docs/pricing/"
		cloudflareURL = "https://developers.cloudflare.com/workers-ai/platform/pricing/"
		qwenURL       = "https://modelstudio.alibabacloud.com/"
		geminiURL     = "https://ai.google.dev/gemini-api/docs/pricing"
	)
	prices := []ModelPrice{
		// OpenAI standard short-context rates. The resolver switches to the
		// long-context snapshot when prompt tokens exceed 272K.
		{Provider: "openai", Model: "gpt-5.6-sol", InputPerM: 5, CachedInputPerM: .5, CacheWritePerM: 6.25, OutputPerM: 30, LongContextThreshold: 272000, LongInputPerM: 10, LongCachedInputPerM: 1, LongCacheWritePerM: 12.5, LongOutputPerM: 45, Source: "official", SourceURL: openAIURL},
		{Provider: "openai", Model: "gpt-5.6-terra", InputPerM: 2.5, CachedInputPerM: .25, CacheWritePerM: 3.125, OutputPerM: 15, LongContextThreshold: 272000, LongInputPerM: 5, LongCachedInputPerM: .5, LongCacheWritePerM: 6.25, LongOutputPerM: 22.5, Source: "official", SourceURL: openAIURL},
		{Provider: "openai", Model: "gpt-5.6-luna", InputPerM: 1, CachedInputPerM: .1, CacheWritePerM: 1.25, OutputPerM: 6, LongContextThreshold: 272000, LongInputPerM: 2, LongCachedInputPerM: .2, LongCacheWritePerM: 2.5, LongOutputPerM: 9, Source: "official", SourceURL: openAIURL},
		{Provider: "openai", Model: "gpt-5.5", InputPerM: 5, CachedInputPerM: .5, CacheWritePerM: 5, OutputPerM: 30, LongContextThreshold: 272000, LongInputPerM: 10, LongCachedInputPerM: 1, LongCacheWritePerM: 10, LongOutputPerM: 45, Source: "official", SourceURL: openAIURL},
		{Provider: "openai", Model: "gpt-5.4", InputPerM: 2.5, CachedInputPerM: .25, CacheWritePerM: 2.5, OutputPerM: 15, LongContextThreshold: 272000, LongInputPerM: 5, LongCachedInputPerM: .5, LongCacheWritePerM: 5, LongOutputPerM: 22.5, Source: "official", SourceURL: openAIURL},
		{Provider: "openai", Model: "gpt-5.4-mini", InputPerM: .75, CachedInputPerM: .075, CacheWritePerM: .75, OutputPerM: 4.5, Source: "official", SourceURL: openAIURL},
		{Provider: "openai", Model: "gpt-5.4-nano", InputPerM: .2, CachedInputPerM: .02, CacheWritePerM: .2, OutputPerM: 1.25, Source: "official", SourceURL: openAIURL},
		{Provider: "openai", Model: "gpt-5.3-codex", InputPerM: 1.75, CachedInputPerM: .175, CacheWritePerM: 1.75, OutputPerM: 14, Source: "official", SourceURL: openAIURL},

		// Alibaba retail list price. Qoder promotions and subscription credits
		// are not token prices and are therefore not applied here.
		{Provider: "qwen", Model: "qwen3.7-max", InputPerM: 2.5, OutputPerM: 7.5, CachedInputPerM: .25, CacheWritePerM: 2.5, Source: "official", SourceURL: qwenURL},
		{Provider: "qwen", Model: "qwen-3.7-max", InputPerM: 2.5, OutputPerM: 7.5, CachedInputPerM: .25, CacheWritePerM: 2.5, Source: "official", SourceURL: qwenURL},
		// Plus is tiered by context on the vendor page. The lower standard tier
		// is retained as a retail-equivalent estimate until the request captures
		// the vendor's exact tier metadata.
		{Provider: "qwen", Model: "qwen3.7-plus", InputPerM: .4, OutputPerM: 1.6, CachedInputPerM: .04, CacheWritePerM: .4, Source: "official_range", SourceURL: qwenURL, Estimated: true},
		{Provider: "qwen", Model: "qwen-3.7-plus", InputPerM: .4, OutputPerM: 1.6, CachedInputPerM: .04, CacheWritePerM: .4, Source: "official_range", SourceURL: qwenURL, Estimated: true},

		// Google bills thinking as output; ReasoningPerM therefore equals output
		// and Meter subtracts the reasoning subset before pricing normal output.
		{Provider: "gemini", Model: "gemini-2.5-pro", InputPerM: 1.25, OutputPerM: 10, CachedInputPerM: .125, CacheWritePerM: 1.25, ReasoningPerM: 10, LongContextThreshold: 200000, LongInputPerM: 2.5, LongCachedInputPerM: .25, LongCacheWritePerM: 2.5, LongOutputPerM: 15, Source: "official", SourceURL: geminiURL},
		{Provider: "gemini", Model: "gemini-2.5-flash", InputPerM: .3, OutputPerM: 2.5, CachedInputPerM: .03, CacheWritePerM: .3, ReasoningPerM: 2.5, Source: "official", SourceURL: geminiURL},
		{Provider: "gemini", Model: "gemini-2.5-flash-lite", InputPerM: .1, OutputPerM: .4, CachedInputPerM: .01, CacheWritePerM: .1, ReasoningPerM: .4, Source: "official", SourceURL: geminiURL},
		{Provider: "gemini", Model: "gemini-3-flash-preview", InputPerM: .5, OutputPerM: 3, CachedInputPerM: .05, CacheWritePerM: .5, ReasoningPerM: 3, Source: "official", SourceURL: geminiURL},
		{Provider: "gemini", Model: "gemini-3.1-pro-preview", InputPerM: 2, OutputPerM: 12, CachedInputPerM: .2, CacheWritePerM: 2, ReasoningPerM: 12, LongContextThreshold: 200000, LongInputPerM: 4, LongCachedInputPerM: .4, LongCacheWritePerM: 4, LongOutputPerM: 18, Source: "official", SourceURL: geminiURL},

		// Cloudflare Workers AI token rates.
		{Provider: "cloudflare-ai", Model: "@cf/zai-org/glm-5.2", InputPerM: 1.4, CachedInputPerM: .26, CacheWritePerM: 1.4, OutputPerM: 4.4, ReasoningPerM: 4.4, Source: "official", SourceURL: cloudflareURL},
		{Provider: "cloudflare-ai", Model: "@cf/zai-org/glm-4.7-flash", InputPerM: .06, OutputPerM: .4, Source: "official", SourceURL: cloudflareURL},
		{Provider: "cloudflare-ai", Model: "@cf/qwen/qwq-32b", InputPerM: .66, OutputPerM: 1, Source: "official", SourceURL: cloudflareURL},
		{Provider: "cloudflare-ai", Model: "@cf/qwen/qwen2.5-coder-32b-instruct", InputPerM: .66, OutputPerM: 1, Source: "official", SourceURL: cloudflareURL},
		{Provider: "cloudflare-ai", Model: "@cf/qwen/qwen3-30b-a3b-fp8", InputPerM: .051, OutputPerM: .335, Source: "official", SourceURL: cloudflareURL},
		{Provider: "cloudflare-ai", Model: "@cf/openai/gpt-oss-120b", InputPerM: .35, OutputPerM: .75, Source: "official", SourceURL: cloudflareURL},
		{Provider: "cloudflare-ai", Model: "@cf/openai/gpt-oss-20b", InputPerM: .2, OutputPerM: .3, Source: "official", SourceURL: cloudflareURL},
		{Provider: "cloudflare-ai", Model: "@cf/moonshotai/kimi-k2.5", InputPerM: .6, CachedInputPerM: .1, OutputPerM: 3, Source: "official", SourceURL: cloudflareURL},
		{Provider: "cloudflare-ai", Model: "@cf/moonshotai/kimi-k2.6", InputPerM: .95, CachedInputPerM: .16, OutputPerM: 4, Source: "official", SourceURL: cloudflareURL},
		{Provider: "cloudflare-ai", Model: "@cf/deepseek-ai/deepseek-r1-distill-qwen-32b", InputPerM: .497, OutputPerM: 4.881, Source: "official", SourceURL: cloudflareURL},
		{Provider: "cloudflare-ai", Model: "@cf/meta/llama-3.3-70b-instruct-fp8-fast", InputPerM: .293, OutputPerM: 2.253, Source: "official", SourceURL: cloudflareURL},
		{Provider: "cloudflare-ai", Model: "@cf/meta/llama-3.2-1b-instruct", InputPerM: .027, OutputPerM: .201, Source: "official", SourceURL: cloudflareURL},
		{Provider: "cloudflare-ai", Model: "@cf/meta/llama-3.2-3b-instruct", InputPerM: .051, OutputPerM: .335, Source: "official", SourceURL: cloudflareURL},
	}

	// Kiro is subscription/credit based. These entries are explicitly marked
	// retail-equivalent rather than actual spend.
	for _, model := range []string{"gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna"} {
		for _, p := range prices {
			if p.Provider != "openai" || p.Model != model {
				continue
			}
			p.Provider = "kiro"
			p.Source = "retail_equivalent"
			p.Estimated = true
			prices = append(prices, p)
			for _, suffix := range []string{"-thinking", "-agentic", "-thinking-agentic"} {
				variant := p
				variant.Model += suffix
				prices = append(prices, variant)
			}
			break
		}
	}
	return prices
}
