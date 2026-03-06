-- Eventos de pool (timeout, circuit breaker, backpressure)
CREATE TABLE IF NOT EXISTS cascata_logs.pool_events (
    timestamp         DateTime64(3),
    tenant_id         String,
    tier              LowCardinality(String),
    event_type        LowCardinality(String),
    -- 'statement_timeout', 'idle_tx_timeout', 'queue_pressure_moderate',
    -- 'queue_pressure_critical', 'circuit_breaker_open', 'circuit_breaker_close',
    -- 'pool_resize', 'pool_warmup', 'connection_error'
    pool_id           String,
    query_hash        Nullable(String),
    duration_ms       Nullable(UInt32),
    queue_depth       Nullable(UInt16),
    queue_capacity    Nullable(UInt16),
    details           String  -- JSON com contexto adicional
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (tenant_id, timestamp)
TTL timestamp + INTERVAL 90 DAY;
