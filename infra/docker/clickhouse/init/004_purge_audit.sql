-- Tabela de auditoria de purge permanente — SEM TTL (imutável e permanente).
-- Registros de purge nunca podem ser deletados — são prova de auditoria.
-- Ref: SRS Req-2.17.6, TASK 0.4.7.
CREATE TABLE IF NOT EXISTS cascata_logs.purge_audit (
    timestamp             DateTime64(3),
    tenant_id             String,
    executor_id           String,
    executor_type         LowCardinality(String),  -- 'member', 'agent', 'system_auto'
    table_name            String,
    original_schema       String,
    row_count_destroyed   UInt64,
    confirmation_method   LowCardinality(String),  -- 'password', 'passkey', 'auto_retention'
    ip_address            String,
    purge_type            LowCardinality(String)   -- 'manual', 'auto_retention'
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (tenant_id, timestamp)
-- SEM TTL — registros de purge são imutáveis e permanentes
SETTINGS storage_policy = 'default';
