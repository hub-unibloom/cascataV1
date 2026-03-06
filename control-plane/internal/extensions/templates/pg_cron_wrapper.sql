-- =============================================================================
-- EXTENSION TEMPLATE: PG_CRON WRAPPER (Etapa 0.8.4)
-- =============================================================================
-- pg_cron exige tabelas distribuídas no schema 'cron', que quebra regras de
-- Multi-Tenancy (Bugs de visibilidade onde o Tenant A vê dados do Tenant B).
-- Para isso, este template engatilha wrappers no próprio schema do Tenant
-- (geralmente "public") que encapsula (e prefixa) o acesso base.
--
-- Ref: SRS Req-2.20.4
-- =============================================================================

-- Substituir '{tenant_schema}' pelo schema do tenant no Enabler Go
-- Substituir '{tenant_id}' pelo UUID oficial no Enabler Go

-- [1] Agendamento (Comando)
CREATE OR REPLACE FUNCTION "{tenant_schema}".schedule_job(
    job_name  TEXT,
    schedule  TEXT,   -- cron expression: '0 * * * *'
    command   TEXT    -- SQL a executar
) RETURNS BIGINT
LANGUAGE plpgsql
SECURITY DEFINER   -- IMPORTANTE: Executa com os privilégios de owner (Control Plane), já que o Tenant normal (api_role) não enxerga `cron`.
SET search_path = "{tenant_schema}", public
AS $$
DECLARE
    v_job_id BIGINT;
    v_prefixed_name TEXT;
BEGIN
    -- Prefixar nome com tenant_id para garantir unicidade global no PG e isolamento
    v_prefixed_name := '{tenant_id}__' || job_name;

    -- Prevenir duplicações para o mesmo tenant sob o mesmo nickname legível
    IF EXISTS (
        SELECT 1 FROM cron.job
        WHERE jobname = v_prefixed_name
    ) THEN
        RAISE EXCEPTION 'O Job de nome "%" já existe neste projeto.', job_name;
    END IF;

    -- O comando cron roda via extension
    SELECT cron.schedule(v_prefixed_name, schedule, command)
    INTO v_job_id;

    -- Emite o sinal ao evento (Listen/Notify) - O Worker backend absorve isso
    PERFORM pg_notify(
        'cascata_cron_created',
        json_build_object(
            'tenant_id', '{tenant_id}',
            'job_id', v_job_id,
            'job_name', job_name,
            'schedule', schedule
        )::text
    );

    RETURN v_job_id;
END;
$$;


-- [2] Listagem de Jobs Isolados (Ocultação de Identidade/Filtragem por Prefix)
CREATE OR REPLACE FUNCTION "{tenant_schema}".list_jobs()
RETURNS TABLE(job_id BIGINT, job_name TEXT, schedule TEXT, active BOOLEAN, last_run TIMESTAMPTZ)
LANGUAGE sql
SECURITY DEFINER
SET search_path = "{tenant_schema}", public
AS $$
    SELECT
        jobid,
        replace(jobname, '{tenant_id}__', '') AS job_name, -- Strip do prefixo local, para não sujar a visibilidade do Dashboard Painel
        schedule,
        active,
        last_run_start_time
    FROM cron.job
    WHERE jobname LIKE '{tenant_id}__%';
$$;


-- [3] Deleção Controlada
CREATE OR REPLACE FUNCTION "{tenant_schema}".unschedule_job(job_name TEXT)
RETURNS VOID
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = "{tenant_schema}", public
AS $$
BEGIN
    PERFORM cron.unschedule('{tenant_id}__' || job_name);
    PERFORM pg_notify(
        'cascata_cron_deleted',
        json_build_object('tenant_id', '{tenant_id}', 'job_name', job_name)::text
    );
END;
$$;
