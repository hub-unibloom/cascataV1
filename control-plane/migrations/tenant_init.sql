-- =====================================================
-- INICIALIZAÇÃO DE BANCO DE TENANT
-- Executado pelo Control Plane no provisionamento
-- =====================================================

-- Schema de reciclagem (soft delete) — deve existir desde o início
CREATE SCHEMA IF NOT EXISTS _recycled;

-- Role de API com permissões mínimas (usado pelo RLS Handshake)
DO $$ BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'cascata_api_role') THEN
        CREATE ROLE cascata_api_role NOLOGIN;
    END IF;
END $$;

-- Role anônimo (requests sem JWT)
DO $$ BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'cascata_anon_role') THEN
        CREATE ROLE cascata_anon_role NOLOGIN;
    END IF;
END $$;

-- Permissões base
GRANT USAGE ON SCHEMA public TO cascata_api_role;
GRANT USAGE ON SCHEMA public TO cascata_anon_role;

-- BLOQUEIO: nenhum role acessa _recycled diretamente
-- (acesso apenas pelo Control Plane com role privilegiado)
REVOKE ALL ON SCHEMA _recycled FROM PUBLIC;
REVOKE ALL ON SCHEMA _recycled FROM cascata_api_role;
REVOKE ALL ON SCHEMA _recycled FROM cascata_anon_role;

-- BLOQUEIO: isolamento Multi-Tenant do pg_cron
-- O schema cron nativo abriga tabelas compartilhadas. Impedimos acesso ao driver puro.
REVOKE ALL ON SCHEMA cron FROM cascata_api_role;
REVOKE ALL ON SCHEMA cron FROM cascata_anon_role;

-- Extensões base (sempre habilitadas)
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Habilitar Row Level Security globalmente
-- (cada tabela criada pelo tenant terá RLS ativado individualmente)
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO cascata_api_role;
