# Documento de Arquitetura de Software (SAD)
# Cascata — "Koenigsegg"

> **Visão Geral:** O Cascata é um orquestrador de infraestrutura distribuída que opera a mesma API para um desenvolvedor solo e para um banco tier-1 — sem mudança de código cliente, sem downtime de migração entre tiers. O verdadeiro orquestrador é o usuário: o Cascata pavimenta os caminhos, o tenant decide como, onde e com quais ferramentas o projeto dele funciona. Sua arquitetura é dividida em Três Planos Rígidos com fronteiras de responsabilidade invioláveis — nenhum plano acessa responsabilidades do outro.

---

## 1. Os Três Planos

A separação em três planos não é organizacional — é física. Cada plano tem fronteiras de rede, responsabilidades e tecnologias distintas. Um bug no Data Plane não pode comprometer o Control Plane. Uma falha no Infra Plane não vaza dados entre tenants. Essa separação é a garantia estrutural que permite o Cascata operar do tier NANO ao SOVEREIGN com a mesma base de código.

---

### A. CONTROL PLANE — O Cérebro Roteador

Governa o ambiente, classifica tenants, roteia para o Data Plane correto, gerencia billing e provisionamento. Não toca em dados brutos de tenant em nenhuma circunstância — essa fronteira é inviolável por design arquitetural. O Control Plane conhece a existência dos tenants e suas configurações. Não conhece o conteúdo dos dados que eles armazenam.

**Linguagem:** Go 1.26+
Concorrência massiva via goroutines sem event loop, binário único com compilação estática, inicialização em milissegundos, zero dependência de ecossistema externo no runtime. Go 1.26 foi lançado em fevereiro de 2026 e é a versão base do projeto.

**Roteador HTTP:** Chi
Minimalista, middleware composável, sem abstrações desnecessárias. O backbone de roteamento do Cascata exige previsibilidade e performance — não mágica de framework.

**Dashboard DX:** SvelteKit + TypeScript
O que vai para produção é JavaScript compilado e otimizado — sem runtime do framework em execução no cliente. TypeScript é a linguagem nativa do SvelteKit, não uma camada adicional. O Dashboard é a interface única através da qual o tenant opera todo o seu projeto: banco de dados, autenticação, storage, funções, logs, billing, compliance e configuração de infra. Toda observabilidade, analytics e métricas do sistema são consumidas e exibidas diretamente neste painel — sem dependência de ferramentas externas de visualização.

**IAM / Autorização:** Cedar Policy Language (Apache 2.0)
Linguagem de policy formal com verificação matemática de completude e consistência. Compilada como biblioteca Rust dentro do Cascata, sem runtime externo. Suporta políticas ABAC com contexto completo: role, IP, horário, dispositivo, status de compliance — tudo combinado em uma única expressão formalmente verificável. Uma política ambígua ou contraditória é detectada em tempo de compilação, não em produção.

**KMS:** OpenBao (Linux Foundation, MPL 2.0)
Fork open source do Vault com API idêntica, mantido pela Linux Foundation. Isola Master Keys por tenant via envelope encryption. Rotação automática de chaves com audit trail completo de todo acesso a secrets. Em tiers SOVEREIGN, roda inteiramente no ambiente on-premise do cliente — nenhuma chave trafega para a infraestrutura do Cascata em nenhuma circunstância.

---

### B. DATA PLANE — O Motor

Processa trilhões de operações sem onerar a experiência do desenvolvedor. Toda latência adicionada por esta camada deve ser mensurável e justificável. O Data Plane não toma decisões de negócio — executa as decisões de roteamento e isolamento definidas pelo Control Plane para cada tenant.

**API Gateway / WAF:** Pingora
Escrito em Rust sobre Tokio pela Cloudflare, disponível como projeto open source. Intercepta requests, valida JWT, aplica Rate Limiting consultando DragonflyDB, transforma payloads em SQL AST otimizado antes de encaminhar ao YugabyteDB. Testado em produção processando mais de 1 trilhão de requests por dia. Atua também como Storage Router — classifica uploads por setor, valida magic bytes e roteia para o provider correto baseado nas regras de storage governance do tenant. Latência adicionada ao pipeline: <0.5ms p99.

**Auth Engine:** Cedar ABAC + RLS Handshake
O motor de autenticação e autorização opera em duas camadas complementares. A camada superior é o Cedar ABAC — políticas contextuais compiladas em Rust que validam toda request antes de qualquer acesso a dado. As políticas operacionais são compiladas e serializadas pelo Control Plane dentro do **DragonflyDB (hot-cache)** de cada tenant. Durante a requisição, o Pingora consulta o DragonflyDB em menos de 0.1ms para resolver a árvore de decisão do Cedar no edge, viabilizando o gatekeeping em tempo real sem onerar o banco YugabyteDB central da aplicação. A camada inferior é o RLS Handshake atômico no YugabyteDB: ao chegar no banco, a execução desce para um role com permissões mínimas (`SET LOCAL ROLE cascata_api_role`), as claims do JWT são injetadas como variáveis de sessão (`set_config('request.jwt.claim.sub', id)`), e as políticas RLS do banco bloqueiam fisicamente o acesso a dados de outros tenants. O vazamento de dado entre tenants é impossível neste modelo — não por código defensivo, mas por impossibilidade física na camada de persistência.

O transport de credenciais (Magic Links, OTPs, tokens de convite) suporta três canais configuráveis por tenant: SMTP local, provider externo (Resend ou qualquer SMTP), e Webhook customizado assinado com HMAC-SHA256 no header `X-Cascata-Signature` — permitindo integração com qualquer sistema de envio externo sem expor a lógica interna do Cascata.

**Transacional OLTP:** YugabyteDB
PostgreSQL distribuído nativo com sharding automático, ACID completo e consistência forte em deployments multi-region. Clientes utilizam drivers PostgreSQL padrão sem adaptação de código. Tablespaces mapeados por país garantem data residency por design de storage — LGPD e GDPR são tratados na camada de persistência, não em middleware. Em tiers SOVEREIGN, instância dedicada com full encryption at-rest via BYOK gerenciado pelo próprio cliente.

**Cache:** DragonflyDB
Motor de cache em C++ multi-thread com compatibilidade total com a API Redis. Sem garbage collector. Suporta session management, rate limiting, distributed locking, hot config cache e reserva preventiva de cota de storage, tudo com latência <0.1ms. Escala verticalmente aproveitando todos os núcleos disponíveis sem configuração adicional. Utilizado também como camada de short-circuit para reads frequentes — um cache hit nunca chega ao YugabyteDB.

**Event Stream:** Redpanda
Plataforma de streaming escrita em C++ que entrega latência p99 abaixo de 1ms e ingestão de milhões de eventos por segundo por broker. Opera sem JVM e sem dependências externas de coordenação, mantendo compatibilidade total com o protocolo Kafka. Toda operação transacional, de storage, de auth e de execução de função no Cascata gera um evento assíncrono no Redpanda sem penalizar o RTT do cliente. É o barramento central de audit trail do sistema.

