-- =============================================================================
-- MIGRATION 004: Metadata dos Cron Jobs do Wrapper pg_cron
-- =============================================================================
-- Para isolar namespaces entre tenants dentro do `cron.job` compartilhado.
-- O painel não faz SELECT direto em cron.job, ele lê os metadados do 
-- Control Plane mantendo strict multi-tenant boundary.
-- Ref: SRS Req-2.20.4
-- =============================================================================

BEGIN;

CREATE TABLE IF NOT EXISTS cascata_cp.cron_jobs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES cascata_cp.tenants(id),
    pg_job_id   BIGINT NOT NULL,        -- ID interno retornado pelo cron.schedule() do Postgres
    job_name    TEXT NOT NULL,          -- nome legível (sem prefixo do tenant 'uuid__')
    schedule    TEXT NOT NULL,          -- "* * * * *"
    command     TEXT NOT NULL,          -- SQL a executar
    active      BOOLEAN NOT NULL DEFAULT true,
    created_by  UUID,                   -- Membro que criou na UI
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_run_at TIMESTAMPTZ,
    UNIQUE(tenant_id, job_name)         -- Garante nomes únicos por tenant
);

-- Indexação rápida para dashboard (Ler os jobs do tenant na UI)
CREATE INDEX IF NOT EXISTS idx_cron_jobs_tenant ON cascata_cp.cron_jobs(tenant_id);

COMMIT;
