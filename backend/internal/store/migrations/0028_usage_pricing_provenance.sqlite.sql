-- Forward-compatible pricing provenance for databases that already applied an
-- earlier 0027 before these columns existed. Duplicate ADD COLUMN statements
-- are intentionally tolerated by the migration runner for fresh databases.
ALTER TABLE usage_records ADD COLUMN pricing_match_kind TEXT NOT NULL DEFAULT '';
ALTER TABLE usage_records ADD COLUMN pricing_source_url TEXT NOT NULL DEFAULT '';
ALTER TABLE usage_records ADD COLUMN pricing_as_of TEXT;
ALTER TABLE usage_records ADD COLUMN pricing_backfilled INTEGER NOT NULL DEFAULT 0;

-- Historical repricing performed by the earlier implementation can be
-- recognized by its retained key/rates/components. Mark it as a backfilled
-- estimate so analytics keep the recovered cost while hard budgets exclude it.
UPDATE usage_records
SET pricing_backfilled = 1,
    pricing_status = CASE WHEN pricing_status = 'free' THEN 'free' ELSE 'estimated' END,
    pricing_match_kind = 'legacy',
    pricing_as_of = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE request_id = ''
  AND prompt_tokens + completion_tokens > 0
  AND pricing_status IN ('priced', 'estimated', 'free')
  AND (
      pricing_key <> ''
      OR input_cost_nanos + cached_cost_nanos + cache_write_cost_nanos + output_cost_nanos + reasoning_cost_nanos > 0
      OR input_rate_per_m <> 0 OR cached_rate_per_m <> 0 OR cache_write_rate_per_m <> 0
      OR output_rate_per_m <> 0 OR reasoning_rate_per_m <> 0
  );

-- Rows migrated from the pre-accounting schema have no request id. Preserve
-- that uncertainty instead of calling their usage provider-measured.
UPDATE usage_records
SET usage_source = CASE
        WHEN cache_hit = 1 THEN 'cache'
        WHEN prompt_tokens + completion_tokens > 0 THEN 'legacy'
        ELSE 'none'
    END
WHERE request_id = '';

-- Retain positive legacy totals without inventing components. Cache rows are
-- legacy avoided-cost facts, while unknown zero-cost token rows stay missing.
UPDATE usage_records
SET pricing_status = CASE
        WHEN prompt_tokens + completion_tokens = 0 THEN 'none'
        WHEN cost_nanos > 0 OR cost_micros > 0 OR cache_hit = 1 THEN 'legacy'
        ELSE 'missing'
    END,
    pricing_source = CASE WHEN prompt_tokens + completion_tokens = 0 THEN 'none' ELSE 'legacy' END,
    pricing_key = '',
    pricing_match_kind = CASE
        WHEN prompt_tokens + completion_tokens = 0 THEN 'none'
        WHEN cost_nanos > 0 OR cost_micros > 0 OR cache_hit = 1 THEN 'legacy'
        ELSE 'none'
    END
WHERE request_id = '' AND pricing_backfilled = 0;

-- Requests recorded by an intermediate accounting build already captured
-- rates at request time but lacked the new provenance columns.
UPDATE usage_records
SET pricing_match_kind = CASE
        WHEN pricing_status IN ('priced', 'estimated', 'free') THEN 'legacy'
        ELSE 'none'
    END,
    pricing_as_of = CASE
        WHEN pricing_status IN ('priced', 'estimated', 'free') THEN created_at
        ELSE pricing_as_of
    END
WHERE request_id <> '' AND pricing_match_kind = '';
