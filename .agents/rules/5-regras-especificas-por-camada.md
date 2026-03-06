---
trigger: always_on
---


## 5. REGRAS ESPECÍFICAS POR CAMADA

### 5.1 Control Plane (Go)
- Toda goroutine lançada deve ter contexto com cancelamento (`context.WithCancel` ou `context.WithTimeout`)
- Erros propagados com contexto (`fmt.Errorf("operação X: %w", err)`)
- Nenhuma configuração hardcoded — tudo via struct de configuração carregada no boot
- Health checks ativos para todo componente monitorado pelo DR Orchestrator
- Toda mudança de estado de tenant registrada no ClickHouse via Redpanda

### 5.2 API Gateway / Pingora (Rust)
- Zero `unwrap()` em código de produção — `?` operator ou `match` explícito
- Todo middleware com seu próprio span OTel
- Validações na ordem do SAD: ABAC → RLS → Validation Engine → Query
- Server Reset Query executada antes de devolver conexão ao pgcat — sem exceção

### 5.3 YugabyteDB / Queries
- Todo `SELECT` público com `LIMIT` explícito
- Toda query de escrita passa pelo RLS Handshake atômico — sem exceção
- Migrations reversíveis (up + down), testadas antes de produção
- Índices justificados com `EXPLAIN ANALYZE` documentado

### 5.4 Schema Metadata
- Todo novo campo adicionado deve ser propagado para: SDK type generator, APIDocs, MCP Server, Protocol Cascata
- Schema metadata é propriedade do Control Plane — nunca modificado diretamente pelo código do tenant

### 5.5 Segurança (transversal)
- Secrets sempre via OpenBao — nunca em env vars diretas, nunca em código
- Comparações de tokens/OTPs via `timingSafeEqual` — nunca `==`
- Toda URL de webhook validada contra blocklist SSRF antes do primeiro DNS lookup
- Magic bytes validados em todo upload — `Content-Type` declarado nunca é confiado
- Logs nunca contêm: tokens JWT, API keys, senhas, CPF, dados de cartão, campos `pii: true`

