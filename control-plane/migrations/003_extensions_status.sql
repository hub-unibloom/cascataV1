-- =============================================================================
-- MIGRATION 003: Extension Status (Pending → Active)
-- =============================================================================
-- Alinha o esquema da tabela `cascata_cp.tenant_extensions` com os requisitos
-- da Etapa 0.8.3, implementando a mecânica de dupla escrita para resiliência a
-- falhas na instalação de extensões do PostgreSQL.
-- Ref: SRS Req-2.20.3
-- =============================================================================

BEGIN;

-- 1. Adicionar o enum informal (via CHECK) do campo status
-- 'pending': O usuário requisitou a instalação (primeira escrita).
-- 'active': A extensão foi criada no tenant (segunda escrita).
ALTER TABLE cascata_cp.tenant_extensions
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending', 'active'));

-- 2. Permitir que enabled_at seja nulo, pois assumiremos o registro apenas no 'active'
ALTER TABLE cascata_cp.tenant_extensions
    ALTER COLUMN enabled_at DROP NOT NULL;

-- 3. Adicionar coluna created_at para histórico do ciclo de vida da intenção
ALTER TABLE cascata_cp.tenant_extensions
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- 4. Atualizar registros antigos (se houver) para 'active'
UPDATE cascata_cp.tenant_extensions
SET status = 'active'
WHERE status = 'pending' AND enabled_at IS NOT NULL;

-- 5. Criar índice parcial para O(1) fetch no Job Reconciliador
-- Este índice garante zero-cost scan: busca apenas status pendentes (0.8.8)
CREATE INDEX IF NOT EXISTS idx_tenant_ext_pending
    ON cascata_cp.tenant_extensions(status, created_at)
    WHERE status = 'pending';

COMMIT;
