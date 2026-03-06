-- =============================================================================
-- SEED DATA: Catálogo de Extensões CASCATA (Etapa 0.8.1)
-- =============================================================================
-- Este script popula a tabela nativa cascata_cp.extensions_catalog.
-- Ele foi construído usando UPSERT (ON CONFLICT DO UPDATE) garantindo
-- idempotência absoluta em múltiplos deploys de staging/production.
-- Ref: SRS Req-2.20.2, Req-2.20.5
-- =============================================================================

INSERT INTO cascata_cp.extensions_catalog
  (name, display_name, category, available_in_shared, available_in_full, 
   compat_level, compat_notes, description, usage_snippet)
VALUES
  -- CATEGORIA 1 — Nativas (sempre disponíveis em ambas as imagens)
  ('uuid-ossp',
   'UUID Generator', 1, true, true, 'total', NULL,
   'Geração de UUIDs v1/v4',
   'SELECT uuid_generate_v4();'),

  ('pgcrypto',
   'PgCrypto', 1, true, true, 'total', NULL,
   'Funções de criptografia simétrica e assimétrica',
   'SELECT crypt(''senha'', gen_salt(''bf''));'),

  ('pg_trgm',
   'Trigram Search', 1, true, true, 'total', NULL,
   'Similaridade de texto e busca fuzzy por trigrama',
   'SELECT similarity(''cascata'', ''cascada'');'),

  ('hstore',
   'HStore', 1, true, true, 'total', NULL,
   'Armazenamento de pares chave-valor em coluna única',
   'SELECT ''a=>1, b=>2''::hstore;'),

  ('citext',
   'CI Text', 1, true, true, 'total', NULL,
   'Tipo de texto case-insensitive nativo',
   'CREATE TABLE t (email citext UNIQUE);'),

  ('ltree',
   'Ltree', 1, true, true, 'total', NULL,
   'Representação e consulta de hierarquias em árvore',
   'SELECT ''brasil.sp.capital''::ltree;'),

  ('intarray',
   'IntArray', 1, true, true, 'total', NULL,
   'Operações e índices GIN/GiST em arrays de inteiros',
   'SELECT sort(ARRAY[3,1,2]);'),

  ('unaccent',
   'Unaccent', 1, true, true, 'total', NULL,
   'Remove acentos de strings para normalização de busca',
   'SELECT unaccent(''ação brasileira'');'),

  ('fuzzystrmatch',
   'Fuzzy String Match', 1, true, true, 'total', NULL,
   'Algoritmos de similaridade fonética: Soundex, Metaphone, Levenshtein',
   'SELECT levenshtein(''cascata'', ''cascada'');'),

  ('pg_stat_statements',
   'Query Stats', 1, true, true, 'total', NULL,
   'Estatísticas de execução de queries para análise de performance',
   'SELECT query, mean_exec_time FROM pg_stat_statements ORDER BY mean_exec_time DESC LIMIT 10;'),

  ('pgaudit',
   'pgAudit', 1, true, true, 'total', NULL,
   'Auditoria detalhada de operações SQL por sessão e objeto',
   '-- Configurado via GUC: pgaudit.log = ''write, ddl'''),

  ('postgres_fdw',
   'Foreign Data Wrapper', 1, true, true, 'total', NULL,
   'Acesso a tabelas em servidores PostgreSQL externos',
   'CREATE SERVER ext FOREIGN DATA WRAPPER postgres_fdw OPTIONS (host ''...'', port ''5432'');'),

  ('dblink',
   'DB Link', 1, true, true, 'partial',
   'Conexões síncronas a bancos externos. Em YugabyteDB, evitar dentro de transações longas — pode segurar conexão do pool.',
   'Conexões ad-hoc a bancos PostgreSQL/YugabyteDB externos',
   'SELECT * FROM dblink(''host=... dbname=...'', ''SELECT id FROM tabela'') AS t(id uuid);'),

  ('plpgsql',
   'PL/pgSQL', 1, true, true, 'total', NULL,
   'Linguagem procedural padrão do PostgreSQL',
   'CREATE FUNCTION soma(a int, b int) RETURNS int LANGUAGE plpgsql AS $$ BEGIN RETURN a + b; END $$;'),

  -- CATEGORIA 2 — Pré-compiladas em ambas as imagens
  ('pg_cron',
   'Cron Jobs', 2, true, true, 'partial',
   'pg_cron não opera nativamente em cluster distribuído YugabyteDB. No Cascata, o acesso é exclusivamente via wrapper do Control Plane — o tenant nunca interage com o schema cron diretamente.',
   'Agendamento de jobs SQL periódicos. No Cascata: gerenciado via API de Cron Jobs.',
   '-- Use a API de Cron Jobs do Cascata no painel ou via SDK. Não use cron.schedule() diretamente.'),

  -- CATEGORIA 3 — Full only (instância dedicada STANDARD+)
  ('postgis',
   'PostGIS', 3, false, true, 'partial',
   'Funções de análise geoespacial e rasterização têm limitações em YugabyteDB distribuído por conta de Raft. Operações de leitura/escrita de geometrias funcionam normalmente.',
   'Dados geoespaciais, logística, cálculo de rotas e proximidade',
   'SELECT ST_Distance(ST_GeomFromText(''POINT(0 0)''), ST_GeomFromText(''POINT(1 1)''));'),

  ('postgis_tiger_geocoder',
   'Tiger Geocoder', 3, false, true, 'partial',
   'Geocodificação focada em endereços dos Estados Unidos. Requer carga de dados TIGER separada.',
   'Geocodificação de endereços (base de dados TIGER/US)',
   'SELECT g.rating, ST_X(g.geomout) AS lon, ST_Y(g.geomout) AS lat FROM geocode(''123 Main St, Springfield'') AS g;'),

  ('postgis_topology',
   'PostGIS Topology', 3, false, true, 'partial',
   'Operações de topologia geoespacial têm limitações em ambiente distribuído. Validar casos de uso específicos antes de adotar em produção.',
   'Topologia geoespacial — faces, arestas e nós',
   'SELECT topology.CreateTopology(''minha_topo'', 4326);')

