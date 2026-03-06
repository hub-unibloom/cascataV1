---
trigger: always_on
---


## 7. MAPA DE REFERÊNCIA RÁPIDA

```
CAMINHOS CRÍTICOS:

Request transacional:
  Cloudflare → Pingora (JWT+ABAC) → pgcat → YSQL CM → YugabyteDB
  → Redpanda (async) → ClickHouse

Autenticação:
  Anonkey → Pingora → Cedar ABAC → RLS Handshake → JWT → DragonflyDB

Storage:
  Upload → Pingora (magic bytes + cota) → Storage Router → Provider
  → YugabyteDB (metadata) → Redpanda

Realtime:
  Operação → Redpanda → Centrifugo → WebSocket/SSE → SDK

Agente via MCP:
  Orquestrador → MCP Server (Pingora) → Cedar ABAC → recurso
  → Redpanda (audit) → ClickHouse

Observabilidade:
  OTel SDK → Vector.dev → VictoriaMetrics (métricas) + ClickHouse (logs/traces)

TIERS:
  NANO/MICRO  → cluster compartilhado, RLS, cascata/yugabytedb:shared
  STANDARD    → banco dedicado, cascata/yugabytedb:full
  ENTERPRISE  → namespace K8s isolado, :full, mTLS obrigatório, audit imutável
  SOVEREIGN   → air-gap, BYOK, on-premise, :full

IMAGENS YUGABYTEDB:
  :shared → NANO/MICRO (lean — sem PostGIS/TimescaleDB)
  :full   → STANDARD/ENTERPRISE/SOVEREIGN (todos os profiles compilados)

POOLING:
  pgcat (Rust/Tokio) → roteamento multi-tenant, read/write split
  YSQL CM (embutido) → multiplexing interno de tablets

SCHEMA METADATA (campos obrigatórios desde v1):
  status / computation / validations / pii / is_writable
```