**Analytics / Logs:** ClickHouse
Motor colunar OLAP com compressão nativa que reduz 1TB de logs para ~80GB em disco. Queries sobre bilhões de linhas retornam em segundos. Ingestão de 1M+ eventos por segundo por nó via pipeline assíncrono Redpanda → ClickHouse. TTL automático por tenant e tiered storage nativos — dados quentes em NVMe, dados frios em object storage, sem intervenção manual. Logs são particionados por tenant — um tenant nunca enxerga dados de outro, mesmo em tiers que compartilham cluster. Os dados do ClickHouse alimentam diretamente o Dashboard SvelteKit do Cascata via queries nativas.

**Storage — Orquestrador de Arquivos**

O storage do Cascata opera em dois níveis distintos e complementares:

*Nível 1 — Storage Router (Pingora/Rust)*
O Pingora executa toda a inteligência de storage antes de qualquer escrita: classificação do arquivo por setor (visual, motion, audio, structured, docs, exec, telemetry, simulation), validação de magic bytes lendo o header binário real do arquivo operando estritamente em **modo stream pass-through (zero-copy)** — garantindo que buffers imensos não esgotem a RAM — e cruzando com o dicionário oficial de assinaturas, reserva preventiva de cota no DragonflyDB antes de aceitar o upload, e roteamento para o provider correto baseado nas regras de storage governance configuradas pelo tenant no painel. Todo esse processo ocorre em Rust, sem overhead de runtime de alto nível.

*Nível 2 — Camada de Adaptadores de Provider*
MinIO é o backbone soberano self-hosted, obrigatório para tiers ENTERPRISE e SOVEREIGN. Para os demais tiers, o tenant configura os providers que deseja no painel. O Cascata expõe uma camada de adaptadores para: AWS S3, Cloudflare R2, Wasabi (S3-compatible), Cloudinary (otimização automática de mídia), ImageKit (CDN de imagem com transformações em tempo real), Google Drive, OneDrive e Dropbox.

O endpoint público permanece sempre o mesmo independente do provider escolhido. O roteamento, as credenciais dos providers e toda a lógica de storage são completamente invisíveis para o usuário final do tenant — o Cascata atua como túnel soberano, sem expor chaves de API ou URLs de providers externos em nenhum momento.


**Realtime Engine: Centrifugo**

Serviço dedicado em Go que opera como bridge entre o Redpanda e os clientes conectados. Consome tópicos do Redpanda via consumer group próprio — sem interferência no pipeline de audit trail. Mantém conexões WebSocket/SSE persistentes com clientes autenticados.

A autorização de canal é delegada ao Cedar ABAC via token de canal gerado pelo Pingora no momento da conexão — o Centrifugo não toma decisões de autorização, apenas verifica o token. Histórico de mensagens recentes mantido no DragonflyDB com TTL configurável por canal.

Footprint em modo Shelter: ~100MB RAM para 50.000 conexões simultâneas. Em tiers ENTERPRISE e SOVEREIGN: múltiplas instâncias provisionadas pelo Kubernetes Operator com sticky sessions gerenciadas pelo Pingora.


**Banco Vetorial: Qdrant**

Escrito em Rust, licença Apache 2.0, self-hosted com modo distribuído nativo. Integrado ao Data Plane como recurso de primeira classe — não como serviço externo. Toda operação passa pelo Pingora com validação Cedar ABAC antes de chegar ao Qdrant.

HNSW com quantização escalar e binária — footprint de vetores reduzido em até 32x em relação a vetores float32 brutos, viabilizando operação em modo Shelter sem degradação de precisão significativa. Payload filtering dentro do índice — multi-tenancy sem overhead de pós-filtragem.

Snapshots periódicos exportados automaticamente para o MinIO do projeto via DR Orchestrator. Fator de replicação ≥ 2 em STANDARD+. Instância dedicada em ENTERPRISE e SOVEREIGN com nós próprios.

**DR Orchestrator**

Serviço dedicado em Go que opera como guardião de resiliência de todos os componentes com estado. Monitora YugabyteDB, ClickHouse, Redpanda, DragonflyDB, MinIO, OpenBao e Qdrant via health checks ativos com timeout agressivo por componente.

Em falha detectada: executa runbook pré-definido para o tier do tenant afetado, atualiza o Tenant Router para redirecionar tráfego, notifica operador via dashboard e webhook, registra evento completo no ClickHouse. Zero decisão heurística — runbooks são sequências determinísticas validadas.

Em tiers ENTERPRISE e SOVEREIGN: chaos drills automatizados em janela de baixo tráfego verificam continuamente que as garantias de RTO/RPO do tier são cumpridas. DR não testado não tem garantia real.

**Webhook Engine**

Consumer dedicado do Redpanda — sem sistema de filas adicional. Consome todos os tópicos de eventos do sistema e avalia os filtros configurados por tenant antes de qualquer chamada HTTP externa. Filter engine executado em Rust no Pingora — zero overhead de runtime de alto nível na avaliação de filtros.

Workers de entrega HTTP executam em Go — goroutines por entrega, concorrência massiva sem event loop. Retry state persistido no DragonflyDB para sobreviver a restart de worker sem perder posição nas tentativas. Dead letters registrados no ClickHouse e Fallback URL disparada via worker dedicado separado do pipeline principal.

SSRF prevention executado antes do primeiro DNS lookup — a URL é validada contra blocklist de endereços internos antes de qualquer resolução de nome.

Footprint em modo Shelter: webhook engine compartilha processo com outros consumers Redpanda — sem serviço adicional no Docker Compose.


**Response Rule Engine**

Executado no Pingora após o commit do YugabyteDB, antes do retorno ao cliente. Verifica se existe uma Response Rule ativa para a operação executada (tabela + tipo de operação + condição avaliada pelo filter engine). Se sim, monta o payload de interceptação, executa a chamada HTTP ao webhook configurado com timeout hard limit, e aplica o comportamento de falha configurado se o webhook não responder dentro do prazo.

A chamada ao webhook de interceptação ocorre no caminho crítico de retorno — diferente dos webhooks de evento (que são assíncronos). O timeout é configurável pelo tenant com valor default de 5000ms. Response Rules com timeout excedido não bloqueiam o commit — apenas aplicam a política de fallback e registram o evento no ClickHouse.

**Push Delivery Workers**

Workers Go dedicados por protocolo de push (APNs worker, FCM worker, WebPush worker). Consomem do Redpanda — sem sistema de fila adicional. Gerenciam reconexão automática com os servidores dos providers, certificate rotation para APNs, e feedback channel para detecção de tokens inválidos (dispositivos desinstalados).


**Connection Pooling: YSQL Connection Manager (nativo do YugabyteDB)**

