-- =============================================================================
-- ClickHouse — Schema de Inicialização do Cascata
-- =============================================================================
-- Executado automaticamente no startup do ClickHouse.
-- Cria o database e as tabelas de observabilidade.
-- Ref: SAD §C (Observabilidade), TASK PR-7.
-- =============================================================================

-- Database de observabilidade
CREATE DATABASE IF NOT EXISTS cascata_logs;

-- =============================================================================
-- SYSTEM LOGS — logs de todos os serviços do Cascata
-- Recebe dados do Vector via HTTP insert
-- =============================================================================
CREATE TABLE IF NOT EXISTS cascata_logs.system_logs (
    timestamp     DateTime64(3),
    service       LowCardinality(String),  -- 'control-plane', 'gateway', 'pgcat', etc.
    level         LowCardinality(String),  -- 'debug','info','warn','error','fatal'
    trace_id      String DEFAULT '',
    span_id       String DEFAULT '',
    tenant_id     Nullable(String),
    msg           String,
    attributes    String DEFAULT '{}'       -- JSON com atributos adicionais
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (service, timestamp)
TTL timestamp + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;

-- =============================================================================
-- SYSTEM TRACES — traces distribuídos (OTel format)
-- Para rastreamento de requests end-to-end
-- =============================================================================
CREATE TABLE IF NOT EXISTS cascata_logs.system_traces (
    timestamp       DateTime64(3),
    trace_id        String,
    span_id         String,
    parent_span_id  String DEFAULT '',
    service         LowCardinality(String),
    operation       String,                  -- nome do span
    duration_ms     Float64,                 -- duração em milissegundos
    status_code     UInt8 DEFAULT 0,         -- 0=UNSET, 1=OK, 2=ERROR
    tenant_id       Nullable(String),
    attributes      String DEFAULT '{}'
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (trace_id, timestamp)
TTL timestamp + INTERVAL 30 DAY
SETTINGS index_granularity = 8192;

-- =============================================================================
-- AUDIT TRAIL — eventos de auditoria por tenant
-- Recebe do Redpanda via consumer assíncrono
-- =============================================================================
CREATE TABLE IF NOT EXISTS cascata_logs.audit_trail (
    timestamp     DateTime64(3),
    tenant_id     String,
    user_id       Nullable(String),
    action        LowCardinality(String),    -- 'INSERT', 'UPDATE', 'DELETE', 'SELECT'
    resource      String,                    -- tabela ou recurso afetado
    resource_id   Nullable(String),          -- ID do registro afetado
    ip_address    Nullable(String),
    user_agent    Nullable(String),
    details       String DEFAULT '{}'        -- JSON com payload da mudança
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (tenant_id, timestamp)
TTL timestamp + INTERVAL 365 DAY              -- Audit trail: 1 ano
SETTINGS index_granularity = 8192;

-- =============================================================================
-- TENANT METRICS — métricas de uso por tenant
-- Para billing, quotas e decisões de promoção de tier
-- =============================================================================
CREATE TABLE IF NOT EXISTS cascata_logs.tenant_metrics (
    timestamp       DateTime64(3),
    tenant_id       String,
    metric_name     LowCardinality(String),  -- 'requests', 'storage_bytes', 'query_duration_ms', etc.
    metric_value    Float64,
    tags            String DEFAULT '{}'      -- JSON com tags adicionais
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (tenant_id, metric_name, timestamp)
TTL timestamp + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;

-- =============================================================================
-- MATERIALIZED VIEWS — agregações para performance do Dashboard
-- =============================================================================

-- Logs por serviço e nível (últimas 24h para dashboard)
CREATE MATERIALIZED VIEW IF NOT EXISTS cascata_logs.logs_by_service_mv
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMMDD(window_start)
ORDER BY (service, level, window_start)
AS SELECT
    toStartOfMinute(timestamp) AS window_start,
    service,
    level,
    count() AS log_count
FROM cascata_logs.system_logs
GROUP BY window_start, service, level;

-- Métricas de request por tenant (para billing e quotas)
CREATE MATERIALIZED VIEW IF NOT EXISTS cascata_logs.tenant_requests_mv
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMMDD(window_start)
ORDER BY (tenant_id, window_start)
AS SELECT
    toStartOfHour(timestamp) AS window_start,
    tenant_id,
    count() AS request_count
FROM cascata_logs.audit_trail
WHERE tenant_id != ''
GROUP BY window_start, tenant_id;

-- =============================================================================
-- AUDITORIA DE CLASSES/TIER (SMART TENANT ENGINE)
-- Registra eventos de upgrade/downgrade/hibernação com 1 ano de retenção
-- =============================================================================
CREATE TABLE IF NOT EXISTS cascata_logs.tier_change_events (
    timestamp           DateTime64(3),
    tenant_id           String,
    from_tier           LowCardinality(String),
    to_tier             LowCardinality(String),
    change_type         LowCardinality(String),
    -- 'upgrade_automatic', 'upgrade_manual', 'downgrade_approved',
    -- 'hibernation', 'reactivation', 'blocked_upgrade', 'blocked_downgrade'
    proposal_id         Nullable(String),
    decided_by          Nullable(String),
    decided_by_type     LowCardinality(Nullable(String)),  -- 'member','agent','system'
    trigger_reason      String,
    block_reason        Nullable(String),  -- preenchido quando change_type = 'blocked_*'
    duration_ms         Nullable(UInt32),  -- tempo de execução da migração
    success             UInt8              -- 1 = sucesso, 0 = falha
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (tenant_id, timestamp)
TTL timestamp + INTERVAL 365 DAY;
