# TASK — Fase 0: Alicerce Invisível
# Cascata "Koenigsegg" — Esqueleto Funcional

> **Objetivo:** Após concluir esta fase, a primeira request de um tenant real pode percorrer o sistema do início ao fim com segurança, isolamento e performance. Todo item abaixo é invisível para o tenant mas crítico para o operador. Nada é construído em fases posteriores que funcione sem esses alicerces.

> **Referências supremas:**
> - [Roadmap](file:///home/cocorico/projetossz/cascataV1/Roadmap_48_Implementacoes_Cascata.md) — Seção "ETAPA 0"
> - [SRS](file:///home/cocorico/projetossz/cascataV1/SRS_CascataV1.md) — Requisitos referenciados por número
> - [SAD](file:///home/cocorico/projetossz/cascataV1/SAD_CascataV1.md) — Decisões arquiteturais permanentes

---

## Pré-requisitos Globais (antes de qualquer implementação da Fase 0)

Antes de tocar em qualquer item 0.x, a infraestrutura base precisa existir minimamente para que os itens tenham onde operar.

### PR-1 — Estrutura de diretórios do monorepo

- [x] **PR-1 criado e commitado**

Decisão: monorepo — SAD §1 diz "a mesma base de código". A árvore abaixo é definitiva:

```
cascataV1/
├── SRS_CascataV1.md                    # Fonte de verdade funcional
├── SAD_CascataV1.md                    # Fonte de verdade arquitetural
├── Roadmap_48_Implementacoes_Cascata.md # Fonte de verdade de prioridade
├── TASK_Fase0_Alicerce_Invisivel.md    # Este arquivo
│
├── control-plane/                      # Go 1.26+ — o cérebro roteador
│   ├── cmd/
│   │   └── cascata-cp/
│   │       └── main.go                 # Entrypoint. Carrega config, inicia Chi, escuta.
│   ├── internal/
│   │   ├── config/
│   │   │   └── config.go               # Struct CascataConfig lida do YAML/env. Zero hardcoded.
│   │   ├── server/
│   │   │   └── server.go               # Chi router, middleware stack, graceful shutdown.
│   │   ├── tenant/
│   │   │   ├── router.go               # Tenant Router — resolve tier → Data Plane endpoint.
│   │   │   ├── classifier.go           # Smart Tenant Classification Engine (promote/downgrade).
│   │   │   └── provisioner.go          # Provisiona recursos por tier.
│   │   ├── pool/
│   │   │   ├── orchestrator.go         # Orquestra pgcat: create/update/delete pools.
│   │   │   ├── warmup.go               # Staggered Pool Warming (anti-thundering herd).
│   │   │   └── sizing.go              # Pool Size Adaptativo (fórmula p95 × safety × growth).
│   │   ├── metadata/
│   │   │   ├── schema.go               # Schema Metadata CRUD no YugabyteDB do CP.
│   │   │   ├── cache.go                # Invalidação / refresh no DragonflyDB.
│   │   │   └── models.go              # Structs: TableMeta, ColumnMeta, ValidationRule, etc.
│   │   ├── recyclebin/
│   │   │   ├── recycler.go             # Operação de soft delete (move para _recycled).
│   │   │   ├── restorer.go             # Restauração do _recycled para schema original.
│   │   │   ├── purger.go               # Purge permanente com 3 camadas de confirmação.
│   │   │   └── impact.go              # Protocol Cascata — análise de impacto pré-delete.
│   │   ├── extensions/
│   │   │   ├── catalog.go              # Catálogo de extensões por imagem.
│   │   │   ├── enabler.go              # CREATE EXTENSION no schema do tenant.
│   │   │   └── cve_monitor.go         # Monitoramento de CVEs em extensões ativas.
│   │   ├── scanning/
│   │   │   ├── scanner.go              # Pipeline de vulnerability scanning.
│   │   │   └── severity.go            # Classificação CRITICAL/HIGH/MEDIUM/LOW.
│   │   ├── observability/
│   │   │   └── otel.go                # Setup OTel SDK, tracer, meter.
│   │   └── health/
│   │       └── health.go              # /health endpoint + component health checks.
│   ├── migrations/
│   │   └── 001_initial_schema.sql     # DDL do banco do Control Plane (ver PR-8).
│   ├── go.mod
│   └── go.sum
│
├── gateway/                            # Rust/Pingora — o motor do Data Plane
│   ├── src/
│   │   ├── main.rs                     # Entrypoint Pingora. Carrega config, monta pipeline.
│   │   ├── config.rs                   # Configuração do gateway (pools, TLS, upstream).
│   │   ├── middleware/
│   │   │   ├── mod.rs
│   │   │   ├── jwt.rs                  # JWT validation e extração de claims.
│   │   │   ├── abac.rs                 # Cedar ABAC policy check.
│   │   │   ├── rate_limit.rs           # Rate limiting via DragonflyDB lookup.
│   │   │   └── rls_handshake.rs       # RLS Handshake: BEGIN→SET LOCAL→set_config→query→COMMIT.
│   │   ├── validation/
│   │   │   ├── mod.rs
│   │   │   ├── engine.rs               # Validation Engine: required→type→regex→cross_field→jwt→unique.
│   │   │   ├── expression.rs           # Avaliador de expressões seguro (zero eval).
│   │   │   └── error.rs               # CascataValidationError format.
│   │   ├── computed/
│   │   │   ├── mod.rs
│   │   │   └── api_columns.rs         # API Computed Columns — injeção na resposta.
│   │   ├── storage/
│   │   │   ├── mod.rs
│   │   │   ├── router.rs               # Storage Router — classifica setor, valida magic bytes.
│   │   │   ├── magic_bytes.rs          # Dicionário de assinaturas de formato.
│   │   │   └── quota.rs               # Reserva preventiva de cota no DragonflyDB.
│   │   ├── proxy/
│   │   │   ├── mod.rs
│   │   │   └── upstream.rs            # Proxy pass para pgcat / YSQL CM.
│   │   └── error/
│   │       ├── mod.rs
│   │       └── pg_translator.rs       # Traduz erros PostgreSQL → CascataValidationError.
│   ├── Cargo.toml
│   └── Cargo.lock
│
├── dashboard/                          # SvelteKit + TypeScript — DX do operador
│   ├── src/
│   │   ├── routes/                     # Rotas do painel
│   │   ├── lib/
│   │   │   ├── components/            # Componentes reutilizáveis
│   │   │   └── stores/               # Stores Svelte (estado global)
│   │   └── app.html
│   ├── package.json
│   └── svelte.config.js
│
├── sdk/                                # SDKs do Cascata
│   ├── typescript/
│   │   ├── cascata-client/            # @cascata/client — SDK nativo
│   │   └── cascata-compat/            # @cascata/compat — compatibilidade Supabase
│   └── cli/
│       └── cascata-cli/               # CLI: cascata init, dev, db diff, etc.
│
├── infra/                              # Infraestrutura e deployment
│   ├── docker/
│   │   ├── docker-compose.shelter.yml  # Modo Shelter — VPS $20/mês
│   │   ├── docker-compose.dev.yml      # Desenvolvimento local (cascata dev)
│   │   ├── yugabytedb/
│   │   │   ├── Dockerfile.shared       # cascata/yugabytedb:shared
│   │   │   └── Dockerfile.full        # cascata/yugabytedb:full
│   │   ├── pgcat/
│   │   │   └── pgcat.toml             # Config base do pgcat
│   │   ├── clickhouse/
│   │   │   └── config.xml             # Config ClickHouse single node
│   │   └── vector/
│   │       └── vector.toml            # Config Vector.dev collector
│   ├── k8s/                            # Kubernetes manifests (STANDARD+)
│   │   └── operators/                 # Custom Operators
│   └── cilium/
│       └── policies/                  # Network Policies eBPF
│
├── scripts/                            # Scripts de automação
│   ├── init-db.sh                      # Inicializa schemas, roles, extensions no YugabyteDB
│   └── seed-extensions-catalog.sql    # Popula catálogo de extensões
│
└── .agents/
    └── workflows/                     # Workflows para o agente
```

**Ref:** SAD §1 (Os Três Planos), SRS Req-2.1.1 a Req-2.1.5

---

### PR-2 — Docker Compose modo Shelter

- [x] **PR-2 criado e testável com `docker compose up`**

Arquivo: `infra/docker/docker-compose.shelter.yml`

Serviços obrigatórios com limites de RAM explícitos (deploy.resources.limits.memory):

| Servicio | Imagem | RAM Limit | Porta | Dep. de |
|---------|--------|-----------|-------|---------|
| `yugabytedb` | `cascata/yugabytedb:shared` | 900MB | 5433 (YSQL), 7100 (master) | — |
| `dragonflydb` | `docker.dragonflydb.io/dragonflydb/dragonfly` | 128MB | 6379 | — |
| `redpanda` | `docker.redpanda.com/redpandadata/redpanda` | 256MB | 9092 (kafka), 8081 (schema registry) | — |
| `clickhouse` | `clickhouse/clickhouse-server` | 384MB | 8123 (HTTP), 9000 (native) | — |
| `openbao` | `quay.io/openbao/openbao` | 64MB | 8200 | — |

| `vector` | `timberio/vector` | 64MB | 8686 (API) | clickhouse, redpanda |
| `victoriametrics` | `victoriametrics/victoria-metrics` | 128MB | 8428 | — |
| `qdrant` | `qdrant/qdrant` | 128MB | 6333 | — |
| `centrifugo` | `centrifugo/centrifugo` | 128MB | 8000 | redpanda |
| `gateway` | build: `../../gateway` | 48MB | 8080 (HTTP), 8443 (HTTPS) | pgcat, dragonflydb |
| `control-plane` | build: `../../control-plane` | 32MB | 9090 | yugabytedb, dragonflydb, clickhouse |

**Total: ~2292MB de limits, ~1800MB de uso real.** O limit é teto; uso real é menor.

Rede: uma bridge Docker chamada `cascata-shelter`. Todos os serviços na mesma rede.

Volumes persistentes nomeados:
- `cascata-yb-data` → `/home/yugabyte/data` (YugabyteDB)
- `cascata-ch-data` → `/var/lib/clickhouse` (ClickHouse)
- `cascata-rp-data` → `/var/lib/redpanda/data` (Redpanda)
- `cascata-df-data` → `/data` (DragonflyDB)
- `cascata-openbao-data` → `/openbao/data` (OpenBao)
- `cascata-minio-data` → `/data` (MinIO — adicionado quando storage entrar)

**Ref:** SAD §2 (Modo Shelter), SRS Req-3.2.1

---

### PR-3 — Imagem Docker `cascata/yugabytedb:shared`

- [ ] **PR-3 — Dockerfile.shared construída e pronta** 

Arquivo: `infra/docker/yugabytedb/Dockerfile.shared`

Conteúdo conceitual:
```dockerfile
# Stage 1: builder (apenas para pg_cron)
FROM yugabytedb/yugabyte:2.25.x-b<latest> AS builder
# Compila pg_cron contra os headers do YugabyteDB
# Copia apenas .so, .control, .sql para stage final

# Stage 2: runtime
FROM yugabytedb/yugabyte:2.25.x-b<latest>
# Copia artefatos pg_cron do builder
COPY --from=builder /usr/share/postgresql/extension/pg_cron* ...
COPY --from=builder /usr/lib/postgresql/*/lib/pg_cron.so ...
# Extensões nativas já incluídas no YugabyteDB base:
# uuid-ossp, pgcrypto, pg_trgm, hstore, citext, ltree,
# intarray, unaccent, fuzzystrmatch, pg_stat_statements,
# pgaudit, postgres_fdw, dblink, plpgsql
# Zero ferramentas de compilação (gcc, make, headers) na imagem final
```

**Ref:** SAD §Extension Profile System, SRS Req-2.20.1, Req-2.20.2

---

### PR-4 — Imagem Docker `cascata/yugabytedb:full`

- [x] **PR-4 — Dockerfile.full construída e testada**

Arquivo: `infra/docker/yugabytedb/Dockerfile.full`

Adiciona sobre `:shared`:
- PostGIS (+ Tiger Geocoder + Topology) — dependências: libgeos, libproj, libgdal
- Todas compiladas no stage builder, apenas `.so`/`.control`/`.sql` no runtime
- O builder possui limitador dinâmico de threads no GCC (via `ARG MAKEFLAGS="-j2"`) que é ativado automaticamente pelo `install.sh` em hosts com <= 2.5GB de RAM para evitar morte por OOM fatal durante a pesada compilação C++ destas extensões.
- Imagem final sem gcc, make, headers

**Ref:** SAD §Extension Profile System, SRS Req-2.20.1

---

### PR-5 — Skeleton do Control Plane (Go 1.26+)

- [x] **PR-5 — `go mod init`, Chi router, config struct, health endpoint**

Arquivo: `control-plane/cmd/cascata-cp/main.go` — entrypoint mínimo:
```go
func main() {
    cfg := config.Load() // Carrega de YAML + env vars. Zero hardcoded.
    
    // Todo contexto com cancelamento
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    // Graceful shutdown em SIGINT/SIGTERM
    go handleShutdown(cancel)
    
    srv := server.New(cfg)
    srv.Run(ctx)
}
```

Arquivo: `control-plane/internal/config/config.go` — struct de configuração:
```go
type CascataConfig struct {
    // Banco do Control Plane (schema metadata, catálogo extensões, etc.)
    ControlDB    PostgresConfig `yaml:"control_db"`
    // DragonflyDB para cache de metadata
    Cache        DragonflyConfig `yaml:"cache"`
    // pgcat management
    PgCat        PgCatConfig    `yaml:"pgcat"`
    // ClickHouse para observabilidade
    Analytics    ClickHouseConfig `yaml:"analytics"`
    // Redpanda para eventos
    EventStream  RedpandaConfig `yaml:"event_stream"`
    // OpenBao para secrets
    KMS          OpenBaoConfig  `yaml:"kms"`
    // Configurações de tier
    Tiers        TierConfig     `yaml:"tiers"`
    // HTTP server
    HTTP         HTTPConfig     `yaml:"http"`
}
```

Regra: todo campo sensível (passwords, keys) referencia OpenBao path, nunca valor direto.

**Ref:** SAD §A (Control Plane), SRS Req-2.1.1, Req-2.1.2

---

### PR-6 — Skeleton do Pingora Gateway (Rust)

- [x] **PR-6 — Cargo init, Pingora pipeline, módulos vazios**

Arquivo: `gateway/Cargo.toml`:
```toml
[package]
name = "cascata-gateway"
version = "0.1.0"
edition = "2021"

[dependencies]
pingora = "0.4"          # ou versão mais recente
pingora-proxy = "0.4"
tokio = { version = "1", features = ["full"] }
serde = { version = "1", features = ["derive"] }
serde_json = "1"
redis = "0.27"           # Para DragonflyDB (API Redis-compatible)
tracing = "0.1"
tracing-subscriber = "0.3"
opentelemetry = "0.27"
```

Pipeline no `main.rs`: request chega → middleware chain → proxy pass para pgcat.
Módulos vazios criados com `// TODO: implementar na task 0.x correspondente` e struct pública para a interface.

**Ref:** SAD §B (Data Plane), SRS Req-2.1.5

---

### PR-7 — Pipeline de observabilidade mínimo

- [x] **PR-7 — OTel, Vector, VictoriaMetrics, ClickHouse configurados**

Pipeline: App (OTel SDK) → Vector.dev → VictoriaMetrics (métricas) + ClickHouse (logs/traces)

Arquivo: `infra/docker/vector/vector.toml` com sources, transforms, sinks configurados.

Tabelas ClickHouse para logs (criadas no init):
```sql
CREATE TABLE IF NOT EXISTS cascata_logs.system_logs (
    timestamp     DateTime64(3),
    service       LowCardinality(String),  -- 'control-plane', 'gateway', 'pgcat'
    level         LowCardinality(String),  -- 'debug','info','warn','error','fatal'
    trace_id      String,
    span_id       String,
    tenant_id     Nullable(String),
    message       String,
    attributes    String  -- JSON
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (service, timestamp)
TTL timestamp + INTERVAL 90 DAY;
```

**Ref:** SAD §C (Infra Plane — Observabilidade), SRS Req-3.1

---

### PR-8 — Banco de dados do Control Plane (DDL inicial)

- [x] **PR-8 — DDL do banco do Control Plane criado e aplicado**

O Control Plane tem seu **próprio banco no YugabyteDB** (separado dos bancos de tenant). Este banco armazena schema metadata, catálogo de extensões, configuração de tenants e todas as tabelas de gestão.

Arquivo: `control-plane/migrations/001_initial_schema.sql`

```sql
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
    pool_id         TEXT,                    -- ID do pool no pgcat
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
    status      TEXT NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending', 'active')),
    enabled_at  TIMESTAMPTZ,
    enabled_by  UUID,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, extension)
);

CREATE INDEX idx_tenant_ext ON cascata_cp.tenant_extensions(tenant_id);
CREATE INDEX idx_tenant_ext_pending
    ON cascata_cp.tenant_extensions(status, created_at)
    WHERE status = 'pending';

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
```

**Ref:** SAD §Schema Metadata, SRS Req-2.17.7, Req-2.18.3, Req-2.19.4

---

### PR-9 — DDL inicial no YugabyteDB de tenant (template)

- [x] **PR-9 — SQL de inicialização de banco de tenant criado**

Quando o Control Plane provisiona um novo tenant, executa este SQL no banco do tenant:

Arquivo: `control-plane/migrations/tenant_init.sql`

```sql
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

-- Extensões base (sempre habilitadas)
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Habilitar Row Level Security globalmente
-- (cada tabela criada pelo tenant terá RLS ativado individualmente)
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO cascata_api_role;
```

**Ref:** SRS Req-2.3.3 (RLS Handshake), Req-2.17.1 (schema _recycled)

---

### PR-10 — Configuração base do gateway

- [x] **PR-10 — gateway.toml com configuração de upstream**

Arquivo: `infra/docker/gateway/gateway.toml`

```toml
pool_mode = "transaction"
# NOTA ARQUITETURAL: Em modo "transaction", o pgcat utiliza Pipeline Mode 
# envando a Server Reset Query e a transação subsequente em lote (batching).
# Isso evita a penalidade de 3x RTT (Round Trip Time) inerente ao Raft do YugabyteDB.

# Server Reset Query — OBRIGATÓRIA. Limpa estado de sessão entre transações.
# Sem isso, claims JWT de um tenant vazam para o próximo.
server_reset_query = """
RESET ROLE;
SELECT set_config('request.jwt.claim.sub', '', false);
SELECT set_config('request.jwt.claim.tenant_id', '', false);
SELECT set_config('request.jwt.claim.role', '', false);
"""

# Timeouts para tier NANO (mais restritivo do cluster compartilhado)
default_statement_timeout = 10000    # 10s em ms
default_idle_in_transaction_timeout = 5000  # 5s em ms

# Pool sizing base — será ajustado pelo Control Plane via API
default_pool_size = 5
min_pool_size = 2
max_pool_size = 50

# Queue (backpressure)
max_client_queue_size = 50
max_client_queue_wait_ms = 5000

[pools.shared_cluster.shards.0]
servers = [["yugabytedb", 5433, "primary"]]
database = "cascata_shared"

[pools.shared_cluster.users.0]
username = "cascata_pool_user"
password = "{{ secret:openbao/cascata/yugabytedb/pool_password }}"
pool_size = 5
```

**Nota:** Pools dedicados (STANDARD+) são adicionados dinamicamente pelo Control Plane via admin API do pgcat. O arquivo base contém apenas o pool do cluster compartilhado.

**Ref:** SRS Req-3.5.2, Req-3.5.7

---

### PR-11 — Query Performance Baseline

- [x] **PR-11 — Guidelines de otimização documentados (RLS e Índices)**

**Estratégia de Índices Obrigatórios:**
No provisionamento de qualquer nova tabela de tenant, os seguintes índices DEVEM ser criados automaticamente no YugabyteDB como baseline de performance pelo TableCreator:
1. Índice em `tenant_id` sempre que a tabela pertencer a um modelo de cluster compartilhado (NANO/MICRO).
2. Índice na coluna `created_at` (crítico para range queries temporais, paginação default e audit trail iterativo).
3. Índice em toda coluna que exerça função de chave estrangeira (FK), prevenindo table scans em relacionamentos.
Qualquer omissão nesta listagem configura falha de design.

**A Regra de Ouro do RLS:**
Toda política Row Level Security (RLS) gerada ou injetada pelo sistema está proibida de usar subqueries ou joins. Mapeamentos devem ser resolvidos EXCLUSIVAMENTE via leitura das variáveis da sessão injetadas pelo RLS Handshake (`current_setting()`).
- **Correto:** `tenant_id = nullif(current_setting('request.jwt.claim.tenant_id', true), '')::uuid`
- **Absolutamente Proibido:** `(SELECT id FROM cascata_cp.tenants WHERE slug = current_user)`
Sem esta diretriz, o planner do banco avalia a subquery para cada linha varrida, inutilizando operações complexas nos Tiers de menor escalonamento.

**Ref:** SRS Req-2.3.3, SAD §Schema Metadata

---

Relatório forense da Fase 0 atestando as implementações concluídas das PR-1 a PR-10 com conformidade detalhada cruzada frente às normas arquiteturais do sistema Cascata (SAD, SRS e TASK). Para mais informações consulte "/home/cocorico/projetossz/cascataV1/relatorio_forense_fase0.md


## 0.1 — Connection Pooling

> **Criticidade:** Sem pooling, o YugabyteDB satura sob concorrência real. 200 tenants × 5 requests = 1000 conexões × 8MB = 8GB só em handles. Com pooling: ~185MB. Inviável operar sem isso.

### Referências cruzadas
- **SRS:** Req-3.5.1 a Req-3.5.12 (seção inteira de Connection Pooling)
- **SAD:** §B — Connection Pooling: pgcat + YSQL Connection Manager
- **Roadmap:** Item 0.1

### Implementação

#### Camada 1 — pgcat (Connection Router Externo)

- [X] **0.1.1 — pgcat: Deploy e configuração base**
  - pgcat rodando no Docker Compose modo Shelter
  - Configuração em `pool_mode = transaction` (único modo compatível com RLS Handshake)
  - **Ref:** SRS Req-3.5.2
  
- [X] **0.1.2 — pgcat: Server Reset Query obrigatória**
  - Após cada COMMIT/ROLLBACK, antes de devolver conexão ao pool:
    ```sql
    RESET ROLE;
    SELECT set_config('request.jwt.claim.sub', '', false);
    SELECT set_config('request.jwt.claim.tenant_id', '', false);
    ```
  - Essa limpeza garante que nenhum estado de sessão de um tenant vaza para outra transação
  - A limpeza é executada pelo pgcat — inviolável independente do código da aplicação
  - **Ref:** SRS Req-3.5.2 (Server Reset Query)

- [/] **0.1.3 — pgcat: Pool por instância de banco**
  - NANO/MICRO → pool do cluster compartilhado
  - STANDARD/ENTERPRISE/SOVEREIGN → pool da instância dedicada (provisionada posteriormente)
  - Control Plane injeta pool correto baseado no tenant identificado no JWT
  - **Ref:** SRS Req-3.5.2 (Pool por instância)

- [/] **0.1.4 — pgcat: Read/Write Splitting**
  - Query parser analisa cada query antes de execução
  - `SELECT` fora de transação explícita → roteado para réplica de leitura
  - Impacto: dobra capacidade de throughput de leitura (80-90% do tráfego é read)
  - **Ref:** SRS Req-3.5.2 (Read/Write Splitting)

- [/] **0.1.5 — pgcat: Prepared Statements Cache**
  - Cache de prepared statements por conexão de servidor
  - Resolve incompatibilidade histórica entre prepared statements e transaction pooling
  - **Ref:** SRS Req-3.5.2 (Prepared Statements Cache)

#### Camada 2 — YSQL Connection Manager (Multiplexer Nativo)

- [X] **0.1.6 — YSQL CM: habilitação no YugabyteDB**
  - Habilitar o YSQL Connection Manager na configuração do YugabyteDB
  - Verificar compatibilidade com `SET LOCAL ROLE` e `set_config()` dentro de transações
  - Validar que o RLS Handshake `BEGIN → SET LOCAL → query → COMMIT` funciona sem workaround
  - **Ref:** SRS Req-3.5.3

- [X] **0.1.7 — YSQL CM: multiplexing de conexões físicas**
  - 200 conexões lógicas (pgcat) → 15-20 conexões físicas reais (YSQL CM → YugabyteDB)
  - Economia: ~640MB de RAM por 100 tenants ativos
  - RAM liberada converte em cache de páginas do YugabyteDB → menos I/O → menos latência
  - **Ref:** SRS Req-3.5.3 (Multiplexing)

#### Camada 3 — mTLS em toda a cadeia

- [/] **0.1.8 — mTLS: Pingora ↔ pgcat ↔ YSQL CM ↔ YugabyteDB**
  - CA raiz do projeto armazenada no OpenBao
  - Certificados emitidos pela CA do projeto para cada componente
  - Rotação automática antes da expiração sem downtime
  - Zero comunicação em plaintext, incluindo NANO em modo Shelter
  - **Ref:** SRS Req-3.5.4, Req-3.4.2

#### Camada 4 — Resiliência do Pool

- [/] **0.1.9 — Statement e Idle-in-Transaction Timeouts por tier**
  - Configurar no pgcat:
  
  | Tier | Statement Timeout | Idle in Transaction |
  |------|-------------------|---------------------|
  | NANO | 10s | 5s |
  | MICRO | 30s | 15s |
  | STANDARD | 60s | 30s |
  | ENTERPRISE | 300s | 120s |
  | SOVEREIGN | Configurável | Configurável |
  
  - Ao encerrar por timeout: ROLLBACK → server_reset_query → HTTP 408 → evento Redpanda → ClickHouse
  - **Ref:** SRS Req-3.5.9

- [/] **0.1.10 — Fila limitada com backpressure em 3 estágios**
  - Pool de fila por tenant com 3 estágios:
    - 0-70%: normal, header `X-Cascata-Queue-Depth`
    - 70-90%: timeout reduzido, header `X-Cascata-Queue-Pressure: moderate`, Control Plane notificado
    - \>90%: HTTP 503 imediato, `Retry-After`, alerta crítico ao Control Plane
  - Tamanhos de fila: NANO=50, MICRO=100, STANDARD=500, ENTERPRISE=2000
  - **Ref:** SRS Req-3.5.10

- [/] **0.1.11 — Circuit Breaker por instância de banco**
  - 3 estados: CLOSED → OPEN → HALF-OPEN
  - OPEN: HTTP 503 imediato, notificação ao Pingora via Unix socket, DR Orchestrator acionado
  - HALF-OPEN: probe de conexão com exponential backoff (30s, 60s, 120s)
  - Granularidade: por instância de banco, não por tenant
  - **Ref:** SRS Req-3.5.11

- [/] **0.1.12 — Staggered Pool Warming (anti-thundering herd)**
  - Jitter determinístico por `tenant_id`: `hash(tenant_id) % warming_window_ms`
  - Lead time: NANO=60s, MICRO=90s, STANDARD=120s, ENTERPRISE=180s, SOVEREIGN=configurável
  - Conexões pré-aquecidas antes do primeiro request → latência idêntica a qualquer request
  - **Ref:** SRS Req-3.5.8

- [/] **0.1.13 — Pool Size Adaptativo**
  - Fórmula: `pool_size = max(tier_min, ceil(p95_concurrent_7d × safety_factor × (1 + growth_trend)))`
  - Dados extraídos do ClickHouse, recálculo a cada 24h
  - Aplicado no pgcat via reconfig a quente sem downtime
  - Safety factors: NANO=1.2, MICRO=1.3, STANDARD=1.5, ENTERPRISE=2.0
  - Tier minimums: NANO=3, MICRO=5, STANDARD=10, ENTERPRISE=25
  - **Ref:** SRS Req-3.5.12

#### Camada 5 — Orquestração pelo Control Plane

- [/] **0.1.14 — Control Plane como orquestrador do pgcat**
  - Provisionamento dinâmico: novo tenant STANDARD → pool criado via API do pgcat a quente
  - Ajuste por promoção/downgrade: pool_size, host de destino, configurações de R/W splitting
  - Health monitoring: verificação a cada 10s, fallback degradado via conexão direta ao YSQL CM
  - Eventos de provisionamento registrados no ClickHouse
  - **Ref:** SRS Req-3.5.7

- [/] **0.1.15 — Observabilidade do pipeline de conexão**
  - Métricas Prometheus do pgcat capturadas via OTel → VictoriaMetrics → Dashboard:
    - Conexões ativas por pool/tenant/tier
    - Tempo médio de espera na fila
    - Pool hit rate
    - Read/Write split ratio
    - Server reset query execution time
    - Conexões físicas abertas no YSQL CM
  - **Ref:** SRS Req-3.5.6

#### Camada 6 — Fluxo RLS Handshake completo (referência para implementação)

Este é o fluxo SQL exato que cada request transacional executa através do pool:

```
1. Request chega ao Pingora
2. Pingora extrai JWT, valida, extrai claims (sub, tenant_id, role)
3. Pingora faz ABAC check via Cedar
4. Pingora envia request ao pgcat

5. pgcat obtém conexão do pool (ou enfileira se pool cheio)

6. Na conexão obtida, Pingora executa o RLS Handshake:
   BEGIN;
   SET LOCAL ROLE cascata_api_role;
   SELECT set_config('request.jwt.claim.sub', '{user_id}', true);
   SELECT set_config('request.jwt.claim.tenant_id', '{tenant_id}', true);
   SELECT set_config('request.jwt.claim.role', '{user_role}', true);
   
   -- A query real do tenant (INSERT, SELECT, UPDATE, DELETE)
   {query_do_tenant};
   
   COMMIT;  -- ou ROLLBACK em caso de erro

7. pgcat executa Server Reset Query ANTES de devolver conexão:
   RESET ROLE;
   SELECT set_config('request.jwt.claim.sub', '', false);
   SELECT set_config('request.jwt.claim.tenant_id', '', false);
   SELECT set_config('request.jwt.claim.role', '', false);

8. Conexão devolvida ao pool — limpa para o próximo tenant
```

**ATENÇÃO:** O `true` no passo 6 (`set_config(..., true)`) significa "local à transação" — o valor desaparece no COMMIT. O `false` no passo 7 é redundante mas é a segunda linha de defesa: limpa explicitamente qualquer estado que por alguma razão tenha sobrevivido ao COMMIT. Essa dupla proteção é intencional e obrigatória.

- [X] **0.1.16 — ClickHouse: tabelas de auditoria do pipeline de conexão**

Arquivo: `infra/docker/clickhouse/init/002_pool_events.sql`

```sql
-- Eventos de pool (timeout, circuit breaker, backpressure)
CREATE TABLE IF NOT EXISTS cascata_logs.pool_events (
    timestamp         DateTime64(3),
    tenant_id         String,
    tier              LowCardinality(String),
    event_type        LowCardinality(String),
    -- 'statement_timeout', 'idle_tx_timeout', 'queue_pressure_moderate',
    -- 'queue_pressure_critical', 'circuit_breaker_open', 'circuit_breaker_close',
    -- 'pool_resize', 'pool_warmup', 'connection_error'
    pool_id           String,
    query_hash        Nullable(String),
    duration_ms       Nullable(UInt32),
    queue_depth       Nullable(UInt16),
    queue_capacity    Nullable(UInt16),
    details           String  -- JSON com contexto adicional
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (tenant_id, timestamp)
TTL timestamp + INTERVAL 90 DAY;
```

**Ref:** SRS Req-3.5.6, Req-3.5.9, Req-3.5.10, Req-3.5.11
- [/] 200 tenants NANO/MICRO simultâneos com 5 requests cada operando em ~185MB de RAM de pooling
- [/] RLS Handshake completo (`BEGIN → SET LOCAL ROLE → set_config → query → COMMIT`) funciona sem vazamento
- [X] Server reset query limpa 100% do estado de sessão entre transações
- [/] Read/Write splitting roteia SELECTs para réplica automaticamente
- [/] mTLS em toda a cadeia (Pingora → pgcat → YSQL CM → YugabyteDB)
- [/] Circuit breaker isola falha de uma instância sem afetar outras
- [/] Métricas de pool visíveis no pipeline de observabilidade

---

## 0.2 — Egress Filtering para Functions (REMOVEMOS NÃO TEMOS MAIS EDGE FUNCTIONS NO PROJETO)


## 0.3 — Vulnerability Scanning de Functions (REMOVEMOS NÃO TEMOS MAIS EDGE FUNCTIONS NO PROJETO)


## 0.4 — Soft Delete / Recycle Bin

> **Criticidade:** O schema de storage de metadados de tabelas precisa ser definido com suporte a soft delete desde o início. Adicionar depois exige migração de todas as tabelas existentes e de todos os artefatos dependentes (TableCreator, APIDocs, SDK type generator, MCP Server, Protocol Cascata).

### Referências cruzadas
- **SRS:** Req-2.17.1 a Req-2.17.7 (seção inteira de Soft Delete)
- **SAD:** ADR — Soft Delete Storage: Schema `_recycled` dedicado
- **Roadmap:** Item 0.4

### Implementação

#### Schema `_recycled`

- [X] **0.4.1 — Criar schema `_recycled` no YugabyteDB**
  - Schema dedicado dentro do banco de cada projeto (já incluído no PR-9 `tenant_init.sql`)
  - Separação física de tabelas ativas e recicladas
  - pgcat bloqueia acesso direto ao schema `_recycled` no nível de roteamento
  - Já incluído no PR-9: `REVOKE ALL ON SCHEMA _recycled FROM cascata_api_role, cascata_anon_role`
  - **Ref:** SRS Req-2.17.1

- [X] **0.4.2 — Operação de reciclagem (soft delete)**

  SQL exato executado pelo Control Plane (transação atômica):
  ```sql
  -- Exemplo: tenant deleta tabela "pedidos" do schema "public"
  -- timestamp_unix = 1735689600, hash_curto = primeiros 4 chars do MD5 do table_id
  
  BEGIN;
  
  -- 1. Obter contagem de linhas para audit
  SELECT count(*) AS row_count FROM public.pedidos;
  
  -- 2. Suspender foreign keys que referenciam esta tabela
  -- (queries de descoberta contra information_schema)
  SELECT tc.constraint_name, tc.table_schema, tc.table_name
  FROM information_schema.table_constraints tc
  JOIN information_schema.constraint_column_usage ccu
    ON tc.constraint_name = ccu.constraint_name
  WHERE ccu.table_name = 'pedidos'
    AND tc.constraint_type = 'FOREIGN KEY';
  -- Para cada FK encontrada:
  ALTER TABLE {referencing_table} DROP CONSTRAINT {fk_name};
  -- (FK salva no metadata para reativação em restauração)
  
  -- 3. Mover tabela para schema _recycled
  ALTER TABLE public.pedidos SET SCHEMA _recycled;
  
  -- 4. Renomear com timestamp e hash
  ALTER TABLE _recycled.pedidos
    RENAME TO pedidos__1735689600__a3f7;
  
  -- 5. Aplicar RLS DENY ALL na tabela reciclada
  ALTER TABLE _recycled.pedidos__1735689600__a3f7 ENABLE ROW LEVEL SECURITY;
  ALTER TABLE _recycled.pedidos__1735689600__a3f7 FORCE ROW LEVEL SECURITY;
  CREATE POLICY deny_all ON _recycled.pedidos__1735689600__a3f7
    FOR ALL USING (false);
  
  COMMIT;
  ```
  - Control Plane atualiza `cascata_cp.table_metadata`: status='recycled', preenche campos de soft delete
  - Cache DragonflyDB invalidado para este tenant
  - Evento publicado no Redpanda → ClickHouse
  - **Ref:** SRS Req-2.17.1, Req-2.17.2

- [X] **0.4.3 — Protocol Cascata: análise de impacto antes do delete**

  Queries SQL de descoberta executadas ANTES do modal de confirmação:
  ```sql
  -- 1. Foreign keys que apontam para esta tabela
  SELECT tc.constraint_name, tc.table_schema, tc.table_name, kcu.column_name
  FROM information_schema.table_constraints tc
  JOIN information_schema.key_column_usage kcu
    ON tc.constraint_name = kcu.constraint_name
  JOIN information_schema.constraint_column_usage ccu
    ON tc.constraint_name = ccu.constraint_name
  WHERE ccu.table_name = '{tabela_alvo}'
    AND tc.constraint_type = 'FOREIGN KEY';
  
  -- 2. Computed columns que referenciam colunas desta tabela
  SELECT column_name, computation->>'expression' AS expr
  FROM cascata_cp.column_metadata
  WHERE table_meta_id IN (
    SELECT id FROM cascata_cp.table_metadata
    WHERE tenant_id = '{tenant_id}' AND status = 'active'
  )
  AND computation IS NOT NULL
  AND computation->>'expression' LIKE '%{tabela_alvo}%';
  ```
  - Resultado: JSON com `{ "foreign_keys": [...], "computed_deps": [...] }`
  - Modal de confirmação no painel listando todos os impactos
  - Foreign keys automaticamente suspensas (não removidas) durante reciclagem
  - **Ref:** SRS Req-2.17.3

- [X] **0.4.4 — Retenção por tier e purge scheduler**
  
  | Tier | Default | Máximo |
  |------|---------|--------|
  | NANO | 7 dias | 7 dias (fixo) |
  | MICRO | 14 dias | 30 dias |
  | STANDARD | 30 dias | 90 dias |
  | ENTERPRISE | 90 dias | 365 dias |
  | SOVEREIGN | 365 dias | Ilimitado |
  
  - Control Plane executa job diário (via pg_cron no banco do CP):
    ```sql
    -- Job de purge automático
    SELECT id, tenant_id, table_name, recycled_name
    FROM cascata_cp.table_metadata
    WHERE status = 'recycled'
      AND scheduled_purge_at <= now();
    -- Para cada resultado: executa purge permanente (0.4.6)
    ```
  - Notificação via Central de Comunicação 48h antes de purge automático
  - Countdown visual na interface Recycle Bin
  - **Ref:** SRS Req-2.17.4

- [X] **0.4.5 — Restauração**

  SQL exato executado pelo Control Plane (transação atômica):
  ```sql
  BEGIN;
  
  -- 1. Verificar se existe tabela com mesmo nome no schema original
  SELECT EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema = 'public' AND table_name = 'pedidos'
  ) AS name_conflict;
  
  -- Se name_conflict = true:
  --   Opção A: ALTER TABLE _recycled.pedidos__1735689600__a3f7
  --            RENAME TO pedidos_restored_1735700000;
  --   Opção B: DROP TABLE public.pedidos; (requer senha do operador)
  
  -- 2. Remover política DENY ALL
  DROP POLICY deny_all ON _recycled.pedidos__1735689600__a3f7;
  
  -- 3. Renomear de volta ao nome original
  ALTER TABLE _recycled.pedidos__1735689600__a3f7
    RENAME TO pedidos;
  
  -- 4. Mover de volta para schema original
  ALTER TABLE _recycled.pedidos SET SCHEMA public;
  
  -- 5. Reativar foreign keys suspensas
  -- (lidas do campo suspended_fkeys do metadata)
  ALTER TABLE {referencing_table}
    ADD CONSTRAINT {fk_name} FOREIGN KEY ({col}) REFERENCES public.pedidos({col});
  
  COMMIT;
  ```
  - Control Plane atualiza metadata: status='active', limpa campos de soft delete
  - Cache DragonflyDB invalidado
  - SDK e MCP Server voltam a enxergar a tabela imediatamente
  - **Ref:** SRS Req-2.17.5

- [X] **0.4.6 — Purge permanente**

  SQL exato (executado APÓS 3 camadas de confirmação):
  ```sql
  -- 3 camadas de confirmação (no painel, antes de chegar aqui):
  -- 1. Operador digita nome da tabela
  -- 2. Autenticação adicional (senha ou Passkey verificado pelo Control Plane)
  -- 3. Confirmação final com checkbox "Entendo que esta ação é irreversível"
  
  BEGIN;
  
  -- Obter contagem final para audit
  SELECT count(*) AS final_row_count
  FROM _recycled.pedidos__1735689600__a3f7;
  
  -- Remover foreign keys suspensas permanentemente
  -- (apenas log — as FKs já foram dropadas na reciclagem)
  
  -- DROP irreversível
  DROP TABLE _recycled.pedidos__1735689600__a3f7;
  
  -- Remover metadata do Control Plane
  DELETE FROM cascata_cp.table_metadata WHERE id = '{table_meta_id}';
  DELETE FROM cascata_cp.column_metadata WHERE table_meta_id = '{table_meta_id}';
  
  COMMIT;
  ```
  - Registro **imutável** no ClickHouse (nunca pode ser deletado):
  ```sql
  -- Inserido no ClickHouse após o purge
  INSERT INTO cascata_logs.purge_audit (
    timestamp, tenant_id, executor_id, executor_type,
    table_name, original_schema, row_count_destroyed,
    confirmation_method, ip_address
  ) VALUES (...)
  ```
  - **Ref:** SRS Req-2.17.6

- [X] **0.4.7 — ClickHouse: tabela de auditoria de purge**

  Arquivo: `infra/docker/clickhouse/init/004_purge_audit.sql`
  ```sql
  CREATE TABLE IF NOT EXISTS cascata_logs.purge_audit (
      timestamp             DateTime64(3),
      tenant_id             String,
      executor_id           String,
      executor_type         LowCardinality(String),  -- 'member','agent','system_auto'
      table_name            String,
      original_schema       String,
      row_count_destroyed   UInt64,
      confirmation_method   LowCardinality(String),  -- 'password','passkey'
      ip_address            String,
      purge_type            LowCardinality(String)   -- 'manual','auto_retention'
  ) ENGINE = MergeTree()
  PARTITION BY toYYYYMM(timestamp)
  ORDER BY (tenant_id, timestamp)
  -- SEM TTL — registros de purge são imutáveis e permanentes
  SETTINGS storage_policy = 'default';
  ```

#### Schema Metadata para Soft Delete

- [X] **0.4.8 — Campos de metadata para tabelas recicladas**
  - Já definidos na DDL do PR-8 (tabela `cascata_cp.table_metadata`)
  - Campos de soft delete preenchidos apenas quando `status = 'recycled'`:
    - `original_name`, `original_schema`, `recycled_name`
    - `deleted_at`, `deleted_by`, `scheduled_purge_at`
    - `row_count_at_deletion`, `suspended_fkeys`, `impact_analysis`
  - Cacheado no DragonflyDB por projeto (key: `meta:{tenant_id}:tables`)
  - Cache invalidado em toda operação de recycle/restore/purge
  - **Ref:** SRS Req-2.17.7, SAD §Schema Metadata

### Critérios de aceitação 0.4
- [X] Tabela deletada → movida para `_recycled` com nome correto, dados e índices intactos
- [/] Tabela reciclada → invisível via SDK, MCP Server, e queries ao schema `public`
- [X] Protocol Cascata identifica FKs e RLS policies impactadas antes do delete
- [X] Restauração funciona atomicamente, inclusive com conflito de nome
- [X] Purge permanente exige 3 camadas de confirmação e gera registro imutável no ClickHouse
- [X] Schema metadata tem campo `status` desde a criação inicial

---

## 0.5 — Computed / Virtual Columns

> **Criticidade:** A estrutura interna do schema metadata (usada pelo TableCreator, APIDocs, SDK type generator, MCP Server) precisa saber desde o início que uma coluna pode ser virtual. Adicionar depois quebra o type generator e o MCP schema introspection.

### Referências cruzadas
- **SRS:** Req-2.18.1 a Req-2.18.4 (seção inteira)
- **SAD:** ADR — Computed Columns banco: PostgreSQL Generated Columns STORED; API: avaliador Rust no Pingora
- **Roadmap:** Item 0.5

### Implementação

#### Stored Generated Columns (camada de banco)

- [/] **0.5.1 — Suporte a Generated Columns no YugabyteDB**
  - Verificar compatibilidade de `GENERATED ALWAYS AS ... STORED` no YugabyteDB
  - Expressões permitidas: aritméticas, string, data, concatenação
  - Validação da expressão em tempo de criação
  - **Ref:** SRS Req-2.18.1

- [/] **0.5.2 — Interface de criação no painel (TableCreator)**
  - Campo "tipo de coluna": regular | stored_generated
  - Editor de expressão com validação em tempo real
  - Preview do valor calculado com dados de exemplo
  - **Ref:** SRS Req-2.18.1

#### API Computed Columns (camada Pingora)

- [X] **0.5.3 — Avaliador de expressões Rust no Pingora**
  - Avaliador seguro — sem eval de código arbitrário
  - Acesso permitido: claims do JWT (`jwt.sub`, `jwt.role`), valores de colunas na mesma linha
  - O valor é injetado na resposta antes de retornar ao cliente
  - Marcada na APIDocs como `computed: api` (não filtrável, não ordenável)
  - **Ref:** SRS Req-2.18.2, SAD ADR — Computed Columns API

#### Schema Metadata para Computed Columns

- [X] **0.5.4 — Campos de metadata para computed columns**
  - Campo `computation` no schema metadata desde o início:
    ```json
    {
      "column_name": "total_price",
      "type": "numeric",
      "is_writable": false,
      "computation": {
        "kind": "stored_generated | api_computed",
        "expression": "quantity * unit_price",
        "layer": "database | api",
        "jwt_claims_required": [],
        "filterable": true/false,
        "sortable": true/false
      }
    }
    ```
  - SDK type generator: colunas computed tipadas como read-only
  - MCP Server: não expõe computed columns como campos escritáveis
  - **Ref:** SRS Req-2.18.3, SAD §Schema Metadata

- [X] **0.5.5 — Protocol Cascata para computed columns**
  - Antes de renomear/deletar coluna referenciada em expressão de computed column → detectar e alertar
  - Modal de impacto: lista todas as colunas computed que dependem da coluna alvo
  - Operador deve resolver dependências antes do delete ser permitido
  - **Ref:** SRS Req-2.18.4

### Critérios de aceitação 0.5
- [/] Stored Generated Column funciona no YugabyteDB com expressões válidas
- [X] API Computed Column retorna valor calculado na resposta sem existir fisicamente no banco
- [X] Schema metadata tem campo `computation` desde a criação
- [/] SDK type generator marca computed columns como read-only
- [X] Protocol Cascata detecta dependências de computed columns antes de deletar coluna referenciada

---

## 0.6 — Data Validation Rules na Camada API

> **Criticidade:** Validações definidas no Pingora precisam ser descritas no schema metadata — o mesmo usado pelo SDK type generator e APIDocs. Se o schema metadata não tiver esse campo desde o início, adicionar depois quebra a geração de tipos.

### Referências cruzadas
- **SRS:** Req-2.19.1 a Req-2.19.7 (seção inteira)
- **SAD:** §Validation Engine (Pingora), ADR — Data Validation: Pingora antes de query ao banco
- **Roadmap:** Item 0.6

### Implementação

#### Validation Engine no Pingora

- [/] **0.6.1 — Engine de validação em Rust**
  - Módulo Rust executado no Pingora para toda request de escrita
  - Carrega validações do projeto a partir do cache DragonflyDB (invalidado em mudanças de schema)
  - Ordem de execução:
    1. `required` — campos obrigatórios presentes
    2. `type` — tipos corretos
    3. `regex`, `range`, `length`, `enum` — validações por campo
    4. `cross_field` — expressões entre campos (avaliador Rust como em 0.5.3)
    5. `jwt_context` — validações com claims do JWT
    6. `unique_soft` — queries de unicidade ao YugabyteDB (apenas quando 1-5 passam)
  - Falha em qualquer etapa → HTTP 422 imediatamente, sem executar etapas posteriores
  - Etapas 2-5 coletam todas as violações antes de retornar
  - **Ref:** SRS Req-2.19.1, SAD §Validation Engine

- [/] **0.6.2 — Cross-field validation**
  - Avaliador de expressões Rust com acesso ao payload completo da request
  - Zero eval de código arbitrário, zero injeção possível
  - Expressões: `end_date > start_date`, `total == quantity * unit_price`, `password == password_confirmation`
  - **Ref:** SRS Req-2.19.2

- [/] **0.6.3 — JWT Context Validation**
  - Validações que dependem de quem escreve:
    - `jwt.role IN ["admin", "manager"]` → apenas certos papéis podem escrever
    - `value <= jwt.claims.credit_limit` → valor não excede limite do usuário
    - `owner_id == jwt.sub` → apenas próprio usuário cria registros com seu ID
  - **Ref:** SRS Req-2.19.3

- [/] **0.6.4 — Resposta de erro estruturada**
  - HTTP 422 com payload:
    ```json
    {
      "error": "validation_failed",
      "violations": [
        {
          "field": "email",
          "rule": "regex",
          "message": "Formato de email inválido",
          "value_received": "nao-e-um-email"
        }
      ]
    }
    ```
  - Múltiplas violações na mesma resposta
  - SDK tipifica como `CascataValidationError` com array de `violations` type-safe
  - **Ref:** SRS Req-2.19.5

- [/] **0.6.5 — Tradução de erros do YugabyteDB**
  - Se validação Pingora passa mas constraint do banco falha → capturar erro PostgreSQL
  - Traduzir para mesmo formato `CascataValidationError`
  - **O tenant nunca vê mensagens de erro cruas do PostgreSQL — em nenhuma circunstância**
  - **Ref:** SRS Req-2.19.7

#### Schema Metadata para Validações

- [/] **0.6.6 — Campo `validations` no schema metadata**
  - Array de regras ordenadas por coluna:
    ```json
    {
      "column_name": "email",
      "type": "text",
      "validations": [
        {
          "type": "required",
          "message": "Email é obrigatório",
          "severity": "error"
        },
        {
          "type": "regex",
          "pattern": "^[a-zA-Z0-9._%+-]+@...",
          "message": "Formato de email inválido",
          "severity": "error"
        }
      ]
    }
    ```
  - Severidade: `error` (bloqueia) ou `warning` (escreve + aviso no header `X-Cascata-Warnings`)
  - **Ref:** SRS Req-2.19.4

- [/] **0.6.7 — Exposição no SDK como constraints em compile time**
  - SDK type generator lê regras de validação e gera tipos TypeScript restritivos:
    - `required` → campo não-opcional no tipo de INSERT
    - `enum` → union type literal
    - `range` → tipo numérico (comentário de IDE indica range válido)
  - **Ref:** SRS Req-2.19.6

#### Interface no painel

- [/] **0.6.8 — Editor de validações no painel por coluna**
  - Configurar regras por coluna: tipo de validação, parâmetros, mensagem customizada, severidade
  - Preview: "teste esta validação com valor X → resultado esperado"
  - **Ref:** SRS Req-2.19.1

 
- [X] **0.6.9 — Tradutor de erros PostgreSQL para contexto de extensões**

  O `pg_translator.rs` no Pingora (ou `pg_translator.go` no Control Plane para operações administrativas) precisa cobrir os erros específicos que `CREATE EXTENSION` e `DROP EXTENSION` produzem:

  ```go
  // control-plane/internal/extensions/pg_translator.go
  // Complementa o translator do Pingora (gateway) para operações administrativas do CP

  var extensionErrorMap = map[string]TranslationRule{
      // CREATE EXTENSION falhou por permissão
      "ERROR: permission denied to create extension": {
          HTTPStatus: 403,
          Code:       "extension_permission_denied",
          Message:    "Permissão insuficiente para habilitar a extensão \"%s\". Entre em contato com o suporte.",
      },
      // Extensão não existe na imagem
      "ERROR: could not open extension control file": {
          HTTPStatus: 422,
          Code:       "extension_not_available",
          Message:    "A extensão \"%s\" não está disponível na imagem atual do cluster. Verifique o Extension Marketplace.",
      },
      // Conflito de versão
      "ERROR: extension \"%s\" has no installation script": {
          HTTPStatus: 422,
          Code:       "extension_version_conflict",
          Message:    "Conflito de versão ao instalar \"%s\". Reporte este problema ao suporte.",
      },
      // Dependência de outra extensão não instalada
      "ERROR: required extension \"%s\" is not installed": {
          HTTPStatus: 422,
          Code:       "extension_dependency_missing",
          Message:    "A extensão \"%s\" requer que \"%s\" esteja habilitada primeiro.",
      },
      // DROP com objetos dependentes (caso o CASCADE não resolver tudo)
      "ERROR: cannot drop extension": {
          HTTPStatus: 422,
          Code:       "extension_has_dependents",
          Message:    "A extensão possui objetos dependentes que impedem a remoção. Verifique os impactos no painel.",
      },
  }
  ```

  Esses erros são capturados no handler do item 0.8.3 e 0.8.5, traduzidos antes de qualquer retorno ao cliente. **O tenant nunca vê mensagem crua do PostgreSQL — em nenhuma circunstância**, incluindo operações administrativas de extensão.

  **Ref:** SRS Req-2.19.7

---

- [X] **0.6.10 — CascataExtensionDependencyError como tipo nativo**

  O erro de "extensão tem dependentes" (retornado pelo item 0.8.5) deve ser um tipo Go estruturado que o SDK e o painel sabem interpretar:

  ```go
  // control-plane/internal/extensions/errors.go

  type CascataExtensionDependencyError struct {
      Extension    string
      Dependencies []ExtensionDependency
      Message      string
  }

  type ExtensionDependency struct {
      ObjectType string  // 'table', 'function', 'type', 'view'
      ObjectName string  // nome do objeto dependente
      DepType    string  // 'n' = normal, 'a' = auto
  }

  func (e *CascataExtensionDependencyError) Error() string {
      return e.Message
  }

  // Serialização para resposta HTTP — mesmo envelope do CascataValidationError
  func (e *CascataExtensionDependencyError) ToHTTP() map[string]interface{} {
      return map[string]interface{}{
          "error": "extension_has_dependents",
          "message": e.Message,
          "violations": buildViolationsFromDeps(e.Dependencies),
          // violations segue o mesmo formato de CascataValidationError
          // para que o SDK e o painel tratem com o mesmo handler
      }
  }
  ```

  O painel recebe `error: "extension_has_dependents"` e renderiza o modal de impacto com a lista de objetos — sem lógica especial, pois já tem o handler de `violations` do item 0.6.4.

  **Ref:** SRS Req-2.19.5 (formato estruturado de erros)

### Critérios de aceitação 0.6
- [ ] Request de escrita com violação → HTTP 422 com payload estruturado antes de query ao banco
- [ ] Cross-field e JWT validation funcionam com expressões seguras
- [ ] Múltiplas violações coletadas e retornadas na mesma resposta
- [ ] Erros do YugabyteDB traduzidos para `CascataValidationError`
- [ ] Schema metadata tem campo `validations` desde a criação
- [/] SDK type generator gera tipos restritivos baseados nas validações

---

## 0.7 — Downgrade Automático de Tier (Smart Tenant Classification Engine — Bidirecional)

> **Criticidade:** O Control Plane já sabe promover tenants. Sem a direção inversa, o sistema acumula instâncias superprovisionadas indefinidamente — custo de infra crescendo sem justificativa, clusters ocupando recursos que poderiam servir novos tenants. O algoritmo precisa das duas direções para que as decisões de provisionamento reflitam a realidade de uso.

> **Princípio fundamental:** Downgrade **nunca é automático**. O sistema detecta, analisa, propõe, e aguarda aprovação explícita do operador humano (ou de agente autorizado). A execução só acontece após confirmação. A única exceção é hibernação por inatividade prolongada — e mesmo nessa há janela de 7 dias após notificação antes de qualquer ação.

### Referências cruzadas
- **SRS:** Req-2.1.1 (Control Plane governa classificação), Req-3.2.3 (promoção/downgrade transparente)
- **SAD:** §A — Control Plane: Smart Tenant Classification Engine, §B — Connection Pooling: Pool Size Adaptativo
- **Roadmap:** Item 0.7

---

### Implementação

#### 7.1 — Monitoramento contínuo de métricas de consumo

- [X] **0.7.1 — Job de coleta de métricas no Control Plane (Go)**

  O Control Plane executa um job contínuo que consulta o ClickHouse a cada ciclo e mantém um snapshot atualizado por tenant. As métricas monitoradas:

  ```go
  // internal/tenant/classifier.go
  type TenantMetrics struct {
      TenantID                string
      Tier                    string
      // Tráfego
      P95RequestsPerHourLast30d  float64  // p95 de requests/hora nos últimos 30 dias
      P95ConcurrentConnsLast30d  float64  // p95 de conexões simultâneas
      // Dados
      StorageUsedGB              float64
      // Usuários
      ActiveUsersLast30d         int64    // usuários únicos com atividade
      SimultaneousAgentsMax      int64    // max agentes simultâneos ativos
      // Atividade
      LastActivityAt             time.Time
      DaysInactiveConsecutive    int      // dias sem qualquer request
      // Tendência
      GrowthTrendCoefficient     float64  // slope da regressão linear 30d
  }
  ```

  Query ClickHouse executada para compor `TenantMetrics`:
  ```sql
  -- p95 de requests/hora nos últimos 30 dias, por tenant
  SELECT
      tenant_id,
      quantile(0.95)(requests_per_hour) AS p95_req_hour,
      quantile(0.95)(concurrent_connections) AS p95_conns,
      count(DISTINCT user_id) AS active_users,
      max(timestamp) AS last_activity
  FROM cascata_logs.request_metrics
  WHERE timestamp >= now() - INTERVAL 30 DAY
    AND tenant_id = {tenant_id}
  GROUP BY tenant_id;
  ```

  Ciclo de execução: **diário** via job agendado no Control Plane. Snapshot persistido no YugabyteDB do CP em `cascata_cp.tenant_metrics_snapshot`.

  **Ref:** SRS Req-2.1.1



---

#### 7.2 — Thresholds de downgrade por transição de tier

- [X] **0.7.2 — Tabela de thresholds na configuração do Control Plane**

  ```go
  // internal/config/config.go — parte da struct TierConfig
  type DowngradeThresholds struct {
      // Quantos dias consecutivos abaixo dos thresholds para gerar proposta
      ConsecutiveDaysRequired int
      // Thresholds de consumo do tier inferior (tenant abaixo disso é candidato)
      MaxP95RequestsPerHour   float64
      MaxActiveUsers          int64
      MaxStorageGB            float64
      MaxConcurrentConns      float64
  }
  ```

  Valores configurados no `config.yaml` base do Control Plane:

  | Transição | Dias consecutivos | p95 req/h | Usuários ativos | Storage |
  |-----------|-------------------|-----------|-----------------|---------|
  | MICRO → NANO | 30 dias | < 200 | < 50 | < 500MB |
  | STANDARD → MICRO | 30 dias | < 1.000 | < 500 | < 2GB |
  | ENTERPRISE → STANDARD | 45 dias | < 10.000 | < 5.000 | < 50GB |
  | SOVEREIGN | **nunca auto-downgraded** | — | — | — |

  **SOVEREIGN nunca entra no algoritmo de downgrade automático.** Tier SOVEREIGN tem requisito de soberania física que não pode ser inferido por métricas de tráfego. Requer processo formal e aprovação explícita do operador owner.

  **Ref:** SRS Req-2.1.1

---

#### 7.3 — Três níveis de inatividade (precedência sobre thresholds de volume)

- [X] **0.7.3 — Detector de inatividade no classifier.go**

  Inatividade é verificada independentemente dos thresholds de volume. Um tenant com alto storage mas zero requests é inativo — o algoritmo de volume não captaria isso.

  ```go
  // internal/tenant/classifier.go
  func (c *Classifier) evaluateInactivity(m TenantMetrics) InactivityStatus {
      days := m.DaysInactiveConsecutive

      switch {
      case days >= 7 && days < 14:
          return InactivityLevel1  // Notificação, sem ação
      case days >= 14 && days < 30:
          return InactivityLevel2  // Proposta de downgrade criada para operador
      case days >= 30:
          return InactivityLevel3  // Proposta de hibernação + janela de 7 dias
      default:
          return InactivityNone
      }
  }
  ```

  **Nível 1 — 7 a 14 dias sem atividade:**
  - Notificação para o operador no painel (badge na seção de tenants)
  - Email via Central de Comunicação: "O projeto `{nome}` está inativo há {N} dias"
  - Nenhuma ação executada. Apenas visibilidade.

  **Nível 2 — 14 a 30 dias sem atividade:**
  - Segunda notificação ao operador
  - Proposta de downgrade criada automaticamente em `cascata_cp.downgrade_proposals`
  - Badge urgente no painel: proposta aguardando decisão
  - O operador tem até o dia 30 para decidir antes de entrar no Nível 3

  **Nível 3 — Mais de 30 dias sem atividade:**
  - Proposta de hibernação criada (mais agressiva que downgrade de tier)
  - Notificação urgente via Central de Comunicação: "Ação necessária em 7 dias"
  - **Janela de 7 dias:** operador pode intervir antes de qualquer execução automática
  - Se não houver resposta em 7 dias: hibernação executada automaticamente
  - Hibernação = tenant `status = 'hibernated'`, recursos liberados, dados preservados intactos

  **Ref:** SRS Req-2.1.1

---

#### 7.4 — Tabela de propostas no banco do Control Plane

- [X] **0.7.4 — DDL da tabela `downgrade_proposals`**

  Adicionar à migration `001_initial_schema.sql` (ou criar migration `002_downgrade_proposals.sql`):

  ```sql
  -- =====================================================
  -- PROPOSTAS DE DOWNGRADE / HIBERNAÇÃO
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
      reminder_sent_at    TIMESTAMPTZ             -- lembrete 48h antes de auto-execução
  );

  CREATE INDEX idx_downgrade_proposals_tenant ON cascata_cp.downgrade_proposals(tenant_id);
  CREATE INDEX idx_downgrade_proposals_status ON cascata_cp.downgrade_proposals(status);
  CREATE INDEX idx_downgrade_proposals_execution ON cascata_cp.downgrade_proposals(execution_after)
      WHERE status = 'approved';
  ```

---

#### 7.5 — Fluxo formal de proposta e aprovação

- [X] **0.7.5 — Handler de decisão no Control Plane**

  O operador, ao ver uma proposta no painel, tem exatamente quatro ações disponíveis:

  ```go
  // internal/tenant/provisioner.go
  type ProposalDecision string
  const (
      DecisionApprove   ProposalDecision = "approved"   // executa na próxima janela
      DecisionReject    ProposalDecision = "rejected"   // proposta encerrada, tenant permanece no tier atual
      DecisionPostpone  ProposalDecision = "postponed"  // requer nova data (mínimo 7 dias à frente)
      DecisionDelegate  ProposalDecision = "delegate"   // delega a agente autorizado via ABAC
  )
  ```

  **Aprovação:** proposta movida para `status = 'approved'`. Execution window configurada para o próximo período de baixo tráfego do tenant (extraído do ClickHouse). Notificação de 7 dias enviada antes da execução.

  **Rejeição:** proposta encerrada. Nova proposta só pode ser gerada após 60 dias (evitar spam de propostas). Motivo da rejeição registrado.

  **Adiamento:** operador escolhe nova data (mínimo 7 dias à frente). `execution_after` atualizado. Sistema monitora — se tenant voltar a crescer antes da nova data, proposta cancelada automaticamente.

  **Delegação a agente:** operador autoriza um agente específico (com escopo ABAC `tier:manage`) a aprovar ou rejeitar a proposta. O agente recebe a proposta via evento Redpanda e responde com decisão estruturada. Toda ação do agente registrada no audit trail com `decided_by_type = 'agent'`.

  **Ref:** SRS Req-2.1.1

---

#### 7.6 — Comunicação com o operador (7 dias antes da execução)

- [/] **0.7.6 — Notificações via Central de Comunicação**

  Email enviado 7 dias antes da execução de qualquer downgrade aprovado:

  ```
  Assunto: [Cascata] Downgrade agendado: {nome_tenant} — execução em 7 dias

  O downgrade do projeto "{nome_tenant}" de {tier_atual} para {tier_proposto}
  está agendado para {data_execucao}.

  Motivo: {trigger_reason_human_readable}
  Métricas dos últimos 30 dias: {resumo_metricas}
  Impacto: {impact_analysis_human_readable}

  Para cancelar ou adiar: {link_painel_proposta}
  Contestação disponível até: {data_execucao - 48h}
  ```

  Para hibernação por inatividade (Nível 3), a notificação de 7 dias é enviada no momento em que a janela começa — o operador tem exatamente 7 dias para agir antes da execução automática.

  **Ref:** SRS Req-2.1.1 (Central de Comunicação)

---

#### 7.7 — Verificações de proteção antes da execução

- [X] **0.7.7 — Bloqueios obrigatórios no provisioner.go**

  Antes de executar qualquer downgrade aprovado, o Control Plane executa verificação de proteção. Se qualquer condição for verdadeira, o downgrade é **bloqueado** e o operador é notificado com o motivo exato:

  ```go
  // internal/tenant/provisioner.go
  func (p *Provisioner) validateDowngradeSafe(tenantID string, proposedTier string) error {

      // BLOQUEIO 1: SLA ativo
      // Tier proposto não oferece SLA e tenant tem SLA contratado?
      if tenant.HasActiveSLA && !tierHasSLA(proposedTier) {
          return ErrDowngradeBlockedActiveSLA
      }

      // BLOQUEIO 2: Audit trail imutável
      // ENTERPRISE/SOVEREIGN têm audit trail imutável — tier proposto tem essa garantia?
      if tierHasImmutableAudit(tenant.CurrentTier) && !tierHasImmutableAudit(proposedTier) {
          return ErrDowngradeBlockedImmutableAudit
      }

      // BLOQUEIO 3: PHI / dados regulados
      // Tenant tem namespace PHI (HIPAA) que requer tier dedicado?
      if tenant.HasPHIData && !tierHasDedicatedNamespace(proposedTier) {
          return ErrDowngradeBlockedPHIData
      }

      // BLOQUEIO 4: Extensões exclusivas do tier atual
      // Tenant usa PostGIS (requer imagem :full)?
      // Downgrade para NANO/MICRO usaria imagem :shared sem essas extensões.
      if tierUsesSharedImage(proposedTier) {
          enabledExtensions := p.getEnabledFullOnlyExtensions(tenantID)
          if len(enabledExtensions) > 0 {
              return fmt.Errorf("%w: extensões ativas incompatíveis: %v",
                  ErrDowngradeBlockedExtensions, enabledExtensions)
          }
      }

      // BLOQUEIO 5: Storage excede cota do tier proposto
      if tenant.StorageUsedGB > tierStorageQuota(proposedTier) {
          return fmt.Errorf("%w: storage utilizado (%.1fGB) excede cota do tier %s (%.1fGB)",
              ErrDowngradeBlockedStorage,
              tenant.StorageUsedGB, proposedTier, tierStorageQuota(proposedTier))
      }

      // BLOQUEIO 6: SOVEREIGN — nunca por este caminho
      if tenant.CurrentTier == "SOVEREIGN" {
          return ErrDowngradeBlockedSovereign
      }

      return nil
  }
  ```

  Se um bloqueio é detectado após a proposta já ter sido aprovada (ex: tenant habilitou PostGIS depois de aprovar o downgrade), a execução é cancelada e o operador notificado. A proposta volta para `status = 'pending'` com o novo bloqueio documentado.

  **Ref:** SRS Req-2.1.1, Req-2.20.3

---

#### 7.8 — Execução do downgrade sem downtime

- [/] **0.7.8 — Executor no provisioner.go**

  Execução apenas em janela de baixo tráfego do tenant (< 20% do p95 histórico). Ordem de operações:

  ```go
  // internal/tenant/provisioner.go
  func (p *Provisioner) executeDowngrade(proposal DowngradeProposal) error {
      ctx := context.Background()

      // 1. Verificação final de proteção (última chance)
      if err := p.validateDowngradeSafe(proposal.TenantID, proposal.ProposedTier); err != nil {
          return p.cancelProposal(proposal, err.Error())
      }

      // 2. Atualizar tier no metadata do tenant
      // (Tenant Router já começa a rotear pelo novo tier)
      if err := p.updateTenantTier(ctx, proposal.TenantID, proposal.ProposedTier); err != nil {
          return err
      }

      // 3. Se é downgrade para cluster compartilhado (STANDARD/ENTERPRISE → MICRO/NANO):
      //    Migrar banco dedicado → cluster compartilhado via YugabyteDB live migration
      if requiresMigrationToShared(proposal) {
          if err := p.migrateToSharedCluster(ctx, proposal.TenantID); err != nil {
              // Rollback: restaurar tier anterior
              p.updateTenantTier(ctx, proposal.TenantID, proposal.CurrentTier)
              return err
          }
      }

      // 4. Ajustar pool no pgcat
      //    Novo pool_size baseado nos thresholds do tier proposto
      newPoolConfig := p.calculatePoolConfig(proposal.ProposedTier, proposal.MetricsSnapshot)
      if err := p.pgcatOrchestrator.UpdatePool(ctx, proposal.TenantID, newPoolConfig); err != nil {
          return err
      }

      // 5. Atualizar routing no Tenant Router
      if err := p.tenantRouter.UpdateRoute(ctx, proposal.TenantID, proposal.ProposedTier); err != nil {
          return err
      }

      // 6. Invalidar cache DragonflyDB do tenant
      p.cache.Invalidate(ctx, "tenant:"+proposal.TenantID)

      // 7. Marcar proposta como executada
      p.markProposalExecuted(ctx, proposal.ID)

      // 8. Publicar evento no Redpanda → ClickHouse
      p.events.Publish(ctx, TierChangeEvent{
          TenantID:    proposal.TenantID,
          FromTier:    proposal.CurrentTier,
          ToTier:      proposal.ProposedTier,
          Reason:      "downgrade_approved",
          ProposalID:  proposal.ID,
          ExecutedBy:  proposal.DecidedBy,
          ExecutedAt:  time.Now(),
      })

      return nil
  }
  ```

  Zero downtime — o YugabyteDB live migration mantém o banco acessível durante a migração. O tenant Router atualiza o roteamento atomicamente ao final.

  **Ref:** SAD §2 (Escala Planetária — Control Plane provisiona e migra)

---

#### 7.9 — Snapshot de métricas no banco do Control Plane

- [X] **0.7.9 — DDL da tabela `tenant_metrics_snapshot`**

  ```sql
  -- Adicionar à migration 001 ou criar 002
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
  ```

---

#### 7.10 — Auditoria completa no ClickHouse

- [X] **0.7.10 — Tabela de eventos de tier change**

  Arquivo: `infra/docker/clickhouse/init/005_tier_events.sql`

  ```sql
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
  -- Retenção de 1 ano — decisões de tier são auditoria de negócio, não apenas técnica
  ```

---

### Critérios de aceitação 0.7

- [ ] Control Plane detecta automaticamente tenants abaixo dos thresholds de downgrade por N dias consecutivos
- [ ] Três níveis de inatividade geram ações distintas: notificação (7d) → proposta (14d) → hibernação com janela (30d)
- [ ] SOVEREIGN nunca entra no algoritmo de downgrade — bloqueio hardcoded
- [ ] Proposta criada com snapshot completo de métricas e análise de impacto
- [ ] Operador pode: aprovar / rejeitar / adiar / delegar a agente autorizado
- [ ] Agente com escopo `tier:manage` pode decidir sobre proposta — ação registrada com `decided_by_type = 'agent'`
- [ ] Notificação por email enviada 7 dias antes de qualquer execução
- [ ] Seis bloqueios de proteção impedindo downgrade destrutivo (SLA, audit imutável, PHI, extensões, storage, SOVEREIGN)
- [ ] Execução sem downtime via YugabyteDB live migration + pgcat reconfig a quente
- [ ] Pool ajustado no pgcat após downgrade refletindo o novo tier
- [ ] Tenant Router atualizado atomicamente ao final da migração
- [ ] Evento completo no ClickHouse para cada decisão de tier (aprovada, rejeitada, bloqueada, executada)
- [ ] Tabela `downgrade_proposals` com todos os estados e histórico preservado
- [ ] Tabela `tenant_metrics_snapshot` capturando estado diário por tenant
---


## 0.8 — Extensions Marketplace

> **Criticidade:** Gestão de extensões do YugabyteDB diretamente no painel. Depende das imagens Docker `:shared` e `:full` (PR-3 e PR-4).

### Referências cruzadas
- **SRS:** Req-2.20.1 a Req-2.20.5 (seção inteira de Extensions)
- **SAD:** §Extension Profile System, ADR sobre imagens YugabyteDB
- **Roadmap:** Item 0.8

---

### Implementação

- [X] **0.8.1 — Catálogo de extensões (seed data)**

  Arquivo: `scripts/seed-extensions-catalog.sql`

  ```sql
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
   'SELECT topology.CreateTopology(''minha_topo'', 4326);');

  -- CATEGORIA 4 — Bloqueadas
  INSERT INTO cascata_cp.extensions_catalog
    (name, display_name, category, available_in_shared, available_in_full,
     compat_level, blocked_reason)
  VALUES
  ('pgvector',
   'pgvector', 4, false, false, 'total',
   'Substituído pelo Qdrant — banco vetorial dedicado com isolamento multi-tenant correto via payload filtering nativo, HNSW com quantização (até 32x redução de footprint), DR integrado ao Orchestrator e audit trail no ClickHouse. pgvector em cluster compartilhado não oferece isolamento equivalente.');
  ```

  **Ref:** SRS Req-2.20.2, Req-2.20.5

---

- [X] **0.8.2 — Interface no painel — Extension Marketplace**

  Query que o dashboard usa para renderizar o marketplace:
  ```sql
  SELECT
    ec.name,
    ec.display_name,
    ec.description,
    ec.category,
    ec.compat_level,
    ec.compat_notes,
    ec.usage_snippet,
    ec.blocked_reason,
    ec.storage_impact,
    ec.performance_impact,
    CASE
      WHEN ec.category = 4                                        THEN 'blocked'
      WHEN te.id IS NOT NULL                                      THEN 'enabled'
      WHEN t.image_variant = 'shared' AND NOT ec.available_in_shared THEN 'unavailable'
      ELSE 'available'
    END AS status,
    te.enabled_at,
    te.enabled_by
  FROM cascata_cp.extensions_catalog ec
  CROSS JOIN cascata_cp.tenants t
  LEFT JOIN cascata_cp.tenant_extensions te
    ON te.extension = ec.name AND te.tenant_id = t.id
  WHERE t.id = '{tenant_id}'
  ORDER BY ec.category, ec.display_name;
  ```

  Comportamento por status no painel:
  - `available` → botão "Habilitar" ativo
  - `enabled` → badge verde + botão "Desabilitar"
  - `unavailable` → botão desabilitado + tooltip "Requer instância dedicada (STANDARD+)"
  - `blocked` → ícone de bloqueio + motivo exibido inline, sem botão

  Compat level exibido como badge inline:
  - `total` → sem badge (comportamento esperado)
  - `partial` → badge amarelo "⚠ Compatibilidade parcial" com `compat_notes` em tooltip expandível

  **Ref:** SRS Req-2.20.5

---

- [X] **0.8.3 — Habilitação com tratamento completo de falha**

  Handler Go no Control Plane (`internal/extensions/enabler.go`):

  O padrão de escrita segue **pending → active** — a intenção é registrada antes da execução.
  Se o processo morrer entre as duas operações, o estado `pending` é visível e tratável.
  Nunca existe a situação inversa: extensão ativa no banco sem registro no CP.

  ```go
  func (e *Enabler) Enable(ctx context.Context, tenantID, extensionName, memberID string) error {

      // 1. Verificar disponibilidade no catálogo
      ext, err := e.catalog.Get(ctx, extensionName)
      if err != nil || ext == nil {
          return ErrExtensionNotFound
      }
      if ext.Category == 4 {
          return fmt.Errorf("%w: %s", ErrExtensionBlocked, ext.BlockedReason)
      }

      tenant, err := e.tenants.Get(ctx, tenantID)
      if err != nil {
          return err
      }
      if tenant.ImageVariant == "shared" && !ext.AvailableInShared {
          return ErrExtensionRequiresDedicatedInstance
      }

      // 2. PRIMEIRA ESCRITA — registrar intenção no CP com status 'pending'
      // Se o processo morrer após este ponto, o reconciliador encontra 'pending'
      // e sabe exatamente o que aconteceu — sem estado ambíguo
      record, err := e.repo.InsertTenantExtension(ctx, TenantExtension{
          TenantID:  tenantID,
          Extension: extensionName,
          EnabledBy: memberID,
          Status:    "pending",  // intenção registrada, execução ainda não ocorreu
      })
      if err != nil {
          return err
      }

      // 3. SEGUNDA ESCRITA — executar CREATE EXTENSION no banco do tenant
      // Timeout de 30s — PostGIS na primeira habilitação pode demorar
      extCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
      defer cancel()

      err = e.tenantDB.Exec(extCtx, tenantID,
          fmt.Sprintf(`CREATE EXTENSION IF NOT EXISTS "%s" SCHEMA "%s"`,
              extensionName, tenant.DBSchema))

      if err != nil {
          // CREATE EXTENSION falhou — traduzir erro antes de qualquer retorno
          // O tenant nunca vê mensagem crua do PostgreSQL (padrão seção 0.6.5 + 0.6.9)
          cascataErr := e.pgTranslator.Translate(err, TranslationContext{
              Operation: "extension_enable",
              Extension: extensionName,
              TenantID:  tenantID,
          })

          // Reverter o registro 'pending' — intenção não se concretizou
          // Estado no marketplace volta para 'available' sem rastro órfão
          _ = e.repo.DeleteTenantExtension(ctx, tenantID, record.ID)

          // Publicar evento de falha no Redpanda → ClickHouse
          e.events.Publish(ctx, ExtensionEvent{
              TenantID:      tenantID,
              Extension:     extensionName,
              Action:        "enabled",
              ExecutedBy:    memberID,
              Result:        "failed",
              FailureReason: cascataErr.Message,
              ImageVariant:  tenant.ImageVariant,
              TierAtTime:    tenant.Tier,
          })

          return cascataErr
      }

      // 4. CONFIRMAÇÃO — promover status de 'pending' para 'active'
      // A partir deste ponto o marketplace exibe 'enabled'
      if err := e.repo.ConfirmTenantExtension(ctx, record.ID); err != nil {
          // CREATE EXTENSION executou com sucesso mas a confirmação falhou.
          // O registro 'pending' no CP é suficiente para o reconciliador
          // detectar e promover para 'active' no próximo ciclo (6h).
          // Não é erro crítico — a extensão funciona, apenas o status no painel
          // ainda mostra 'pending' até a reconciliação.
          e.log.Warn("extension_confirmation_failed_reconciler_will_fix",
              "tenant_id", tenantID,
              "extension", extensionName,
              "record_id", record.ID,
              "error", err)
          // Não retornar erro ao cliente — a operação foi bem-sucedida no banco
      }

      // 5. Invalidar cache DragonflyDB do tenant
      e.cache.Invalidate(ctx, "tenant:"+tenantID+":extensions")

      // 6. Publicar evento de sucesso no Redpanda → ClickHouse
      e.events.Publish(ctx, ExtensionEvent{
          TenantID:     tenantID,
          Extension:    extensionName,
          Action:       "enabled",
          ExecutedBy:   memberID,
          Result:       "success",
          ImageVariant: tenant.ImageVariant,
          TierAtTime:   tenant.Tier,
      })

      return nil
  }
  ```

  **O que muda na tabela `cascata_cp.tenant_extensions` para suportar este padrão:**

  ```sql
  -- Adicionar campo status à tabela tenant_extensions (migration 001 ou 002)
  -- Substituir a definição original por esta:

  CREATE TABLE cascata_cp.tenant_extensions (
      id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      tenant_id   UUID NOT NULL REFERENCES cascata_cp.tenants(id),
      extension   TEXT NOT NULL REFERENCES cascata_cp.extensions_catalog(name),
      status      TEXT NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending', 'active')),
      enabled_at  TIMESTAMPTZ,                  -- preenchido apenas quando status='active'
      enabled_by  UUID,
      created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
      UNIQUE(tenant_id, extension)
  );

  -- Índice para o reconciliador encontrar 'pending' rapidamente
  CREATE INDEX idx_tenant_ext_pending
      ON cascata_cp.tenant_extensions(status, created_at)
      WHERE status = 'pending';
  ```

  **Ref:** SRS Req-2.20.3, Req-2.19.7 (zero mensagem crua do PostgreSQL)

---

- [ ] **0.8.4 — Isolamento de extensão por schema + wrapper pg_cron**

  **Isolamento padrão:**
  Toda extensão é criada no schema do tenant — funções e types são objetos do schema, não globais. PostGIS do tenant A não interfere com tenant B.

  **Wrapper obrigatório para pg_cron:**

  pg_cron cria objetos no schema `cron` compartilhado entre todos os tenants do cluster. Exposição direta seria uma violação de isolamento. O Cascata resolve com três camadas:

  **Camada 1 — Bloqueio de acesso direto** (já no `tenant_init.sql`):
  ```sql
  -- Garantir que roles de tenant não acessam schema cron diretamente
  REVOKE ALL ON SCHEMA cron FROM cascata_api_role;
  REVOKE ALL ON SCHEMA cron FROM cascata_anon_role;
  ```

  **Camada 2 — Função wrapper no schema do tenant:**
  ```sql
  -- Criada pelo Control Plane no banco do tenant ao habilitar pg_cron
  -- Arquivo: control-plane/internal/extensions/templates/pg_cron_wrapper.sql

  CREATE OR REPLACE FUNCTION "{tenant_schema}".schedule_job(
      job_name  TEXT,
      schedule  TEXT,   -- cron expression: '0 * * * *'
      command   TEXT    -- SQL a executar
  ) RETURNS BIGINT
  LANGUAGE plpgsql
  SECURITY DEFINER   -- executa com privilégios do owner da função (CP role)
  SET search_path = "{tenant_schema}", public
  AS $$
  DECLARE
      v_job_id BIGINT;
      v_prefixed_name TEXT;
  BEGIN
      -- Prefixar nome com tenant_id para garantir unicidade e isolamento
      v_prefixed_name := '{tenant_id}__' || job_name;

      -- Verificar se já existe job com esse nome para este tenant
      IF EXISTS (
          SELECT 1 FROM cron.job
          WHERE jobname = v_prefixed_name
      ) THEN
          RAISE EXCEPTION 'Job "%" já existe neste projeto', job_name;
      END IF;

      SELECT cron.schedule(v_prefixed_name, schedule, command)
      INTO v_job_id;

      -- Registrar no metadata do Control Plane
      -- (via NOTIFY — Control Plane escuta e persiste em cascata_cp.cron_jobs)
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

  -- Wrapper de listagem: tenant só vê seus próprios jobs
  CREATE OR REPLACE FUNCTION "{tenant_schema}".list_jobs()
  RETURNS TABLE(job_id BIGINT, job_name TEXT, schedule TEXT, active BOOLEAN, last_run TIMESTAMPTZ)
  LANGUAGE sql
  SECURITY DEFINER
  SET search_path = "{tenant_schema}", public
  AS $$
      SELECT
          jobid,
          replace(jobname, '{tenant_id}__', '') AS job_name,
          schedule,
          active,
          last_run_start_time
      FROM cron.job
      WHERE jobname LIKE '{tenant_id}__%';
  $$;

  -- Wrapper de remoção
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
  ```

  **Camada 3 — Metadata de jobs no Control Plane:**
  ```sql
  -- Adicionar à migration 001 ou criar migration 003_cron_jobs.sql
  CREATE TABLE cascata_cp.cron_jobs (
      id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      tenant_id   UUID NOT NULL REFERENCES cascata_cp.tenants(id),
      pg_job_id   BIGINT NOT NULL,        -- ID retornado pelo cron.schedule()
      job_name    TEXT NOT NULL,          -- nome sem prefixo (versão legível pelo tenant)
      schedule    TEXT NOT NULL,
      command     TEXT NOT NULL,
      active      BOOLEAN NOT NULL DEFAULT true,
      created_by  UUID,
      created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
      last_run_at TIMESTAMPTZ,
      UNIQUE(tenant_id, job_name)
  );
  ```

  **Reconciliação Anti-SPOF (Control Plane Boot):**
  Eventos `LISTEN/NOTIFY` são voláteis. Para não perder rastreio caso reinicie, o Control Plane implementa no seu startup um *reconciliador primário de Crons*. Ele não pode confiar apenas no listener; ao iniciar, deve consultar ativamente `cascata_cp.cron_jobs` (ou `cron.job` em cada tenant com permissões administrativas) e reconstruir a visão completa dos cron engines em memória antes de engatar o listener assíncrono. Isso previne o SPOF (Single Point of Failure).

  **Ref:** SRS Req-2.20.4

---

- [ ] **0.8.5 — Desabilitação de extensão com verificação de impacto**

  Handler Go no Control Plane (`internal/extensions/enabler.go`):

  ```go
  func (e *Enabler) Disable(ctx context.Context, tenantID, extensionName, memberID string) error {
      // 1. Verificar objetos dependentes no banco do tenant
      deps, err := e.tenantDB.QueryRows(ctx, tenantID, `
          SELECT
              d.classid::regclass AS object_type,
              CASE d.classid::regclass::text
                  WHEN 'pg_class' THEN c.relname
                  WHEN 'pg_proc'  THEN p.proname
                  ELSE d.objid::text
              END AS object_name,
              d.deptype
          FROM pg_depend d
          LEFT JOIN pg_class c ON d.classid = 'pg_class'::regclass AND d.objid = c.oid
          LEFT JOIN pg_proc p  ON d.classid = 'pg_proc'::regclass  AND d.objid = p.oid
          WHERE d.refclassid = 'pg_extension'::regclass
            AND d.refobjid = (
                SELECT oid FROM pg_extension WHERE extname = $1
            )
            AND d.deptype != 'e'  -- excluir objetos próprios da extensão
      `, extensionName)
      if err != nil {
          return err
      }

      // 2. Se há dependências → retornar lista para modal de impacto no painel
      if len(deps) > 0 {
          return &CascataExtensionDependencyError{
              Extension:    extensionName,
              Dependencies: deps,
              Message: fmt.Sprintf(
                  "A extensão \"%s\" possui %d objeto(s) dependente(s). "+
                  "Remova as dependências antes de desabilitar.",
                  extensionName, len(deps)),
          }
      }

      // 3. Tratamento especial: pg_cron requer remoção dos wrappers antes do DROP
      if extensionName == "pg_cron" {
          if err := e.removePgCronWrappers(ctx, tenantID); err != nil {
              return err
          }
      }

      // 4. DROP EXTENSION
      err = e.tenantDB.Exec(ctx, tenantID,
          fmt.Sprintf(`DROP EXTENSION IF EXISTS "%s" CASCADE`, extensionName))
      if err != nil {
          return e.pgTranslator.Translate(err, TranslationContext{
              Operation: "extension_disable",
              Extension: extensionName,
          })
      }

      // 5. Remover do registro do Control Plane
      if err := e.repo.DeleteTenantExtension(ctx, tenantID, extensionName); err != nil {
          return err
      }

      // 6. Invalidar cache
      e.cache.Invalidate(ctx, "tenant:"+tenantID+":extensions")

      // 7. Publicar evento
      e.events.Publish(ctx, ExtensionEvent{
          TenantID:   tenantID,
          Extension:  extensionName,
          Action:     "disabled",
          ExecutedBy: memberID,
          Result:     "success",
      })

      return nil
  }
  ```

  **Ref:** SRS Req-2.20.3

---

- [X] **0.8.6 — CVE Monitoring com fontes específicas e comportamento definido**

  Handler Go no Control Plane (`internal/extensions/cve_monitor.go`):

  ```go
  // Executado uma vez por dia via job agendado no Control Plane
  // Tolerante a falha: indisponibilidade das APIs não bloqueia operação

  type CVEMonitor struct {
      githubToken string // lido do OpenBao — path: cascata/cve/github_token
      httpClient  *http.Client
      repo        ExtensionRepository
      events      EventPublisher
      log         Logger
  }

  // Mapeamento: nome da extensão → pacote nas bases de CVE
  var extensionAdvisoryMap = map[string][]string{
      "postgis":     {"postgis"},
      "pg_cron":     {"pg_cron"},
      "plpgsql":     {},  // parte do core PostgreSQL — cobertura via PostgreSQL advisory
  }

  func (m *CVEMonitor) Run(ctx context.Context) {
      // 1. Carregar todas as extensões habilitadas em todos os tenants
      activeExtensions, err := m.repo.GetAllActiveExtensions(ctx)
      if err != nil {
          m.log.Error("cve_monitor_failed_to_load_extensions", "error", err)
          return // não propaga — job tenta novamente amanhã
      }

      uniqueExtensions := deduplicateExtensions(activeExtensions)

      for _, extName := range uniqueExtensions {
          advisories, source, err := m.fetchAdvisories(ctx, extName)
          if err != nil {
              // Falha de conectividade: registrar skipped, não bloquear
              m.log.Warn("cve_advisory_fetch_skipped",
                  "extension", extName,
                  "error", err)
              m.events.Publish(ctx, CVECheckEvent{
                  Extension: extName,
                  Result:    "skipped",
                  Reason:    err.Error(),
              })
              continue
          }

          for _, advisory := range advisories {
              m.processAdvisory(ctx, advisory, extName, source, activeExtensions)
          }
      }
  }

  func (m *CVEMonitor) fetchAdvisories(ctx context.Context, extName string) ([]Advisory, string, error) {
      // FONTE PRIMÁRIA: OSV.dev — cobre NVD, GitHub Advisory, e outros numa única API
      // Endpoint: https://api.osv.dev/v1/query
      // Sem autenticação necessária para consultas básicas
      osvAdvisories, err := m.queryOSV(ctx, extName)
      if err == nil {
          return osvAdvisories, "osv.dev", nil
      }
      m.log.Warn("osv_fetch_failed_trying_github", "extension", extName, "error", err)

      // FONTE SECUNDÁRIA: GitHub Advisory Database
      // Rate limit: 60 req/h sem token, 5000/h com token (armazenado no OpenBao)
      // Endpoint: https://api.github.com/advisories
      ghAdvisories, err := m.queryGitHubAdvisory(ctx, extName)
      if err == nil {
          return ghAdvisories, "github_advisory", nil
      }

      // Ambas falharam → retornar erro para tratamento no caller
      return nil, "", fmt.Errorf("todas as fontes de CVE indisponíveis para %s: %w", extName, err)
  }

  func (m *CVEMonitor) processAdvisory(
      ctx context.Context,
      advisory Advisory,
      extName string,
      source string,
      activeExtensions []TenantExtension,
  ) {
      // Filtrar tenants afetados (usam esta extensão)
      affectedTenants := filterTenantsByExtension(activeExtensions, extName)

      for _, te := range affectedTenants {
          // Ação por severidade — não bloqueia extensão, apenas alerta
          switch advisory.Severity {
          case "CRITICAL", "HIGH":
              // Badge vermelho no painel + notificação urgente via Central de Comunicação
              m.createUrgentAlert(ctx, te.TenantID, advisory, extName)

          case "MEDIUM":
              // Badge amarelo no painel — sem notificação ativa
              m.createWarningAlert(ctx, te.TenantID, advisory, extName)

          case "LOW", "INFORMATIONAL":
              // Apenas audit log no ClickHouse — sem badge no painel
          }

          // Sempre publicar no ClickHouse independente da severidade
          m.events.Publish(ctx, ExtensionEvent{
              TenantID:   te.TenantID,
              Extension:  extName,
              Action:     "cve_alert",
              Result:     "success",
              CVEIDs:     advisory.IDs, // JSON array: ["CVE-2024-XXXX", "GHSA-xxxx"]
              // Campos adicionais: severity, source, summary, fix_available
          })
      }
  }
  ```

  Sugestões de ação exibidas no painel por severidade:
  - **CRITICAL/HIGH:** "Atualizar imagem Docker para versão corrigida" + link para changelog + botão "Desabilitar extensão temporariamente"
  - **MEDIUM:** "Monitorar correção disponível" + link para advisory
  - **LOW:** Apenas visível no log de auditoria da extensão

  **Ref:** SRS Req-2.20.5

---

- [X] **0.8.7 — ClickHouse: tabela de auditoria de extensões**

  Arquivo: `infra/docker/clickhouse/init/006_extension_events.sql`

  ```sql
  -- Toda habilitação, desabilitação, falha e alerta de CVE
  -- registrados aqui de forma imutável
  CREATE TABLE IF NOT EXISTS cascata_logs.extension_events (
      timestamp           DateTime64(3),
      tenant_id           String,
      extension_name      LowCardinality(String),
      action              LowCardinality(String),
      -- 'enabled', 'disabled', 'blocked', 'cve_alert',
      -- 'inconsistency_warning', 'cve_check_skipped'
      executed_by         Nullable(String),        -- member_id ou agent_id
      executed_by_type    LowCardinality(Nullable(String)), -- 'member','agent','system'
      tier_at_time        LowCardinality(String),
      image_variant       LowCardinality(String),  -- 'shared','full'
      result              LowCardinality(String),  -- 'success','failed','blocked','skipped'
      failure_reason      Nullable(String),
      cve_ids             Nullable(String),        -- JSON array quando action='cve_alert'
      cve_severity        LowCardinality(Nullable(String)), -- 'CRITICAL','HIGH','MEDIUM','LOW'
      cve_source          LowCardinality(Nullable(String))  -- 'osv.dev','github_advisory'
  ) ENGINE = MergeTree()
  PARTITION BY toYYYYMM(timestamp)
  ORDER BY (tenant_id, timestamp)
  TTL timestamp + INTERVAL 365 DAY;
  -- 1 ano de retenção — extensões em bancos de dados regulados
  -- precisam de trilha de auditoria longa
  ```

  **Ref:** SRS Req-2.5.1, Req-2.5.3 (audit trail imutável ENTERPRISE/SOVEREIGN)

---
- [X] **0.8.8 — Job de reconciliação de estado (extensões)**

  Arquivo: `control-plane/internal/extensions/reconciler.go`

  Executado a cada 6 horas. Com o padrão `pending → active` do item 0.8.3, o reconciliador
  não precisa mais comparar duas fontes de verdade. Ele tem uma única pergunta:
  **existe algum registro `pending` que ficou parado no tempo?**

  Um `pending` com mais de 5 minutos significa que o processo morreu entre a primeira
  e a segunda escrita. O reconciliador verifica o que realmente aconteceu no banco
  do tenant e resolve de forma determinística — sem ambiguidade, sem heurística.

  ```go
  // control-plane/internal/extensions/reconciler.go

  const stalePendingThreshold = 5 * time.Minute

  func (r *Reconciler) ReconcileExtensions(ctx context.Context) {
      stale, err := r.repo.GetStalePendingExtensions(ctx, stalePendingThreshold)
      if err != nil {
          r.log.Error("reconciler_query_failed", "error", err)
          // Falha silenciosa — tentará novamente no próximo ciclo de 6h
          return
      }

      if len(stale) == 0 {
          // Caso normal — nada a fazer
          return
      }

      r.log.Info("reconciler_found_stale_pending", "count", len(stale))

      for _, record := range stale {
          r.reconcileRecord(ctx, record)
      }
  }

  func (r *Reconciler) reconcileRecord(ctx context.Context, record TenantExtension) {
      // Verificar no banco real do tenant se o CREATE EXTENSION chegou a executar
      exists, err := r.tenantDB.ExtensionExists(ctx, record.TenantID, record.Extension)
      if err != nil {
          // Banco do tenant inacessível no momento — não tomar decisão com incerteza
          // O registro permanece 'pending' e será reavaliado no próximo ciclo
          r.log.Warn("reconciler_tenant_db_unreachable",
              "tenant_id", record.TenantID,
              "extension", record.Extension,
              "record_id", record.ID)
          return
      }

      if exists {
          // CREATE EXTENSION executou — processo morreu antes da confirmação
          // Promover para 'active': a extensão funciona, o painel precisa refletir isso
          if err := r.repo.ConfirmTenantExtension(ctx, record.ID); err != nil {
              r.log.Error("reconciler_confirmation_failed",
                  "record_id", record.ID, "error", err)
              return
          }
          r.log.Info("reconciler_promoted_to_active",
              "tenant_id", record.TenantID,
              "extension", record.Extension)
          r.publishReconciliationEvent(ctx, record, "promoted")

      } else {
          // CREATE EXTENSION não executou — processo morreu antes da segunda escrita
          // Limpar registro 'pending': intenção não se concretizou, estado fica limpo
          if err := r.repo.DeleteTenantExtension(ctx, record.TenantID, record.ID); err != nil {
              r.log.Error("reconciler_cleanup_failed",
                  "record_id", record.ID, "error", err)
              return
          }
          r.log.Info("reconciler_cleaned_stale_pending",
              "tenant_id", record.TenantID,
              "extension", record.Extension)
          r.publishReconciliationEvent(ctx, record, "cleaned")
      }
  }

  func (r *Reconciler) publishReconciliationEvent(
      ctx context.Context,
      record TenantExtension,
      outcome string, // "promoted" | "cleaned"
  ) {
      r.events.Publish(ctx, ExtensionEvent{
          TenantID:  record.TenantID,
          Extension: record.Extension,
          Action:    "reconciled",
          Result:    outcome,
          // executed_by_type = 'system' — rastreável no ClickHouse
      })
  }
  ```

  Query usada por `GetStalePendingExtensions`:
  ```sql
  SELECT id, tenant_id, extension, created_at
  FROM cascata_cp.tenant_extensions
  WHERE status = 'pending'
    AND created_at < now() - $1::interval
  ORDER BY created_at ASC;
  -- O índice parcial 'idx_tenant_ext_pending WHERE status = pending'
  -- criado no 0.8.3 garante que esta query nunca faz full scan
  -- mesmo com milhares de extensões ativas no catálogo
  ```

  **Ref:** SRS Req-2.20.3

---

### Critérios de aceitação 0.8

- [X] Marketplace lista extensões corretas por imagem (`:shared` vs `:full`) sem item errado
- [X] Catálogo: 14 Cat.1 + 1 Cat.2 + 4 Cat.3 + 1 Cat.4 — total 20 extensões
- [X] Habilitação bem-sucedida → registro `pending` criado antes do `CREATE EXTENSION`, promovido para `active` após confirmação, evento `result='success'` no ClickHouse
- [X] Falha no `CREATE EXTENSION` → registro `pending` removido, erro traduzido via `CascataValidationError`, marketplace permanece `available`, evento `result='failed'` no ClickHouse
- [X] Extensão bloqueada (pgvector) → motivo exibido inline, zero botão de habilitar
- [X] pg_cron: acesso direto ao schema `cron` bloqueado para todos os roles de tenant
- [X] pg_cron: tenant cria/lista/remove jobs apenas via wrappers — prefixo `tenant_id__` aplicado automaticamente e invisível ao tenant
- [X] Tenant A não visualiza jobs do tenant B via `list_jobs()`
- [X] Desabilitação com dependências → modal de impacto com lista exata de objetos, zero `DROP CASCADE` silencioso
- [X] CVE monitoring: falha de API registra `skipped` no ClickHouse e tenta no ciclo seguinte — nunca bloqueia operação
- [X] CVE CRITICAL/HIGH → badge vermelho no painel + notificação via Central de Comunicação
- [X] CVE MEDIUM → badge amarelo no painel, sem notificação ativa
- [X] CVE LOW → apenas `extension_events` no ClickHouse, sem badge
- [X] Toda ação registrada em `cascata_logs.extension_events` com 1 ano de retenção
- [X] Reconciliador executa a cada 6h via job agendado no Control Plane
- [X] Reconciliador com banco do tenant inacessível → mantém `pending` e tenta no próximo ciclo, sem decisão sob incerteza
- [X] Reconciliador `promoted` → extensão aparece como `enabled` no marketplace após confirmação
- [X] Reconciliador `cleaned` → marketplace volta para `available` sem rastro órfão
- [X] Índice parcial `WHERE status = 'pending'` garante que o job nunca faz full scan em `tenant_extensions`


---

## Checklist de Integração Pós-Fase 0

Após todos os itens 0.x concluídos, verificar a integração completa. Cada item tem comandos de verificação explícitos.

- [ ] **INT-1 — Request transacional end-to-end funciona**
  - Fluxo completo:
    ```
    SDK → Cloudflare → Pingora (JWT + ABAC + Validation)
      → pgcat (connection pool, transaction mode)
        → YSQL CM (multiplexing)
          → YugabyteDB (query real com RLS ativo)
      ← response passa por Pingora (computed columns injetadas)
      ← DragonflyDB (cache atualizado)
      ← Redpanda (evento de audit publicado)
      ← ClickHouse (evento persistido)
    ```
  - Targets: P99 < 5ms (cache miss), P99 < 1ms (cache hit)
  - Verificação:
    ```bash
    # 1. Subir ambiente completo
    cd infra/docker && docker compose -f docker-compose.shelter.yml up -d

    # 2. Verificar saúde de todos os serviços
    docker compose -f docker-compose.shelter.yml ps  # todos 'healthy'

    # 3. Fazer request transacional de teste
    curl -X POST http://localhost:8080/rest/v1/test_table \
      -H "Authorization: Bearer {jwt_test}" \
      -H "apikey: {anon_key}" \
      -d '{"name": "teste_e2e", "email": "teste@cascata.dev"}'

    # 4. Verificar que RLS Handshake não vazou claims
    # (Server Reset Query executou entre transações)
    docker exec pgcat psql -c "SHOW request.jwt.claim.sub;"  # deve retornar vazio

    # 5. Verificar evento no ClickHouse
    docker exec clickhouse clickhouse-client \
      --query "SELECT * FROM cascata_logs.system_logs ORDER BY timestamp DESC LIMIT 5"
    ```

- [ ] **INT-2 — Schema metadata completo desde o início**
  - Toda tabela tem: `status` (active/recycled), `computation`, `validations`, `pii`, `is_writable`
  - Cacheado no DragonflyDB (key: `meta:{tenant_id}:tables`), revalidado em mudanças de schema
  - Verificação:
    ```sql
    -- No banco do Control Plane
    SELECT column_name, data_type, column_default
    FROM information_schema.columns
    WHERE table_schema = 'cascata_cp'
      AND table_name IN ('table_metadata', 'column_metadata')
    ORDER BY table_name, ordinal_position;
    -- Confirmar presença de: status, computation, validations, pii, is_writable
    ```

- [ ] **INT-3 — Observabilidade completa**
  - Métricas de todos os componentes fluindo via OTel → VictoriaMetrics → Dashboard
  - Logs de todos os eventos fluindo via Redpanda → ClickHouse
  - Pool de conexão, egress filtering, vulnerability scanning, soft delete, validations — tudo visível
  - Verificação:
    ```bash
    # Métricas no VictoriaMetrics
    curl http://localhost:8428/api/v1/query?query=up
    # Deve mostrar todos os targets como up=1

    # Logs no ClickHouse
    docker exec clickhouse clickhouse-client \
      --query "SELECT service, count() FROM cascata_logs.system_logs GROUP BY service"
    # Deve mostrar: 'control-plane', 'gateway', 'pgcat'

    # Tabelas de auditoria existem
    docker exec clickhouse clickhouse-client \
      --query "SHOW TABLES FROM cascata_logs"
    # Deve listar: system_logs, pool_events, egress_events, purge_audit
    ```

- [ ] **INT-4 — Modo Shelter validado**
  - Todos os serviços coexistem em Docker Compose sem crash por OOM
  - Total RAM < 1.8GB com margem de segurança em VPS de 3GB
  - Funcionalidade completa — não é versão reduzida
  - Verificação:
    ```bash
    # Verificar uso de RAM de cada container
    docker stats --no-stream --format "table {{.Name}}\t{{.MemUsage}}\t{{.MemPerc}}"
    # Total deve estar abaixo de 1.8GB

    # Verificar que nenhum container está em restart loop
    docker compose -f docker-compose.shelter.yml ps
    # Todos com status 'Up' e sem '(restarting)'

    # Stress test mínimo: 100 requests concorrentes
    # (verifica que não há OOM sob carga leve)
    ```

- [ ] **INT-5 — Segurança baseline**
  - mTLS em toda comunicação interna
  - Zero secrets hardcoded (tudo via OpenBao)
  - SSRF blocking ativo
  - Egress filtering ativo
  - Vulnerability scanning ativo
  - Server reset query limpa estado de sessão entre transações
  - Verificação:
    ```bash
    # 1. mTLS ativo
    # Tentar conexão sem certificado deve falhar:
    psql "host=yugabytedb port=5433 sslmode=require" 2>&1 | grep -i "certificate"

    # 2. Zero secrets no código
    grep -rn "password\|secret\|api_key" control-plane/ gateway/ \
      --include="*.go" --include="*.rs" --include="*.toml" \
      | grep -v "openbao\|OpenBao\|vault\|test\|example\|TODO"
    # Resultado deve ser vazio

    # 3. SSRF blocking
    curl -X POST http://localhost:8080/rest/v1/webhooks \
      -d '{"url": "http://169.254.169.254/latest/meta-data/"}' \
      2>&1 | grep -i "blocked\|ssrf"
    # Deve retornar erro de SSRF
    ```

- [ ] **INT-6 — Banco do Control Plane íntegro**
  - Verificação:
    ```sql
    -- Todas as tabelas do CP existem
    SELECT table_name FROM information_schema.tables
    WHERE table_schema = 'cascata_cp'
    ORDER BY table_name;
    -- Esperado: column_metadata, extensions_catalog, pool_configs,
    --           table_metadata, tenant_extensions, tenants

    -- Catálogo de extensões populado
    SELECT category, count(*) FROM cascata_cp.extensions_catalog
    GROUP BY category ORDER BY category;
    -- Esperado: 1→14, 2→1, 3→4, 4→1

    -- Schema _recycled existe no banco de tenant
    SELECT schema_name FROM information_schema.schemata
    WHERE schema_name = '_recycled';
    ```

---

> **Princípio guia:** Nenhum item da Fase 0 é visível para o tenant. Todos são críticos para o operador. Sem eles, nada que venha depois funciona em produção real. A Fase 0 é o alicerce — e alicerces não impressionam, mas sem eles a casa cai.

> **Regra de uso deste arquivo:** Ao iniciar qualquer task 0.x, marque o checkbox como `[/]` (em progresso). Ao concluir, marque como `[x]`. Nunca delete linhas — apenas adicione novas linhas quando detalhes forem descobertos durante a implementação. Este arquivo é a memória de trabalho; ele cresce, nunca encolhe.