O pipeline de conexão opera de forma unificada. O YSQL Connection Manager atua nativamente multiplexando transações. A cadeia completa opera sob mTLS gerenciado pelo OpenBao.

Cinco mecanismos de resiliência operam em paralelo, cada um cobrindo um modo de falha distinto:

**Staggered Pool Warming** — elimina thundering herd em wakeups simultâneos via jitter determinístico por `tenant_id`, com lead time calibrado por tier. Cada tenant acorda exatamente na janela calculada pelo Control Plane baseada em histórico do ClickHouse.

**Statement e Idle-in-Transaction Timeouts** — protegem o pool de conexões perdidas por queries lentas ou clientes desconectados sem fechar transações. Configurados nativamente na base por tier. Timeout encerra a transação, executa server_reset_query, e devolve a conexão ao pool.

**Fila Limitada com Backpressure** — protege a exaustão de conexões. Estágios graduados notificam o Control Plane para avaliação de promoção de tier automática.

**Circuit Breaker por Instância de Banco** — três estados (CLOSED/OPEN/HALF-OPEN) com notificação direta ao Pingora via Unix socket. Quando o YSQL CM de uma instância falha, o tenant afetado recebe HTTP 503 imediato em vez de timeout silencioso. DR Orchestrator acionado automaticamente.

**Pool Size Adaptativo** — dimensionamento baseado em p95 de transações simultâneas dos últimos 7 dias com safety factor. Recalculado a cada 24h pelo Control Plane.

Em modo Shelter com 200 tenants ativos: footprint total ~185MB vs ~8GB sem pooling. Os ~7.8GB liberados convertem-se em cache de páginas do YugabyteDB, reduzindo latência de queries para todos os tenants.



**Schema Metadata — Estrutura Fundacional**

O schema metadata do Cascata é a representação interna de cada tabela de cada projeto. É lido por múltiplos componentes do sistema: TableCreator (painel), APIDocs generator, SDK type generator, MCP Server, Protocol Cascata, Pingora (validation rules e computed columns). Por ser consumido por tantos componentes, sua estrutura é definida como fundação e não pode ser alterada retroativamente sem migração de todos os sistemas que dependem dela.

O schema metadata suporta desde a versão inicial os seguintes campos por coluna:

- `status`: `active | recycled` — para soft delete
- `computation`: `null | { kind, expression, layer }` — para computed columns
- `validations`: `[]` — para data validation rules
- `pii`: `boolean` — para data masking
- `is_writable`: `boolean` — derivado de `computation` e de universal padlock

O schema metadata é armazenado no YugabyteDB do Control Plane (não no banco do tenant) e cacheado no DragonflyDB por projeto. Toda mudança de schema via painel ou CLI invalida o cache e regenera os artefatos dependentes: tipos TypeScript, documentação OpenAPI, ferramentas MCP.

**Validation Engine (Pingora)**

Módulo Rust executado no Pingora para toda request de escrita. Carrega as validações do projeto a partir do cache DragonflyDB (invalidado automaticamente em mudanças de schema). Executa validações na seguinte ordem:

1. `required` — campos obrigatórios presentes
2. `type` — tipos corretos para os campos presentes
3. `regex`, `range`, `length`, `enum` — validações por campo
4. `cross_field` — expressões entre campos (avaliador de expressões Rust — sem eval)
5. `jwt_context` — validações com claims do JWT
6. `unique_soft` — queries de unicidade ao YugabyteDB (apenas quando todos os anteriores passam)

Falha em qualquer etapa retorna HTTP 422 imediatamente sem executar as etapas posteriores, exceto quando múltiplas violações da mesma etapa podem ser coletadas (etapas 2-5 coletam todas as violações antes de retornar).



---

### C. INFRA PLANE — O Chassi

A fundação invisível. Nenhum cliente ou tenant interage com esta camada diretamente. O Infra Plane existe para garantir que os outros dois planos operem com isolamento, segurança e resiliência independente do tier ou do volume de tráfego.

**Orquestração:** Kubernetes
Namespace por tenant para tiers ENTERPRISE e SOVEREIGN. Operators customizados provisionam YugabyteDB e DragonflyDB por tenant automaticamente, sem intervenção manual do operador do Cascata. HPA (Horizontal Pod Autoscaler) para autoscaling nativo baseado em métricas reais de consumo. Multi-cloud por definição — o mesmo Operator funciona em AWS, GCP, Azure, bare metal ou VPC privada sem alteração de configuração.

**Networking / Segurança:** Cilium + eBPF
Roteamento de pacotes na camada de kernel via eBPF, bypassando IPtables com zero overhead de sidecar proxy. IP spoofing entre tenants é fisicamente impossível em nível de kernel — não é prevenido por código da aplicação, é prevenido pela rede. mTLS transparente entre todos os pods sem configuração por serviço. Network Policies com granularidade de processo — não apenas de IP ou porta. Para tiers ENTERPRISE e SOVEREIGN, cada namespace Kubernetes tem políticas Cilium que impedem qualquer comunicação cruzada, mesmo que o código da aplicação tente explicitamente.

**KMS:** OpenBao
Isolamento de Master Keys por tenant via envelope encryption. Rotação automática de chaves com audit trail completo de todo acesso. Para tiers SOVEREIGN: OpenBao roda inteiramente no ambiente on-premise do cliente — zero chaves em servidores do Cascata.

**Observabilidade:** OpenTelemetry → VictoriaMetrics + ClickHouse
Pipeline completamente vendor-neutral: OTel SDK coleta traces, métricas e logs de todos os serviços → Vector.dev (collector em Rust) processa e encaminha → VictoriaMetrics para métricas de sistema (single binary, compressão superior, sem dependências externas de escala) + ClickHouse para logs e traces. Todo dado de observabilidade é consumido e exibido diretamente no Dashboard SvelteKit do Cascata. O operador enxerga saúde do sistema, performance por tenant, alertas e traces sem precisar abrir nenhuma ferramenta externa.

**Object Storage:** MinIO
S3-compatible, self-hosted, com erasure coding nativo — dados sobrevivem à perda de N/2 nós simultâneos sem degradação. WORM (Write Once Read Many) ativo para compliance bancário e médico. BYOK por tenant via OpenBao. Lifecycle policies automáticas por tenant para tiering de dados quentes/frios. Para tiers SOVEREIGN: MinIO roda dentro do VPC do cliente, garantindo que nenhum arquivo trafega para infraestrutura externa em nenhuma circunstância.

**Extension Profile System**

O YugabyteDB é provisionado em uma de duas imagens Docker gerenciadas e publicadas pelo Cascata:

