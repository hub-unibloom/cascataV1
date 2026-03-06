-- =====================================================
-- 002_downgrade_proposals.sql
-- Adiciona tabelas para o Motor de Classificação e Downgrade (Etapa 0.7)
-- =====================================================

-- =====================================================
-- PROPOSTAS DE DOWNGRADE / HIBERNAÇÃO (Req-2.1.1)
-- Criadas pelo algoritmo, executadas apenas após aprovação
-- =====================================================
CREATE TABLE cascata_cp.downgrade_proposals (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES cascata_cp.tenants(id),
    proposal_type       TEXT NOT NULL CHECK (proposal_type IN (
                            'downgrade_tier',    -- mudança de tier
                            'hibernation',       -- hibernação por inatividade
                            'reactivation'       -- reverter hibernação
                        )),
    status              TEXT NOT NULL DEFAULT 'pending' CHECK (status IN (
                            'pending',           -- aguardando decisão do operador
                            'approved',          -- aprovada, aguardando janela de execução
                            'rejected',          -- rejeitada pelo operador
                            'postponed',         -- adiada (nova data em execution_after)
                            'executed',          -- concluída com sucesso
                            'cancelled',         -- cancelada (ex: tenant voltou a ser ativo)
                            'failed'             -- falhou durante execução
                        )),
    -- Tiers envolvidos
    current_tier        TEXT NOT NULL,
    proposed_tier       TEXT,                    -- null para hibernação
    -- Dados da análise que gerou a proposta
    trigger_reason      TEXT NOT NULL,           -- 'volume_threshold' | 'inactivity_level_2' | 'inactivity_level_3'
    days_below_threshold INTEGER,                -- dias consecutivos abaixo dos thresholds
    metrics_snapshot    JSONB NOT NULL,          -- snapshot de TenantMetrics no momento da proposta
    impact_analysis     JSONB,                   -- features afetadas, storage que excede nova cota, etc.
    -- Economia estimada
    estimated_savings   JSONB,                   -- { "ram_mb": N, "description": "..." }
    -- Decisão
    decided_by          UUID,                    -- member_id ou agent_id
    decided_by_type     TEXT CHECK (decided_by_type IN ('member', 'agent', 'system')),
    decided_at          TIMESTAMPTZ,
    decision_notes      TEXT,                    -- motivo da rejeição ou observações do operador
    -- Execução
    execution_after     TIMESTAMPTZ,             -- não executar antes desta data (para 'postponed')
    executed_at         TIMESTAMPTZ,
    execution_log       TEXT,                    -- log de erros se status = 'failed'
    -- Timestamps
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Notificações
    notification_sent_at TIMESTAMPTZ,
    reminder_sent_at     TIMESTAMPTZ             -- lembrete 48h antes de auto-execução
);

CREATE INDEX idx_downgrade_proposals_tenant ON cascata_cp.downgrade_proposals(tenant_id);
CREATE INDEX idx_downgrade_proposals_status ON cascata_cp.downgrade_proposals(status);
CREATE INDEX idx_downgrade_proposals_execution ON cascata_cp.downgrade_proposals(execution_after)
    WHERE status = 'approved';

-- =====================================================
-- SNAPSHOTS DIÁRIOS DE MÉTRICAS (Req-2.1.1)
-- =====================================================
CREATE TABLE cascata_cp.tenant_metrics_snapshot (
    id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id                   UUID NOT NULL REFERENCES cascata_cp.tenants(id),
    captured_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Tráfego
    p95_requests_per_hour_30d   NUMERIC(12,2),
    p95_concurrent_conns_30d    NUMERIC(12,2),
    avg_requests_per_day_30d    NUMERIC(12,2),
    -- Dados
    storage_used_gb             NUMERIC(10,3),
    -- Usuários
    active_users_30d            BIGINT,
    max_simultaneous_agents     INTEGER,
    -- Atividade
    last_activity_at            TIMESTAMPTZ,
    days_inactive_consecutive   INTEGER NOT NULL DEFAULT 0,
    -- Tendência
    growth_trend_coefficient    NUMERIC(8,4),   -- positivo = crescendo, negativo = decrescendo
    -- Classificação resultante deste snapshot
    classification_result       TEXT,           -- 'stable' | 'candidate_downgrade' | 'candidate_upgrade' | 'inactive_level_1' | 'inactive_level_2' | 'inactive_level_3'
    proposal_id                 UUID REFERENCES cascata_cp.downgrade_proposals(id)
);

CREATE INDEX idx_metrics_snapshot_tenant ON cascata_cp.tenant_metrics_snapshot(tenant_id);
CREATE INDEX idx_metrics_snapshot_date ON cascata_cp.tenant_metrics_snapshot(captured_at);
