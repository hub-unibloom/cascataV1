-- =====================================================
-- BANCO DO CONTROL PLANE — NÃO É BANCO DE TENANT
-- Armazena: metadata de schemas, config de tenants,
-- catálogo de extensões, estado de pools, etc.
-- =====================================================

-- Schema principal do Control Plane
CREATE SCHEMA IF NOT EXISTS cascata_cp;

-- =====================================================
-- TENANTS — registro de cada projeto/tenant
-- =====================================================
CREATE TABLE cascata_cp.tenants (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    slug            TEXT NOT NULL UNIQUE,
    tier            TEXT NOT NULL CHECK (tier IN ('NANO','MICRO','STANDARD','ENTERPRISE','SOVEREIGN')),
    status          TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','hibernated','suspended','deleted')),
    db_host         TEXT NOT NULL,           -- host do YugabyteDB deste tenant
    db_port         INTEGER NOT NULL DEFAULT 5433,
    db_name         TEXT NOT NULL,           -- nome do database
    db_schema       TEXT NOT NULL DEFAULT 'public',
    pool_id         TEXT,                    -- ID do pool no YSQL CM
    image_variant   TEXT NOT NULL CHECK (image_variant IN ('shared','full')),
    retention_days  INTEGER NOT NULL DEFAULT 7,  -- retenção soft delete
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_active_at  TIMESTAMPTZ,
    config_json     JSONB NOT NULL DEFAULT '{}'::jsonb  -- config extensível
);

-- =====================================================
-- SCHEMA METADATA — representação interna de tabelas
-- Lido por: TableCreator, APIDocs, SDK type gen, MCP Server, Protocol Cascata, Pingora
-- Campos obrigatórios desde v1: status, computation, validations, pii, is_writable
-- =====================================================
CREATE TABLE cascata_cp.table_metadata (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id             UUID NOT NULL REFERENCES cascata_cp.tenants(id),
    table_name            TEXT NOT NULL,
    table_schema          TEXT NOT NULL DEFAULT 'public',
    status                TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','recycled')),
    -- Campos de soft delete (preenchidos quando status = 'recycled')
    original_name         TEXT,
    original_schema       TEXT,
    recycled_name         TEXT,
    deleted_at            TIMESTAMPTZ,
    deleted_by            UUID,
    scheduled_purge_at    TIMESTAMPTZ,
    row_count_at_deletion BIGINT,
    suspended_fkeys       JSONB DEFAULT '[]'::jsonb,
    impact_analysis       JSONB,
    -- Metadata geral
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, table_schema, table_name)
);

CREATE TABLE cascata_cp.column_metadata (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    table_meta_id   UUID NOT NULL REFERENCES cascata_cp.table_metadata(id) ON DELETE CASCADE,
    tenant_id       UUID NOT NULL REFERENCES cascata_cp.tenants(id),
    column_name     TEXT NOT NULL,
    data_type       TEXT NOT NULL,
    is_nullable     BOOLEAN NOT NULL DEFAULT true,
    default_value   TEXT,
    ordinal_pos     INTEGER NOT NULL,
    -- Campo: status (soft delete awareness)
    status          TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','recycled')),
    -- Campo: computation (computed/virtual columns)
    computation     JSONB,  -- null = coluna regular
    -- Estrutura quando não-null:
    -- { "kind": "stored_generated"|"api_computed",
    --   "expression": "quantity * unit_price",
    --   "layer": "database"|"api",
    --   "jwt_claims_required": [],
    --   "filterable": true/false,
    --   "sortable": true/false }
    -- Campo: validations (data validation rules)
    validations     JSONB NOT NULL DEFAULT '[]'::jsonb,
    -- Estrutura: array de regras:
    -- [{ "type": "required"|"regex"|"range"|"length"|"enum"|"cross_field"|"jwt_context"|"unique_soft",
    --    "message": "Mensagem legível",
    --    "severity": "error"|"warning",
    --    "params": { ... dependem do tipo } }]
    -- Campo: pii (data masking)
    pii             BOOLEAN NOT NULL DEFAULT false,
    -- Campo: is_writable (derivado de computation)
    is_writable     BOOLEAN NOT NULL DEFAULT true,
    -- Metadata
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(table_meta_id, column_name)
);

-- =====================================================
-- CATÁLOGO DE EXTENSÕES
-- =====================================================
CREATE TABLE cascata_cp.extensions_catalog (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                TEXT NOT NULL UNIQUE,
    display_name        TEXT NOT NULL,
    description         TEXT,
    category            INTEGER NOT NULL CHECK (category IN (1,2,3,4)),
    -- 1=Nativa, 2=Pré-compilada ambas imagens, 3=Full only, 4=Bloqueada
    available_in_shared BOOLEAN NOT NULL DEFAULT false,
    available_in_full   BOOLEAN NOT NULL DEFAULT false,
    compat_level        TEXT NOT NULL DEFAULT 'total' CHECK (compat_level IN ('total','partial')),
    compat_notes        TEXT,
    usage_snippet       TEXT,
    blocked_reason      TEXT,  -- preenchido quando category=4
    storage_impact      TEXT,
    performance_impact  TEXT,
    version             TEXT
);

-- Extensões habilitadas por tenant
CREATE TABLE cascata_cp.tenant_extensions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES cascata_cp.tenants(id),
    extension   TEXT NOT NULL REFERENCES cascata_cp.extensions_catalog(name),
    enabled_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    enabled_by  UUID,
    UNIQUE(tenant_id, extension)
);

-- =====================================================
-- CONFIGURAÇÃO DE POOL POR TENANT
-- =====================================================
CREATE TABLE cascata_cp.pool_configs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES cascata_cp.tenants(id) UNIQUE,
    pool_size       INTEGER NOT NULL,
    queue_limit     INTEGER NOT NULL,
    stmt_timeout_ms INTEGER NOT NULL,
    idle_tx_timeout_ms INTEGER NOT NULL,
    safety_factor   NUMERIC(3,2) NOT NULL,
    last_resized_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    resize_source   TEXT  -- 'adaptive'|'manual'|'promotion'|'downgrade'
);

-- =====================================================
-- ÍNDICES
-- =====================================================
CREATE INDEX idx_table_meta_tenant ON cascata_cp.table_metadata(tenant_id);
CREATE INDEX idx_table_meta_status ON cascata_cp.table_metadata(status);
CREATE INDEX idx_column_meta_table ON cascata_cp.column_metadata(table_meta_id);
CREATE INDEX idx_tenant_ext ON cascata_cp.tenant_extensions(tenant_id);