```
cascata/yugabytedb:shared
  Inclui: extensões nativas YugabyteDB + pg_cron
  Usada: cluster compartilhado NANO/MICRO
  Objetivo: footprint mínimo — cada MB é recurso coletivo dos tenants

cascata/yugabytedb:full
  Inclui: tudo do :shared + PostGIS (+ Tiger + Topology)
  Dependências compiladas: libgeos, libproj, libgdal, libboost
  Usada: toda instância dedicada STANDARD / ENTERPRISE / SOVEREIGN
  Objetivo: zero friction — desenvolvedor habilita qualquer extensão
             sem processo adicional, sem rolling upgrade
```

**Build Pipeline das Imagens:**
Dockerfile multi-stage — compilação em estágio builder isolado, apenas os binários resultantes (`.so`, `.control`, `.sql`) copiados para a imagem final de runtime. Zero ferramentas de compilação na superfície de ataque. Imagens publicadas no registry privado do Cascata e versionadas por release do YugabyteDB suportado.

**Provisionamento pelo Control Plane:**
No provisionamento de novo tenant, o Control Plane seleciona a imagem automaticamente baseado no tier:
- NANO / MICRO → `cascata/yugabytedb:shared` (cluster compartilhado existente — sem nova instância)
- STANDARD / ENTERPRISE / SOVEREIGN → `cascata/yugabytedb:full` (nova instância provisionada pelo Kubernetes Operator)

Não existe seleção de profile pelo tenant. Não existe decisão de imagem no Provisioning Wizard. É automático e invisível.

**Ausência de Rolling Upgrade por Extensão:**
A decisão de usar imagem `:full` em toda instância dedicada elimina completamente o problema de rolling upgrade para adição de extensão. Um tenant STANDARD que decide usar PostGIS hoje não precisa de nenhuma operação de infraestrutura — a imagem já contém o binário. A habilitação é `CREATE EXTENSION postgis` via painel.



---

## 2. Filosofia "Day 1 Shelter" vs Escala Planetária

Uma das propriedades mais críticas do Cascata é a capacidade de operar em extremos opostos de escala com a mesma base de código, a mesma API e a mesma interface de configuração.

### Modo Shelter — VPS Única ($20/mês)

Tiers NANO e MICRO operam inteiramente em uma única VPS rodando Docker Compose. Todos os binários coexistem sem crash por OOM. O sistema é completo em funcionalidade desde o primeiro container levantado — não é uma versão reduzida do Cascata, é o Cascata completo em modo econômico.

```
VPS Host ($20/mês)
├── Pingora Gateway (Rust)          — ~30MB RAM
├── Control Plane (Go)              — ~20MB RAM
├── YugabyteDB Shared Cluster       — ~800MB RAM
├── DragonflyDB                     — ~100MB RAM
├── Redpanda (1 broker)             — ~200MB RAM
├── ClickHouse (modo single node)   — ~300MB RAM
├── OpenBao                         — ~50MB RAM
├── Qdrant (Banco Vetorial)         — ~100MB RAM
├── Centrifugo (Realtime Engine)    — ~100MB RAM
└── Dashboard SvelteKit             — ~0MB RAM adicional
    (build estático servido pelo Pingora Gateway)

Total estimado: ~1.8GB RAM
Operacional em VPS de 3GB com margem de segurança
```

### Escala Planetária

Quando um tenant percebe que precisa de escala real, o processo é uma ação no painel — não uma migração. O Control Plane executa automaticamente:

1. Provisiona recursos adicionais via Kubernetes Operator no cloud provider configurado
2. Migra o tenant sem downtime via YugabyteDB live migration
3. Atualiza o routing no Tenant Router automaticamente
4. Redireciona o tráfego para os novos nós sem janela de manutenção

O cliente do tenant não muda uma linha de código. O endpoint de API permanece idêntico. A promoção de tier é invisível para o usuário final. O operador do Cascata altera um único parâmetro no painel para apontar novos clusters para clouds externas (AWS, GCP, bare metal). Nenhuma edição manual de configuração de infraestrutura, nenhum downtime.

---

## 3. Fluxo Transacional Completo

O Cascata processa quatro categorias distintas de request. Cada categoria tem seu próprio caminho otimizado — não existe um caminho genérico que trata tudo da mesma forma.

---

### Caminho A — Request Transacional (SQL)

```
1. SDK Cliente
   └── HTTPS → Cloudflare Workers
               (DDoS mitigation, GeoIP, TLS termination)
               ↓
2. Pingora WAF (Rust)
   └── Cilium XDP intercepta na placa de rede
       Rate Limit via DragonflyDB (<0.1ms)
       JWT validation + Cedar ABAC policy check
       Payload → SQL AST otimizado
               ↓
3. Tenant Router (Control Plane — Go)
   └── Identifica tier do tenant
       Resolve endpoint do Data Plane correto
       Aplica SLA policies do tier
               ↓
4A. Cache Hit (DragonflyDB)
   └── Read encontrada em cache → retorno imediato
       Pipeline encerra aqui para reads cacheadas
       P99 < 1ms totais
               ↓ (apenas em cache miss)
4B. YugabyteDB
   └── BEGIN — lock transacional atômico
       SET LOCAL ROLE cascata_api_role
       set_config('request.jwt.claim.sub', id_usuario)
       Execução transacional ACID sob RLS
       Confirmação de commit
       Resultado gravado no DragonflyDB
               ↓ (assíncrono — não bloqueia retorno)
5. Redpanda
   └── Audit Trail event publicado
       Redpanda → ClickHouse (batch async)
       Alimenta dashboard de analytics do tenant
               ↓
6. Retorno ao Cliente
   └── P99 < 5ms totais (cache miss)
       P99 < 1ms totais (cache hit)
```

---

### Caminho B — Request de Storage (Upload / Download)

```
1. SDK Cliente
   └── HTTPS → Cloudflare Workers
               ↓
2. Pingora Storage Router (Rust)
   └── Cilium XDP intercepta na placa de rede
       JWT validation + Cedar ABAC policy check
       Magic Bytes validation
       (leitura do header binário real do arquivo)
       Classificação de setor:
       visual / motion / audio / structured / docs /
       exec (hard-block) / telemetry / simulation
       Checagem e reserva de cota via DragonflyDB
       (reserva preventiva antes de aceitar o upload)
               ↓
3. Tenant Router (Control Plane — Go)
   └── Lê regras de storage governance do tenant
       Decide provider por setor configurado:
       ├── MinIO         → backbone soberano (ENTERPRISE/SOVEREIGN obrigatório)
       ├── Cloudflare R2 → edge global S3-compatible
       ├── AWS S3        → governança apontada para AWS
       ├── Wasabi        → S3-compatible alta capacidade
       ├── Cloudinary    → mídia com otimização automática
       ├── ImageKit      → imagem/vídeo com CDN em tempo real
       ├── Google Drive  → integração via OAuth do tenant
       ├── OneDrive      → Microsoft Graph API
       └── Dropbox       → integração via OAuth do tenant
               ↓
4. Provider de Destino
   └── Escrita com BYOK encryption (OpenBao)
       Metadata gravado no YugabyteDB
       Endpoint público sempre: seudominio.io/storage/...
       Credenciais e URLs de provider nunca expostas ao cliente
               ↓ (assíncrono)
5. Redpanda
   └── Storage event publicado
       Redpanda → ClickHouse (audit + usage metering)
               ↓
6. Retorno ao Cliente
   └── URL de acesso + metadados do arquivo
```

