-- =============================================================================
-- CLICKHOUSE INIT SCRIPT: Extension Events Audit Trail (Etapa 0.8.7)
-- =============================================================================
-- Esta tabela opera como repositório forense imutável para lifecycle de Extensões.
-- Toda Habilitação, Erro de Habilitação, CVE Alert, e Bloqueio cai aqui.
-- O TTL de retenção de 365 dias é Requirement de Compliance Enterprise (Req-2.5.1).
-- =============================================================================

CREATE TABLE IF NOT EXISTS cascata_logs.extension_events (
    timestamp           DateTime64(3),
    tenant_id           String,
    
    -- Operação primária
    extension_name      LowCardinality(String),
    action              LowCardinality(String), 
    -- Actions válidas: 'enabled', 'disabled', 'blocked', 'cve_alert', 'reconciled', 'cve_check_skipped', 'inconsistency_warning'
    
    -- Identificação de quem engatilhou
    executed_by         Nullable(String),         -- member_id_uuid do Painel
    executed_by_type    LowCardinality(Nullable(String)), -- 'member', 'agent', 'system'
    
    -- Snapshots Ambientais de Rastreio (Contexto no ato da operação)
    tier_at_time        LowCardinality(String),   -- 'NANO', 'MICRO', 'STANDARD', etc...
    image_variant       LowCardinality(String),   -- 'shared', 'full'
    
    -- Determinística de Sucesso
    result              LowCardinality(String),   -- 'success', 'failed', 'blocked', 'skipped'
    failure_reason      Nullable(String),         -- Dumps de Exception (Ex: translated messages)
    
    -- Alerta CVE (Apenas se action = 'cve_alert')
    cve_ids             Nullable(String),         -- JSON array em String, Ex: '["CVE-2024-XXXX", "GHSA-xxxx"]'
    cve_severity        LowCardinality(Nullable(String)), -- 'CRITICAL', 'HIGH', 'MEDIUM', 'LOW'
    cve_source          LowCardinality(Nullable(String))  -- 'osv.dev', 'github_advisory'
    
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (tenant_id, timestamp)
TTL toDateTime(timestamp) + INTERVAL 365 DAY;
