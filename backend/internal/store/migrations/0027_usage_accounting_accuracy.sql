-- Canonical request accounting and immutable pricing snapshots.
-- cost_nanos is authoritative so small per-request charges do not round to zero.
ALTER TABLE usage_records ADD COLUMN request_id TEXT NOT NULL DEFAULT '';
ALTER TABLE usage_records ADD COLUMN status TEXT NOT NULL DEFAULT 'success';
ALTER TABLE usage_records ADD COLUMN error_kind TEXT NOT NULL DEFAULT '';
ALTER TABLE usage_records ADD COLUMN usage_source TEXT NOT NULL DEFAULT 'provider';
ALTER TABLE usage_records ADD COLUMN reasoning_tokens BIGINT NOT NULL DEFAULT 0;

ALTER TABLE usage_records ADD COLUMN cost_nanos BIGINT NOT NULL DEFAULT 0;
ALTER TABLE usage_records ADD COLUMN input_cost_nanos BIGINT NOT NULL DEFAULT 0;
ALTER TABLE usage_records ADD COLUMN cached_cost_nanos BIGINT NOT NULL DEFAULT 0;
ALTER TABLE usage_records ADD COLUMN cache_write_cost_nanos BIGINT NOT NULL DEFAULT 0;
ALTER TABLE usage_records ADD COLUMN output_cost_nanos BIGINT NOT NULL DEFAULT 0;
ALTER TABLE usage_records ADD COLUMN reasoning_cost_nanos BIGINT NOT NULL DEFAULT 0;
ALTER TABLE usage_records ADD COLUMN avoided_cost_nanos BIGINT NOT NULL DEFAULT 0;
ALTER TABLE usage_records ADD COLUMN saved_cost_nanos BIGINT NOT NULL DEFAULT 0;

ALTER TABLE usage_records ADD COLUMN pricing_status TEXT NOT NULL DEFAULT 'legacy';
ALTER TABLE usage_records ADD COLUMN pricing_source TEXT NOT NULL DEFAULT 'legacy';
ALTER TABLE usage_records ADD COLUMN pricing_key TEXT NOT NULL DEFAULT '';
ALTER TABLE usage_records ADD COLUMN pricing_match_kind TEXT NOT NULL DEFAULT '';
ALTER TABLE usage_records ADD COLUMN pricing_source_url TEXT NOT NULL DEFAULT '';
ALTER TABLE usage_records ADD COLUMN pricing_as_of TEXT;
ALTER TABLE usage_records ADD COLUMN pricing_backfilled INTEGER NOT NULL DEFAULT 0;
ALTER TABLE usage_records ADD COLUMN input_rate_per_m DOUBLE PRECISION NOT NULL DEFAULT 0;
ALTER TABLE usage_records ADD COLUMN cached_rate_per_m DOUBLE PRECISION NOT NULL DEFAULT 0;
ALTER TABLE usage_records ADD COLUMN cache_write_rate_per_m DOUBLE PRECISION NOT NULL DEFAULT 0;
ALTER TABLE usage_records ADD COLUMN output_rate_per_m DOUBLE PRECISION NOT NULL DEFAULT 0;
ALTER TABLE usage_records ADD COLUMN reasoning_rate_per_m DOUBLE PRECISION NOT NULL DEFAULT 0;

ALTER TABLE usage_records ADD COLUMN upstream_latency_ms INTEGER NOT NULL DEFAULT 0;
ALTER TABLE usage_records ADD COLUMN end_to_end_latency_ms INTEGER NOT NULL DEFAULT 0;

UPDATE usage_records
SET cost_nanos = cost_micros * 1000,
    upstream_latency_ms = latency_ms,
    end_to_end_latency_ms = latency_ms,
    status = CASE WHEN cache_hit = 1 THEN 'cache_hit' ELSE 'success' END,
    usage_source = CASE
        WHEN cache_hit = 1 THEN 'cache'
        WHEN prompt_tokens + completion_tokens > 0 THEN 'legacy'
        ELSE 'none'
    END,
    pricing_status = CASE
        WHEN prompt_tokens + completion_tokens = 0 THEN 'none'
        WHEN cost_micros > 0 OR cache_hit = 1 THEN 'legacy'
        ELSE 'missing'
    END,
    pricing_match_kind = CASE WHEN prompt_tokens + completion_tokens > 0 THEN 'legacy' ELSE 'none' END;

CREATE INDEX IF NOT EXISTS idx_usage_tenant_time_status
    ON usage_records(tenant_id, created_at, status);
CREATE INDEX IF NOT EXISTS idx_usage_tenant_time_pricing
    ON usage_records(tenant_id, created_at, pricing_status);
CREATE INDEX IF NOT EXISTS idx_usage_request_id
    ON usage_records(request_id);