---

### Caminho C — Invocação de Sandbox Function (Cancelaado, sem suporte a edge functions)


---

### Caminho D — Fluxo de Autenticação

```
1. Cliente (app web / mobile nativo)
   └── Request de auth com Anonkey do projeto
       HTTPS → Cloudflare Workers
               ↓
2. Pingora WAF (Rust)
   └── Anonkey validation
       Identifica tenant e projeto de origem
       Rate Limit anti-brute-force via DragonflyDB
       Identifica método de auth da request:
       Passkey/FIDO2 / Magic Link / OTP / OAuth / CPF+Senha / etc.
               ↓
3. Tenant Router (Control Plane — Go)
   └── Lê configuração de auth do tenant
       Verifica se o método solicitado está habilitado no projeto
       Resolve fluxo de auth correspondente
               ↓
4A. Fluxo OAuth (Google / GitHub / Apple / etc.)
   └── Token recebido do provider OAuth
       Validação direta no servidor criptográfico do provider
       (não confia no payload do cliente — valida na fonte)
       Identidade vinculada à conta em auth.identities
       Anonkey resolve redirect correto (web URL / mobile deep link)
       Sessão estabelecida via RLS Handshake
               ↓
4B. Fluxo Magic Link / OTP
   └── Identifica se email/telefone existe na base
       SE NÃO EXISTE: delay randômico + retorno de sucesso falso
       (anti-timing attack — atacante não enumera contas válidas)
       SE EXISTE: token gerado e enviado via transport configurado
       (SMTP local / provider externo / Webhook HMAC-SHA256)
       OTP validation via timingSafeEqual em buffer de kernel
       (tempo constante — não vaza informação por latência de CPU)
       Sessão estabelecida via RLS Handshake
               ↓
4C. Fluxo Passkey / FIDO2
   └── Challenge gerado pelo servidor
       Resposta biométrica validada contra credencial registrada
       Sem senha, sem token de email — autenticação pura por chave
       Sessão estabelecida via RLS Handshake
               ↓
5. Sessão Estabelecida
   └── JWT gerado com claims do usuário e do tenant
       Sessão gravada no DragonflyDB (session cache)
       RLS Handshake preparado para requests subsequentes
               ↓ (assíncrono)
6. Redpanda
   └── Auth event publicado (sucesso ou falha)
       Redpanda → ClickHouse (audit trail de autenticação)
               ↓
7. Retorno ao Cliente
   └── JWT + refresh token
       App mobile: redirect via deep link da Anonkey
       App web: redirect para URL configurada na Anonkey
       P99 < 8ms (fluxo sem email/SMS)
```


### Caminho E — Realtime Subscription (Cliente conectado recebendo eventos)

```
1. SDK Cliente (@cascata/client ou @cascata/compat)
   └── cascata.realtime.channel('pedidos').on('INSERT', cb).subscribe()
       HTTPS Upgrade → WebSocket
       Cloudflare Workers (mantém conexão WebSocket persistente)
               ↓
2. Pingora Gateway (Rust)
   └── JWT validation
       Cedar ABAC — verifica permissão de subscribe no canal
       Gera token de canal assinado com TTL
       Roteia conexão para Centrifugo
               ↓
3. Centrifugo (Go)
   └── Valida token de canal
       Registra cliente no canal solicitado
       Confirma subscribe ao cliente
       (conexão WebSocket mantida aberta)
               ↕ (conexão persistente estabelecida)

--- Em paralelo: quando uma operação ocorre no sistema ---

4. Operação no Cascata (qualquer caminho A, B, C ou D)
   └── Transação / Upload / Function / Auth concluída
               ↓ (assíncrono)
5. Redpanda
   └── Evento publicado no tópico correspondente
               ↓
6. Centrifugo
   └── Consome evento do tópico Redpanda
       Filtra clientes subscritos ao canal afetado
       Cedar ABAC — revalida permissão do cliente receptor
       Entrega evento via WebSocket/SSE
               ↓
7. SDK Cliente
   └── Callback do tenant executado com payload do evento
       Latência fim-a-fim desde operação: P99 < 50ms
       Sem polling, sem request adicional ao servidor
```

### Caminho F — Failover Automático (DR Orchestrator em ação)

```
Componente com estado detecta falha ou degradação
   └── Health check ativo timeout excedido
       (YugabyteDB / Redpanda / ClickHouse / MinIO / Qdrant / OpenBao)
               ↓
DR Orchestrator (Go)
   └── Classifica severidade: nó / zona / região
       Identifica tier do(s) tenant(s) afetado(s)
       Seleciona runbook correspondente ao tier + componente + severidade
               ↓
Execução do Runbook (determinística)
   ├── Isola nó falho do cluster
   ├── Promove réplica saudável a primária (onde aplicável)
   ├── Atualiza Tenant Router no Control Plane
   │   (tráfego redirecionado para nós saudáveis)
   ├── Verifica integridade dos dados pós-failover
   └── Confirma que RTO do tier foi cumprido
               ↓ (assíncrono — não bloqueia tráfego restaurado)
Redpanda
   └── DR event publicado com contexto completo
       Redpanda → ClickHouse (audit trail imutável do evento de DR)
               ↓
Dashboard do Operador
   └── Alerta em tempo real com:
       componente afetado / tenant(s) impactado(s) /
       runbook executado / RTO alcançado / RPO atingido /
       próximas ações recomendadas
               ↓ (em background)
DR Orchestrator
   └── Monitora recuperação do nó falho
       Quando recuperado: reintegra ao cluster
       Ressincroniza dados via replicação nativa do componente
       Notifica operador da recuperação completa
```



### Caminho G — Pool Exhaustion com Promoção Automática de Tier

```
Tenant em MICRO recebe spike de tráfego real
  └── Requests chegam ao YugabyteDB (YSQL CM)
               ↓
YSQL CM — transaciona pools do tenant
  └── Fila entre 70-90%: Estágio 2
      Header X-Cascata-Queue-Pressure: moderate
      Notifica Control Plane
               ↓
Control Plane (Go)
  └── Consulta ClickHouse: padrão de tráfego deste tenant
      Calcula: spike pontual ou crescimento real?
      SE crescimento real:
        Inicia promoção automática MICRO → STANDARD
        Provisiona instância YugabyteDB dedicada
        Provisiona pool ajustado nativamente no banco
        Migra tenant sem downtime via YugabyteDB live migration
        Atualiza routing no Tenant Router
      SE spike pontual:
        Aumenta pool_size temporariamente (burst mode) por 15min
        Monitora se spike cede
               ↓ (se fila atinge >90% antes da promoção concluir)
YSQL CM — Estágio 3: rejeita novas requests excedentes
  └── HTTP 503 com Retry-After calculado
      Evento crítico no ClickHouse
      Operador notificado via Central de Comunicação
               ↓ (após promoção ou calmaria do spike)
Sistema retorna ao estado normal
  └── Evento de resolução registrado no ClickHouse
      Relatório de incidente disponível no painel do operador
```