ON CONFLICT (name) DO UPDATE SET
  display_name        = EXCLUDED.display_name,
  category            = EXCLUDED.category,
  available_in_shared = EXCLUDED.available_in_shared,
  available_in_full   = EXCLUDED.available_in_full,
  compat_level        = EXCLUDED.compat_level,
  compat_notes        = EXCLUDED.compat_notes,
  description         = EXCLUDED.description,
  usage_snippet       = EXCLUDED.usage_snippet;

-- =============================================================================
-- CATEGORIA 4 — Bloqueadas
-- Inseridas de forma independente porque utilizam blocked_reason em vez
-- de description/usage_snippet, preservando a semântica da engine.
-- =============================================================================
INSERT INTO cascata_cp.extensions_catalog
  (name, display_name, category, available_in_shared, available_in_full,
   compat_level, blocked_reason)
VALUES
  ('pgvector',
   'pgvector', 4, false, false, 'total',
   'Substituído pelo Qdrant — banco vetorial dedicado com isolamento multi-tenant correto via payload filtering nativo, HNSW com quantização (até 32x redução de footprint), DR integrado ao Orchestrator e audit trail no ClickHouse. pgvector em cluster compartilhado não oferece isolamento equivalente.')
ON CONFLICT (name) DO UPDATE SET
  display_name        = EXCLUDED.display_name,
  category            = EXCLUDED.category,
  available_in_shared = EXCLUDED.available_in_shared,
  available_in_full   = EXCLUDED.available_in_full,
  compat_level        = EXCLUDED.compat_level,
  blocked_reason      = EXCLUDED.blocked_reason;
