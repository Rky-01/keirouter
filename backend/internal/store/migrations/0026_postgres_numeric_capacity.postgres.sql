-- PostgreSQL uses 32-bit INTEGER while SQLite INTEGER stores signed 64-bit
-- values. Promote counters, token/cost fields, and resource byte measurements
-- whose Go models are int64 or whose normal values can exceed 2 GiB.

ALTER TABLE resource_samples
    ALTER COLUMN id TYPE BIGINT,
    ALTER COLUMN heap_alloc_bytes TYPE BIGINT,
    ALTER COLUMN heap_sys_bytes TYPE BIGINT,
    ALTER COLUMN gc_pause_ns TYPE BIGINT,
    ALTER COLUMN next_gc_bytes TYPE BIGINT,
    ALTER COLUMN num_gc TYPE BIGINT,
    ALTER COLUMN proc_rss_bytes TYPE BIGINT,
    ALTER COLUMN host_mem_used_bytes TYPE BIGINT,
    ALTER COLUMN host_mem_total_bytes TYPE BIGINT,
    ALTER COLUMN host_disk_used_bytes TYPE BIGINT,
    ALTER COLUMN host_disk_total_bytes TYPE BIGINT,
    ALTER COLUMN host_net_sent_bytes TYPE BIGINT,
    ALTER COLUMN host_net_recv_bytes TYPE BIGINT,
    ALTER COLUMN proc_cpu_percent TYPE DOUBLE PRECISION,
    ALTER COLUMN host_cpu_percent TYPE DOUBLE PRECISION,
    ALTER COLUMN host_load1 TYPE DOUBLE PRECISION,
    ALTER COLUMN host_load5 TYPE DOUBLE PRECISION,
    ALTER COLUMN host_load15 TYPE DOUBLE PRECISION;

CREATE SEQUENCE IF NOT EXISTS resource_samples_id_seq;
SELECT setval(
    'resource_samples_id_seq',
    COALESCE((SELECT MAX(id) FROM resource_samples), 0) + 1,
    false
);
ALTER SEQUENCE resource_samples_id_seq OWNED BY resource_samples.id;
ALTER TABLE resource_samples
    ALTER COLUMN id SET DEFAULT nextval('resource_samples_id_seq');

ALTER TABLE usage_records
    ALTER COLUMN prompt_tokens TYPE BIGINT,
    ALTER COLUMN completion_tokens TYPE BIGINT,
    ALTER COLUMN cached_tokens TYPE BIGINT,
    ALTER COLUMN cache_write_tokens TYPE BIGINT,
    ALTER COLUMN cost_micros TYPE BIGINT,
    ALTER COLUMN slim_bytes_saved TYPE BIGINT,
    ALTER COLUMN slim_tokens_saved TYPE BIGINT,
    ALTER COLUMN headroom_tokens_saved TYPE BIGINT,
    ALTER COLUMN headroom_bytes_saved TYPE BIGINT;

ALTER TABLE budgets
    ALTER COLUMN limit_micros TYPE BIGINT,
    ALTER COLUMN limit_tokens TYPE BIGINT;

ALTER TABLE plans
    ALTER COLUMN limit_micros TYPE BIGINT,
    ALTER COLUMN limit_tokens TYPE BIGINT,
    ALTER COLUMN rpm_limit TYPE BIGINT,
    ALTER COLUMN tpm_limit TYPE BIGINT,
    ALTER COLUMN concurrency_limit TYPE BIGINT;

ALTER TABLE custom_models
    ALTER COLUMN input_per_m TYPE DOUBLE PRECISION,
    ALTER COLUMN output_per_m TYPE DOUBLE PRECISION;

ALTER TABLE provider_health_current
    ALTER COLUMN success_rate TYPE DOUBLE PRECISION,
    ALTER COLUMN error_rate TYPE DOUBLE PRECISION,
    ALTER COLUMN request_count TYPE BIGINT,
    ALTER COLUMN fallback_count TYPE BIGINT;

ALTER TABLE provider_health_snapshots
    ALTER COLUMN request_count TYPE BIGINT,
    ALTER COLUMN success_count TYPE BIGINT,
    ALTER COLUMN failure_count TYPE BIGINT,
    ALTER COLUMN fallback_count TYPE BIGINT,
    ALTER COLUMN final_failure_count TYPE BIGINT,
    ALTER COLUMN input_tokens TYPE BIGINT,
    ALTER COLUMN output_tokens TYPE BIGINT,
    ALTER COLUMN estimated_cost_microusd TYPE BIGINT,
    ALTER COLUMN rate_limited_count TYPE BIGINT,
    ALTER COLUMN auth_error_count TYPE BIGINT,
    ALTER COLUMN quota_exceeded_count TYPE BIGINT,
    ALTER COLUMN timeout_count TYPE BIGINT,
    ALTER COLUMN provider_5xx_count TYPE BIGINT,
    ALTER COLUMN bad_request_count TYPE BIGINT,
    ALTER COLUMN network_error_count TYPE BIGINT,
    ALTER COLUMN unsupported_count TYPE BIGINT,
    ALTER COLUMN unknown_error_count TYPE BIGINT;

ALTER TABLE provider_probe_results
    ALTER COLUMN prompt_tokens TYPE BIGINT,
    ALTER COLUMN completion_tokens TYPE BIGINT,
    ALTER COLUMN estimated_cost_microusd TYPE BIGINT;