---

## 4 Arquitetura AI-First — O Agente como Operador

### 4.1 Princípio Fundador

O Cascata foi projetado em 2026 — período em que agentes autônomos de IA já operam em produção em sistemas críticos. A decisão arquitetural central foi: agentes não são integrações externas. São operadores internos do sistema, com identidade, com permissões, com rastreabilidade — exatamente como operadores humanos, com as mesmas garantias de segurança e isolamento.

Isso tem uma consequência direta: qualquer caminho que um humano percorre no sistema, um agente pode percorrer — com as permissões corretas configuradas pelo tenant. Não existe caminho especial para IA. Existe o sistema, e o sistema já sabe lidar com agentes.

### 4.2 Fluxo de um Agente Operando no Cascata

```
Orquestrador do Tenant
(LangGraph / CrewAI / AutoGen / código próprio)
   └── Conecta ao MCP Server do projeto via endpoint autenticado
               ↓
MCP Server (Pingora — gerado por projeto)
   └── Recebe tool call do agente
       Valida identidade do agente (agent.id + agent.scope)
       Cedar ABAC — verifica se o agente tem permissão
       para esta operação específica neste contexto
               ↓
       Roteia para o recurso correto:
       ├── query(sql)        → YugabyteDB via RLS Handshake
       ├── storage.write()   → Storage Router (magic bytes + governance)
       ├── storage.read()    → Provider correto transparente
       ├── function.invoke() → Sandbox Runtime (Deno / CF Workers)
       ├── events.publish()  → Redpanda (tópico autorizado)
       ├── events.subscribe()→ Redpanda (stream em tempo real)
       └── analytics.query() → ClickHouse (partição do tenant)
               ↓ (assíncrono — não bloqueia resposta ao agente)
Redpanda
   └── Agent operation event publicado
       Redpanda → ClickHouse
       (audit trail completo: agente, modelo, operação, dado, resultado)
               ↓
Retorno ao Orquestrador
   └── Tool result estruturado (JSON)
       O orquestrador decide o próximo passo
       P99 < 8ms por tool call (operações sem I/O externo)
```

### 4.3 Isolamento de Agente por Tier

O nível de isolamento de um agente segue o tier do projeto em que opera:

**NANO / MICRO** — agente opera no cluster compartilhado com RLS e ABAC. Adequado para automações simples, assistentes de projeto, agentes de desenvolvimento.

**STANDARD** — agente opera no banco dedicado do tenant. Adequado para automações de negócio, agentes de atendimento, pipelines de processamento.

**ENTERPRISE** — agente opera em namespace Kubernetes isolado. Toda operação do agente é auditada em ClickHouse imutável. Adequado para agentes financeiros, agentes de saúde, automações reguladas.

**SOVEREIGN** — agente opera inteiramente dentro do VPC do cliente. O modelo pode ser local (rodando on-premise) ou externo (com a API key gerenciada pelo OpenBao do cliente). Zero dados do agente trafegam para infraestrutura do Cascata. Adequado para bancos, defesa, órgãos com requisito de soberania de processamento.

### 4.4 O Tenant como Orchestrador de Agentes

O painel do Cascata é a interface onde o tenant define a topologia de agentes do projeto. Não é uma interface de chat com IA — é uma interface de configuração de operadores autônomos:

- Quais agentes existem no projeto
- Qual identidade cada agente tem
- Quais ferramentas MCP cada agente pode usar
- Quais eventos cada agente escuta
- Qual janela de tempo cada agente opera
- Qual humano é responsável por cada agente
- Qual o audit trail de cada agente

O Cascata não decide quais agentes o tenant usa, qual modelo, qual orquestrador, qual arquitetura de agente. Ele garante que qualquer agente — de qualquer provider, de qualquer modelo, construído com qualquer framework — pode operar no projeto com segurança, rastreabilidade e isolamento total.

Essa é a diferença entre IA encaixada e IA desde o berço: o sistema não sabe apenas hospedar agentes. Ele sabe ser operado por eles.


**MCP Server — Camada de Acesso de Agentes**

O Cascata gera automaticamente um MCP Server por projeto, servido pelo Pingora Gateway. Cada MCP Server é isolado por tenant — um agente com credenciais do projeto A não pode enxergar ou operar no projeto B, mesmo que ambos estejam no mesmo cluster.

O MCP Server traduz as ferramentas disponíveis no projeto (tabelas, funções, storage, eventos, logs) em ferramentas MCP padronizadas. Quando o tenant cria uma nova tabela no painel, essa tabela aparece automaticamente como ferramenta disponível no MCP Server do projeto — sem configuração adicional, sem código de integração.

A autenticação do MCP Server usa o mesmo mecanismo de identidade de agente descrito no SRS — Cedar ABAC aplicado a cada tool call, RLS Handshake obrigatório em toda query ao YugabyteDB, audit trail registrado no ClickHouse para cada operação. As chaves de acesso emitidas aos modelos transitam de forma imaculada e originam-se no OpenBao. Para evitar latência destrutiva para os agentes — e não transformar o OpenBao em um SPOF —, os *lease-tokens* e API Keys correspondentes operam em **cache transitório baseados rigorosamente em seus TTL (Time-To-Live)** na RAM quente do Pingora. O Pingora valida as calls via hot-cache e renova de forma assíncrona as chaves próximo à sua expiração (funcionamento nativo dos agentes Vault).

O protocolo MCP é stateless por design — cada tool call carrega as credenciais do agente e o contexto necessário. Isso significa que qualquer modelo, qualquer orquestrador, qualquer linguagem de programação que implemente o protocolo MCP pode operar nos projetos Cascata sem adapter customizado.



## 5. SDK — A Interface do Produto

### 5.1 Estratégia Dual

O SDK do Cascata opera em dois modos com responsabilidades distintas e sem sobreposição:

**`@cascata/client` — SDK Nativo**
TypeScript-first, tipos gerados a partir do schema real do projeto via CLI. Expõe todos os domínios do Cascata: db, auth, storage, realtime, functions, agents, analytics. É a interface completa do produto. Projetos novos usam este SDK.

**`@cascata/compat` — Compatibilidade Supabase**
Implementa a interface pública do Supabase JS client exatamente. Roteamento interno para o Cascata é completamente transparente. Ferramentas no-code que suportam Supabase, projetos existentes e documentação da comunidade Supabase funcionam sem modificação. Este pacote é mantido como cidadão de primeira classe — não é um hack nem uma camada temporária. É a estratégia permanente de zero barreira de adoção.

