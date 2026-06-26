-- Dynamic custom provider instances and per-provider custom models.
--
-- custom_providers stores user-defined provider instances built on top of the
-- OpenAI-compatible or Anthropic-compatible dialects. Each instance gets a
-- unique provider id (e.g. "custom-openai-myvllm") so multiple endpoints of the
-- same base type stay fully isolated: their own base URL, accounts, and models.
CREATE TABLE IF NOT EXISTS custom_providers (
    id           TEXT PRIMARY KEY,
    tenant_id    TEXT NOT NULL,
    display_name TEXT NOT NULL,
    alias        TEXT NOT NULL DEFAULT '',
    dialect      TEXT NOT NULL,
    base_url     TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_custom_providers_tenant
    ON custom_providers(tenant_id);

-- custom_models stores user-registered models. provider_id may reference either
-- a custom provider instance OR a built-in catalog provider, so users can add
-- their own models beyond the predefined set on any provider.
CREATE TABLE IF NOT EXISTS custom_models (
    id             TEXT PRIMARY KEY,
    tenant_id      TEXT NOT NULL,
    provider_id    TEXT NOT NULL,
    model_id       TEXT NOT NULL,
    display_name   TEXT NOT NULL DEFAULT '',
    kind           TEXT NOT NULL DEFAULT 'llm',
    context_window INTEGER NOT NULL DEFAULT 0,
    input_per_m    REAL NOT NULL DEFAULT 0,
    output_per_m   REAL NOT NULL DEFAULT 0,
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL
);

-- A model id is unique within a provider instance.
CREATE UNIQUE INDEX IF NOT EXISTS idx_custom_models_provider_model
    ON custom_models(provider_id, model_id);

CREATE INDEX IF NOT EXISTS idx_custom_models_provider
    ON custom_models(provider_id);