### 5.2 Geração de Tipos e Type Safety

O `cascata types generate` introspeta o YugabyteDB do projeto e gera um arquivo TypeScript com tipos completos: tabelas, colunas, relacionamentos, enums, funções RPC, perfis compostos. O resultado é type-safety end-to-end — desde a query no código do tenant até o retorno da API, sem cast manual, sem `any`.

### 5.3 Desenvolvimento Local

O comando `cascata dev` levanta o ambiente completo localmente via Docker Compose. Funcionalmente idêntico à produção — sem mocks. O painel do Cascata fica disponível em `localhost:3000`, o endpoint de API em `localhost:54321`. Hot-reload de funções, streaming de logs, e geração de tipos integrados ao workflow de desenvolvimento.

### 5.4 Footprint e Performance do SDK

O `@cascata/client` é tree-shakeable — o bundle final inclui apenas os módulos que o projeto usa. Um projeto que usa apenas db e auth não carrega o módulo de agents ou analytics. O bundle mínimo (db + auth + realtime) é inferior a 15KB gzipped.

A camada de realtime do SDK mantém uma única conexão WebSocket multiplexada para todos os canais ativos — não abre uma conexão por canal. Reconexão automática com backoff exponencial e recovery de histórico perdido durante a desconexão.

### 5.5 Versionamento e Compatibilidade

O `@cascata/client` segue semver estrito. Breaking changes acontecem apenas em major versions com período de deprecation de 12 meses. O `@cascata/compat` segue a versão do Supabase JS client que implementa — atualizações da interface Supabase são acompanhadas em até 30 dias após release estável.


## 6. Resiliência — Garantias e Arquitetura de DR

### 6.1 Modelo de Garantias por Tier

```
NANO / MICRO — Modo Shelter
  YugabyteDB single node
  └── Snapshot diário → MinIO local
      RPO: até 24h (NANO) / 6h (MICRO)
      RTO: ~30min / ~15min
      Adequado: desenvolvimento, MVPs, validação de produto

STANDARD — Cluster Resiliente
  YugabyteDB 3 nós (Raft)
  Redpanda 3 brokers (fator replicação 3)
  ClickHouse ReplicatedMergeTree (2 réplicas)
  Qdrant fator replicação 2
  └── PITR ativo
      RPO: < 1 minuto
      RTO: < 5 minutos (failover automático)
      Adequado: SaaS com SLA, dados sensíveis em crescimento

ENTERPRISE — Multi-AZ Zero Perda
  YugabyteDB replicação síncrona multi-AZ
  Redpanda geo-aware com fator replicação 3
  ClickHouse réplicas em AZs distintas
  Qdrant nós dedicados multi-AZ
  └── Chaos drills automatizados
      RPO: 0 (replicação síncrona)
      RTO: < 30 segundos
      Adequado: fintechs, clínicas, sistemas regulados

SOVEREIGN — Multi-Region Air-Gap
  YugabyteDB replicação síncrona multi-region
  Redpanda geo-replication entre sites do cliente
  ClickHouse replicação entre data centers do cliente
  OpenBao cluster Raft dedicado on-premise
  Qdrant instância dedicada com nós próprios
  └── DR site também air-gap (dados nunca saem do perímetro)
      Chaos drills com relatório de auditoria independente
      RPO: 0
      RTO: < 15 segundos
      Adequado: bancos tier-1, defesa, órgãos regulados
```

### 6.2 Caminho F — Failover Automático (DR Orchestrator em ação)

```
Componente com estado detecta falha ou degradação
   └── Health check ativo timeout excedido
       (YugabyteDB / Redpanda / ClickHouse / MinIO / Qdrant / OpenBao)
               ↓
DR Orchestrator (Go)
   └── Classifica severidade: nó / zona / região
       Identifica tier do(s) tenant(s) afetado(s)
       Seleciona runbook correspondente ao tier + componente + severidade
               ↓
Execução do Runbook (determinística)
   ├── Isola nó falho do cluster
   ├── Promove réplica saudável a primária (onde aplicável)
   ├── Atualiza Tenant Router no Control Plane
   │   (tráfego redirecionado para nós saudáveis)
   ├── Verifica integridade dos dados pós-failover
   └── Confirma que RTO do tier foi cumprido
               ↓ (assíncrono — não bloqueia tráfego restaurado)
Redpanda
   └── DR event publicado com contexto completo
       Redpanda → ClickHouse (audit trail imutável do evento de DR)
               ↓
Dashboard do Operador
   └── Alerta em tempo real com:
       componente afetado / tenant(s) impactado(s) /
       runbook executado / RTO alcançado / RPO atingido /
       próximas ações recomendadas
               ↓ (em background)
DR Orchestrator
   └── Monitora recuperação do nó falho
       Quando recuperado: reintegra ao cluster
       Ressincroniza dados via replicação nativa do componente
       Notifica operador da recuperação completa
```

### 6.3 Chaos Drills Automatizados

Em tiers ENTERPRISE e SOVEREIGN, o DR Orchestrator agenda chaos drills em janelas de baixo tráfego (configuráveis pelo operador no painel):

**Tipos de drill:**
- Node failure — desliga um nó e verifica failover dentro do RTO
- AZ failure — simula perda de zona inteira (ENTERPRISE/SOVEREIGN)
- Network partition — particiona a rede entre nós e verifica consenso Raft
- Storage corruption — corrompe dados de um nó e verifica detecção e recovery
- Cascading failure — múltiplos componentes simultaneamente

**Resultado de cada drill:**
- Relatório completo no dashboard: passou/falhou, RTO medido vs garantido, RPO medido vs garantido
- Gravado no ClickHouse como audit imutável 
- Para SOVEREIGN: relatório exportável para auditoria regulatória independente

**Princípio:** um sistema de DR que nunca falhou propositalmente não tem garantias reais. O Cascata valida suas garantias continuamente, não apenas quando o desastre acontece.





---
##   \(\infty \) . Decisões Arquiteturais Permanentes (ADR Log)

Toda decisão abaixo foi tomada com motivo técnico documentado. Reversão exige novo ADR com benchmark comparativo público e aprovação explícita. O objetivo do ADR Log não é registrar o passado — é proteger o futuro do projeto de decisões impulsivas baseadas em hype.

| Camada | Escolha Final | Motivo |
|--------|--------------|--------|
| Sistema Operacional Host | AlmaLinux 9 (Exclusividade Absoluta) | Docker compartilha o Kernel do Host. Kernel 5.14 nativo para eBPF pleno. Família RHEL é o ecossistema oficial do Yugabyte e suporta FIPS/SELinux. Elimina fricção de emulação. EOL estabelecido em 2032. |
| Control Plane runtime | Go 1.26+ | Concorrência nativa via goroutines, binário único, zero dependência de ecossistema externo, inicialização em milissegundos |
| API Gateway / WAF | Pingora (Rust) | Construído sobre Tokio, testado em 1 trilhão de requests/dia, connection pooling, TLS e load balancing resolvidos com <0.5ms p99 |
| Dashboard DX | SvelteKit + TypeScript | Codebase compilada, zero runtime de framework em produção, surface de ataque reduzida, visualização de observabilidade integrada sem ferramenta externa |
| Auth Model | Cedar ABAC | Verificação matemática formal de políticas, contexto combinado (role + IP + horário + dispositivo + compliance), impossível expressar ambiguidade |
| OLTP | YugabyteDB | PostgreSQL distribuído nativo, ACID completo, sharding automático, tablespaces por país para data residency nativo |
| Cache | DragonflyDB | C++ multi-thread, API Redis-compatible, <0.1ms, escala vertical completa sem configuração adicional |
| Streaming | Redpanda | C++ sem JVM, latência p99 <1ms, API Kafka-compatible, barramento central de audit trail |
| KMS | OpenBao (Linux Foundation) | Fork open source do Vault, MPL 2.0, API idêntica, sem licença proprietária |
| Networking | Cilium / eBPF | Isolamento em nível de kernel, zero overhead de sidecar, mTLS transparente, IP spoofing fisicamente impossível |
| Observabilidade | OTel → VictoriaMetrics + ClickHouse | Pipeline vendor-neutral, single binary, visualização integrada no Dashboard do Cascata sem ferramenta externa |
| Logs / Analytics | ClickHouse | Motor colunar OLAP, compressão 1TB→80GB, queries em bilhões de linhas em segundos, particionamento por tenant nativo |
| Object Storage | MinIO + Camada de Adaptadores | MinIO como backbone soberano; adaptadores para S3, R2, Wasabi, Cloudinary, ImageKit, Google Drive, OneDrive, Dropbox |
| Orquestração | Kubernetes + Operators customizados | Namespace por tenant, provisionamento automático, HPA nativo, multi-cloud por definição |
| Banco Vetorial | Qdrant | Rust, Apache 2.0, HNSW com quantização (até 32x redução de footprint), payload filtering dentro do índice (multi-tenancy sem overhead), modo distribuído nativo, snapshots para MinIO |
| DR Orchestrator | Custom Go Service | Runbooks determinísticos por tier, zero heurística, integrado ao Tenant Router, audit trail em ClickHouse, chaos drills automatizados |
| Realtime Engine | Centrifugo | Go, MIT, consome Redpanda nativamente, WebSocket/SSE/HTTP-Streaming, presença e histórico com recovery, 50K conexões/100MB RAM |
| Webhook Queue | Redpanda (consumer dedicado) | Sem nova dependência de infraestrutura — Redpanda já é o barramento central. Retry state no DragonflyDB. Dead letter no ClickHouse |
| Webhook Filter Engine | Rust (Pingora) | Avaliação de filtros na origem, zero overhead de runtime externo, decisão tomada antes de qualquer chamada HTTP |
| Modelo de distribuição | Open Source (MIT) | Soberania total do operador. Sem telemetria, sem feature gate, sem dependência do mantenedor para operar |
| Push Notifications | APNs + FCM + WebPush nativos | Três protocolos cobrem 100% dos dispositivos. Credentials gerenciadas pelo OpenBao. Pipeline unificado via Redpanda — sem novo sistema de fila |
| Response Rule Engine | Pingora (caminho crítico síncrono) | Interceptação acontece antes do retorno ao cliente, no mesmo processo que validou a request. Timeout hard limit protege o RTT do cliente |
| Rate Limit Nerf | DragonflyDB + Pingora | Estado do consumo por chave/grupo/usuário no DragonflyDB (<0.1ms lookup). Decisão de block ou nerf executada no Pingora antes de qualquer acesso ao banco |
| Thundering Herd | Staggered Warming com jitter determinístico | Elimina contenção na origem. Determinístico = testável e previsível |
| Statement Timeout | Configurado na root da engine por tier | Proteção no pooler — inviolável independente do código da aplicação |
| Pool Exhaustion | Fila limitada com backpressure em 3 estágios | Protege memória E gera sinal para promoção de tier. Sem limite de fila = risco de OOM |
| Circuit Breaker | Por instância de banco, 3 estados, notificação via Unix socket | Granularidade por tenant. Unix socket garante notificação sem depender de infraestrutura que pode estar falhando |
| Pool Size | Adaptativo via p95 + safety factor + growth trend | Pool fixo por tier desperdiça recursos em tenants leves e insuficiente em tenants pesados. Dados reais do ClickHouse eliminam o conservadorismo desnecessário |
| Soft Delete Storage | Schema `_recycled` dedicado | Separação física de tabelas ativas e recicladas. RLS bloqueia acesso direto ao schema `_recycled` |
| Computed Columns — banco | PostgreSQL Generated Columns STORED | Nativo ao YugabyteDB, indexável, zero overhead de leitura, validado em criação |
| Computed Columns — API | Avaliador de expressões Rust no Pingora | Acesso a claims JWT e payload completo. Sem eval — avaliador próprio e seguro |
| Data Validation | Pingora antes de query ao banco | Mensagens legíveis, contexto JWT, cross-field, zero queries desnecessárias ao banco em caso de falha. Constraints do banco como última linha de defesa |
| Schema Metadata Storage | YugabyteDB do Control Plane + cache DragonflyDB | Control Plane — fonte de verdade. DragonflyDB — performance de leitura para Pingora e MCP Server sem query ao banco de controle a cada request |
| Imagem YugabyteDB — cluster compartilhado | `cascata/yugabytedb:shared` (lean) | Footprint mínimo para cluster onde cada MB é recurso coletivo de centenas de tenants NANO/MICRO. Extensões pesadas ausentes por design |
| Imagem YugabyteDB — instância dedicada | `cascata/yugabytedb:full` (completa) | Todo tenant com instância dedicada tem todos os profiles compilados. Zero friction, zero rolling upgrade, zero limitação do SQL Editor por escolha de profile no provisionamento |
| pgvector | Bloqueado em ambas as imagens | Substituído pelo Qdrant — isolamento multi-tenant correto, performance superior com HNSW + quantização, DR integrado ao Orchestrator. pgvector em banco compartilhado cria superfície de ataque sem equivalente ao payload filtering do Qdrant |
| pg_cron em YugabyteDB distribuído | Wrapper via Control Plane | pg_cron não opera nativamente em cluster distribuído sem schema compartilhado. O Control Plane abstrai o agendamento — tenant usa API de Cron Jobs do Cascata, Control Plane usa pg_cron como executor com isolamento por tenant garantido |
| Habilitação de extensão | Padrão pending→active (duas fases explícitas) | A intenção é registrada no CP antes da execução. O reconciliador não detecta inconsistência — finaliza operações interrompidas com estado explícito. Sem comparação de fontes, sem heurística. |