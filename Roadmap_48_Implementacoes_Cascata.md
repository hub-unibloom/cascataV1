# Roadmap de Implementação — Cascata "Koenigsegg"
# 48 Implementações Ordenadas por Prioridade Fundacional

> **Princípio de ordenação:** Nada construído em uma etapa deve ser quebrado ou refatorado por uma etapa posterior. A ordem respeita dependências técnicas, segurança como fundação, e experiência do desenvolvedor como produto.

---

## ETAPA 0 — Alicerce Invisível
*O sistema quebra em produção real sem esses itens. São invisíveis para o tenant mas críticos para o operador. Devem existir antes de qualquer tenant externo usar o sistema.*

### 0.1 — Connection Pooling
**Por que é fundação:** Em modo Shelter com múltiplos tenants NANO/MICRO, cada request abre uma conexão direta no YugabyteDB. Sem pooling, o banco satura sob concorrência real antes do sistema ser útil para qualquer cenário de produção.
**Decisão:** PgBouncer em modo transaction pooling, gerenciado pelo Control Plane, transparente para o tenant. Pool dedicado por tier a partir de STANDARD.

### 0.2 — Egress Filtering para Functions
**Por que é fundação:** Uma Sandbox Function publicada hoje pode exfiltrar dados para qualquer endpoint externo — não existe política de egress. Isso é uma vulnerabilidade de segurança aberta que precisa existir antes do primeiro tenant publicar uma função.
**Decisão:** O tenant declara no manifesto da função quais domínios externos ela pode contatar. Cilium eBPF bloqueia qualquer tráfego de saída da sandbox não declarado no manifesto. Validado no Pingora antes da publicação.

### 0.3 — Vulnerability Scanning de Functions
**Por que é fundação:** Código submetido por tenant não pode ser publicado sem análise estática. Dependências com CVEs críticos, padrões de code injection, tentativas de bypass de sandbox.
**Decisão:** Pipeline de scan em Go executado no Control Plane antes de qualquer publicação. Resultado bloqueante — função com vulnerabilidade crítica não é publicada. Resultado disponível no painel com detalhe do que foi encontrado.

### 0.4 — Soft Delete / Recycle Bin
**Por que é fundação:** O schema de storage de metadados de tabelas precisa ser definido com suporte a soft delete desde o início. Adicionar depois exige migração de todas as tabelas existentes.
**Decisão:** Tabelas deletadas movidas para namespace `_recycled_` com timestamp. Purge permanente requer confirmação por senha do operador. Retenção configurável por tier.

### 0.5 — Computed / Virtual Columns
**Por que é fundação:** A estrutura interna de descrição de schema (usada pelo TableCreator, APIDocs, SDK type generator, MCP Server) precisa saber desde o início que uma coluna pode ser virtual. Adicionar depois quebra o type generator e o MCP schema introspection.
**Decisão:** Coluna marcada como `computed` no schema metadata do Cascata. Expressão SQL armazenada no YugabyteDB como generated column. Exposta no SDK como campo read-only com tipo inferido.

### 0.6 — Data Validation Rules na Camada API
**Por que é fundação:** Validações definidas no Pingora precisam ser descritas no schema metadata — o mesmo usado pelo SDK type generator e APIDocs. Se o schema metadata não tiver esse campo desde o início, adicionar depois quebra a geração de tipos.
**Decisão:** Regras de validação declaradas no painel por coluna: regex, range numérico, lista de valores permitidos, validações cruzadas entre campos. Executadas no Pingora antes de qualquer escrita. Expostas no SDK como constraints visíveis em compile time.

### 0.7 — Downgrade Automático de Tier
**Por que é fundação:** O Smart Tenant Classification Engine só está metade especificado — sabe promover, não sabe recuar. O algoritmo de classificação precisa das duas direções desde o início para não tomar decisões de provisionamento erradas.
**Decisão:** O Control Plane monitora métricas de consumo continuamente. Downgrade proposto automaticamente quando tenant fica abaixo dos thresholds por N dias consecutivos. Operador aprova no painel — nunca automático sem confirmação humana ou de agente autorizado.



### 0.8 — Extensions Marketplace
**Dependências:** 6.2
**O quê:** Gestão de extensões do YugabyteDB diretamente no painel.
**Sub-etapas:**
- a) Lista de extensões disponíveis com descrição e impacto em performance
- b) Instalação e remoção com um clique
- c) Alertas de extensões com CVEs conhecidos

---

## ETAPA 1 — Fundação da Experiência do Desenvolvedor
*O primeiro contato do desenvolvedor com o Cascata. Se essa etapa for ruim, o produto não é adotado independente de quão poderoso seja o backend.*

### 1.1 — Tenant Provisioning Wizard
**Dependências:** Etapa 0 completa.
**O quê:** Wizard de criação de projeto que define a topologia desde o início — tier, região, compliance, auth providers, storage, comunicação, inteligência. O projeto nasce configurado para o que é.
**Sub-etapas:**
- a) Wizard de 8 passos no painel
- b) Lógica de sugestão de tier baseada nas respostas
- c) Ativação automática de namespaces de compliance (PHI, PCI) baseada nas seleções
- d) Provisionamento dos recursos exatos declarados — sem sobrealocação

### 1.2 — Draft / Live Environments
**Dependências:** 1.1
**O quê:** Ambiente Draft isolado por projeto onde o tenant desenvolve e testa schema changes sem afetar produção. Sincronização opcional de dados do Live para o Draft.
**Sub-etapas:**
- a) Provisionamento do ambiente Draft pelo Control Plane
- b) Toggle de Live Data Sync no painel (mirror read-only do Live para Draft)
- c) Deploy Wizard: Safe Merge (DDL only + data rules) vs Destructive Swap
- d) Schema Diff Viewer antes do deploy: changes, security policies, raw SQL
- e) Rollback de deploy com um clique

### 1.3 — Database Branching
**Dependências:** 1.2
**O quê:** Múltiplos branches do banco simultaneamente, além do par Draft/Live. Para times com múltiplas features em desenvolvimento paralelo.
**Sub-etapas:**
- a) Branch criado a partir de qualquer ponto no tempo via PITR
- b) Merge de branch de volta ao trunk com diff visual
- c) Branches listados no painel com status, criador, e data de expiração configurável
- d) Integração com CI/CD: branch criado automaticamente por pull request

### 1.4 — Schema Migrations Completo
**Dependências:** 1.2
**O quê:** O fluxo completo de evolução do schema sem downtime, com análise de impacto antes de executar.
**Sub-etapas:**
- a) `cascata db diff` — detecta mudanças e gera migration SQL
- b) Protocol Cascata — análise de impacto em cascata: foreign keys afetadas, RPCs quebradas, políticas RLS impactadas
- c) `cascata db push` com preview e confirmação
- d) Canary deployment de migration: % do tráfego na nova versão antes do rollout
- e) `cascata db rollback` — reverter última migration aplicada

### 1.5 — Schema Testing Framework
**Dependências:** 1.3, 1.4
**O quê:** Escrever e rodar testes de infraestrutura: validar que políticas RLS bloqueiam acesso correto, testar que uma migration não quebra queries existentes.
**Sub-etapas:**
- a) API de testing no SDK: `cascata.test.asUser(id).from('tabela').select()` → deve retornar X linhas
- b) Runner de testes integrado ao `cascata dev`
- c) Integração com CI/CD: testes de schema rodam antes de qualquer deploy

### 1.6 — CI/CD Pipeline
**Dependências:** 1.4, 1.5
**O quê:** O Cascata como parte do pipeline de deploy do tenant, não só uma ferramenta local.
**Sub-etapas:**
- a) GitHub Actions action oficial: `cascata/deploy-action`
- b) GitLab CI template oficial
- c) Sequência padrão: criar branch → rodar testes → diff → deploy para staging → promoção manual para prod
- d) Secrets de CI gerenciados pelo OpenBao do projeto

### 1.7 — Query Performance Analyzer
**Dependências:** 1.1
**O quê:** EXPLAIN ANALYZE visual no painel, queries lentas identificadas em tempo real, índices sugeridos.
**Sub-etapas:**
- a) Captura de slow queries (>N ms configurável) no Pingora com log para ClickHouse
- b) EXPLAIN ANALYZE visual no SQL Editor do painel
- c) Sugestão automática de índices baseada em queries frequentes
- d) Alert quando query média de uma tabela degrada mais de X% em relação à baseline

---

## ETAPA 2 — Completude de API e Protocolos
*O Cascata expõe três formatos de resposta e suporta múltiplas versões simultâneas. Precisa ser definido antes que qualquer SDK seja considerado estável.*

### 2.1 — Três Formatos de Resposta: REST, GraphQL e TOON
**Dependências:** Etapa 1 completa, SDK base.
**O quê:** Toda query no Cascata pode retornar em três formatos.
**Sub-etapas:**
- a) **REST/JSON** (padrão atual) — sem mudança
- b) **GraphQL** — endpoint `/graphql` gerado automaticamente a partir do schema. Mutations, queries e subscriptions. Atualizado automaticamente quando o schema muda
- c) **TOON (Token-Oriented Object Notation)** — formato colunar para consumo eficiente por modelos de IA. `{ "columns": [...], "rows": [[...]] }`. Redução de ~70% de tokens vs JSON padrão em resultsets grandes
- d) SDK expõe `.format('json' | 'graphql' | 'toon')` em toda query
- e) MCP Server expõe `query.format` como parâmetro de toda tool call

### 2.2 — API Versioning
**Dependências:** 2.1
**O quê:** Múltiplas versões da API do projeto em paralelo sem forçar migração imediata.
**Sub-etapas:**
- a) Tenant declara versões ativas no painel: `/v1/`, `/v2/`
- b) Schema snapshots associados a cada versão
- c) Deprecation notice configurável: header `X-Cascata-Deprecation` em requests para versões antigas
- d) Traffic analytics por versão — percentual do tráfego ainda em v1 vs v2

### 2.3 — API Docs Auto-Gerada
**Dependências:** 2.1, 2.2
**O quê:** Documentação Swagger/OpenAPI gerada automaticamente do schema, com snippets de código para todos os SDKs e integrações no-code.
**Sub-etapas:**
- a) OpenAPI 3.1 gerado automaticamente e atualizado a cada migration
- b) Snippets para `@cascata/client`, `@cascata/compat`, fetch nativo, Python, Swift, Kotlin
- c) Seção de integração no-code: FlutterFlow, AppSmith, Bubble, n8n
- d) AI Technical Writer — agente interno que gera descrição em linguagem natural de cada endpoint
- e) Playground interativo diretamente na APIDocs

### 2.4 — Full-Text Search Nativo
**Dependências:** 2.1
**O quê:** Busca textual em colunas via índices GIN do YugabyteDB, configurável no painel.
**Sub-etapas:**
- a) Tenant marca colunas como `searchable` no painel
- b) Cascata cria índice GIN automaticamente e mantém `tsvector` atualizado
- c) SDK expõe `.textSearch('coluna', 'query')` com ranking por relevância
- d) Suporte a múltiplos idiomas (pt-BR, en-US) com dicionário configurável por projeto

### 2.5 — Foreign Data Wrappers
**Dependências:** 2.1
**O quê:** Conectar o YugabyteDB do projeto a fontes externas como tabelas locais.
**Sub-etapas:**
- a) Suporte a FDW para PostgreSQL externo, MySQL, e REST API (via Multicorn)
- b) Configuração no painel com credenciais gerenciadas pelo OpenBao
- c) Tabela externa aparece no Database Explorer como qualquer tabela — com RLS aplicado
- d) Aviso de performance no painel para queries que cruzam FDW

### 2.6 — Import / Migration de Outros BaaS
**Dependências:** 2.1, 1.4
**O quê:** Ferramenta para migrar projetos existentes para o Cascata sem reescrever código.
**Sub-etapas:**
- a) Import de dump Supabase (pg_dump + storage + auth users)
- b) Import de Firebase (Firestore → YugabyteDB com mapeamento de estrutura, Firebase Auth → auth.identities)
- c) Validação pós-import: contagem de registros, integridade referencial, teste de auth
- d) Modo dry-run com relatório de compatibilidade antes de executar

---

## ETAPA 3 — Segurança Profunda e Compliance
*Sem essa etapa completa, o Cascata não pode ser instalado em ambiente regulado. É o que separa um BaaS de um BaaS enterprise.*

### 3.1 — Right to Erasure Completo
**Dependências:** Todas as etapas anteriores.
**O quê:** O direito ao esquecimento da LGPD/GDPR executado como fluxo único que apaga dados de TODOS os sistemas simultaneamente.
**Sistemas cobertos:** YugabyteDB (crypto-shredding), ClickHouse (audit trail do usuário), Qdrant (vetores com payload do usuário), MinIO (arquivos do usuário), DragonflyDB (sessões e cache), backups (marcação para exclusão no próximo ciclo)
**Sub-etapas:**
- a) Endpoint `DELETE /auth/users/{id}/erase` que inicia o fluxo
- b) Orquestrador de erasure em Go que coordena todos os sistemas com transaction-like semantics
- c) Relatório de erasure gerado ao final: o quê foi apagado, em qual sistema, com timestamp
- d) Relatório exportável como prova de compliance para o usuário que solicitou

### 3.2 — Data Portability
**Dependências:** 3.1
**O quê:** O usuário final pode solicitar exportação de todos os seus dados em formato legível por máquina.
**Sub-etapas:**
- a) Endpoint `GET /auth/users/{id}/export` que agrega dados de todos os sistemas
- b) Formato de exportação: JSON estruturado com todos os dados do usuário em todas as tabelas onde aparece
- c) Download disponível por link assinado com TTL de 24h (via MinIO)
- d) Notificação ao usuário quando exportação estiver pronta

### 3.3 — Data Masking / PII em Logs
**Dependências:** 3.1, 3.2
**O quê:** Colunas marcadas como PII são automaticamente ofuscadas em todos os sistemas de log.
**Sub-etapas:**
- a) Tenant marca colunas como `pii` no painel (nome, email, CPF, telefone, etc.)
- b) Pingora intercepta respostas e aplica masking antes de gravar no ClickHouse
- c) Diferentes níveis: `redact` (substitui por `[REDACTED]`), `hash` (SHA-256 one-way para correlação), `partial` (mostra apenas primeiros/últimos N chars)
- d) Break-glass: operador com permissão especial pode ver dados reais com audit trail imutável do acesso

### 3.4 — IP Allowlist / Blocklist por Projeto
**Dependências:** 3.3
**O quê:** Controle de acesso por IP configurável pelo tenant no painel.
**Sub-etapas:**
- a) Allowlist: apenas IPs/ranges declarados podem acessar o projeto
- b) Blocklist: IPs banidos manualmente ou automaticamente após N tentativas de auth falhas
- c) Auto-ban configurável: "bloquear IP que gerar mais de X erros 401 em Y minutos"
- d) Geo-blocking: bloquear acesso de países específicos (útil para compliance de data residency)

### 3.5 — Session Management Avançado
**Dependências:** 3.3
**O quê:** Controle granular de sessões ativas por usuário.
**Sub-etapas:**
- a) Painel do tenant lista todas as sessões ativas de um usuário: dispositivo, IP, última atividade
- b) Força logout de sessão específica ou de todas as sessões
- c) Limite configurável de sessões simultâneas por usuário
- d) Sessão ancorada a IP/device fingerprint — alerta em nova localização

### 3.6 — SAML / Enterprise SSO para Equipe
**Dependências:** 3.5
**O quê:** Login corporativo para membros da equipe do operador via SAML 2.0.
**Sub-etapas:**
- a) Configuração de Identity Provider SAML no painel (Okta, Azure AD, Google Workspace)
- b) Provisionamento automático de membros de equipe via SCIM
- c) Mapeamento de grupos do IdP para papéis do Cascata
- d) Just-in-time provisioning: membro criado automaticamente no primeiro login via SSO

### 3.7 — HSM para SOVEREIGN
**Dependências:** 3.6
**O quê:** Suporte a Hardware Security Module físico para chaves raiz em ambientes SOVEREIGN.
**Sub-etapas:**
- a) OpenBao configurado com backend HSM (YubiHSM 2, Thales Luna, AWS CloudHSM)
- b) Chaves raiz nunca materializadas em software — geradas e armazenadas no hardware
- c) Documentação de integração para os principais HSMs do mercado
- d) Certificação de que nenhuma chave SOVEREIGN pode ser extraída sem acesso físico ao hardware

### 3.8 — Compliance Report
**Dependências:** 3.7
**O quê:** Relatório estruturado de compliance sob demanda, assinado digitalmente, para auditoria regulatória.
**Sub-etapas:**
- a) Relatório LGPD: acessos a dados de um usuário específico no período solicitado
- b) Relatório PCI-DSS: todas as operações em tabelas com dados de pagamento
- c) Relatório HIPAA: todos os acessos a dados PHI com identidade do acessador
- d) Assinatura digital do relatório via OpenBao
- e) Exportação em PDF estruturado e JSON para sistemas de GRC

### 3.9 — SOC 2 / ISO 27001 Audit Export
**Dependências:** 3.8
**O quê:** Evidências de controle para auditorias de certificação.
**Sub-etapas:**
- a) Mapeamento automático de controles do Cascata para frameworks SOC 2 Type II e ISO 27001
- b) Evidências exportáveis por controle: logs, configurações, políticas ativas
- c) Relatório de gaps: controles que precisam de ação do operador para certificação

---

## ETAPA 4 — Operacional e Produção
*O Cascata em escala real, gerenciando muitos projetos com autonomia máxima e mínima intervenção humana.*

### 4.1 — Cron Jobs / Funções Agendadas
**Dependências:** Etapa 3 completa.
**O quê:** Execução de Sandbox Functions em horários e intervalos configurados.
**Sub-etapas:**
- a) Interface de criação de cron no painel com cron expression e preview de próximas execuções
- b) Timezone por projeto respeitada (UTC para armazenamento, timezone do tenant para exibição)
- c) Histórico de execuções com status, duração e output no ClickHouse
- d) Alert automático em falha consecutiva de N execuções
- e) Retry configurável por job com política de backoff

### 4.2 — Alerting Configurável pelo Tenant
**Dependências:** 4.1, Central de Comunicação
**O quê:** O tenant define thresholds e recebe alertas pelos canais que configurou.
**Sub-etapas:**
- a) Alertas de performance: error rate, latência p99, throughput
- b) Alertas de capacidade: storage, requests/hour, conexões
- c) Alertas de segurança: auth failures, IP ban, ABAC blocks
- d) Alertas customizados: qualquer métrica do ClickHouse com query configurável
- e) Delivery via Central de Comunicação (push, email, webhook)

### 4.3 — Secrets Manager para Functions
**Dependências:** 4.1
**O quê:** Variáveis de ambiente seguras para Sandbox Functions, gerenciadas pelo OpenBao.
**Sub-etapas:**
- a) Interface no painel para criar e gerenciar secrets por projeto
- b) Secrets injetados como variáveis de ambiente no processo da função — nunca no código
- c) Rotação de secret sem necessidade de redeploy da função
- d) Audit trail de todo acesso a secret por função e por execução

### 4.4 — Feature Flags Nativos
**Dependências:** 4.2
**O quê:** Habilitar/desabilitar features por usuário ou grupo sem deploy.
**Sub-etapas:**
- a) Interface de criação de flags no painel: nome, descrição, estado default
- b) Targeting: habilitar para % aleatório dos usuários, para grupos específicos, para usuários individuais
- c) SDK expõe `cascata.flags.isEnabled('nome_da_flag')` como primitivo de primeira classe
- d) Flags avaliadas via Cedar ABAC — podem combinar com atributos do usuário
- e) Histórico de mudanças de flag com audit trail

### 4.5 — Tenant Cloning
**Dependências:** 4.3
**O quê:** Criar cópia exata de um projeto para staging, onboarding de novo cliente, ou template.
**Sub-etapas:**
- a) Clone completo: schema + configurações + políticas RLS + webhooks + funções
- b) Clone com dados: inclui snapshot dos dados do projeto original
- c) Clone sem dados: apenas estrutura (adequado para onboarding de novo cliente)
- d) O clone recebe um novo ID e endpoint — completamente independente do original

### 4.6 — Tenant Hibernation
**Dependências:** 4.5
**O quê:** Tenant inativo reduz footprint automaticamente, acorda quando recebe tráfego.
**Sub-etapas:**
- a) Threshold de inatividade configurável pelo operador (default: 30 dias sem tráfego)
- b) Hibernação: YugabyteDB reduz para modo de baixo consumo, DragonflyDB libera keyspace, Centrifugo libera conexões
- c) Wake-on-request: primeira request após hibernação acorda o tenant em <5s
- d) Notificação ao tenant antes da hibernação com N dias de antecedência

### 4.7 — Custom Domain por Projeto
**Dependências:** 4.6
**O quê:** Tenant enterprise usa `api.minhaaplicacao.com` em vez do domínio do operador.
**Sub-etapas:**
- a) Interface para configurar domínio customizado no painel
- b) TLS automático via Let's Encrypt gerenciado pelo Cascata
- c) Certificado armazenado no OpenBao do projeto
- d) CNAME validation antes de ativar o domínio
- e) Wildcard para projetos que queiram `{slug}.minhaaplicacao.com`

### 4.8 — Templates de Projeto
**Dependências:** 4.7
**O quê:** Projetos pré-configurados para casos de uso comuns.
**Sub-etapas:**
- a) Biblioteca de templates: e-commerce, saúde, educação, delivery, fintech, SaaS B2B
- b) Cada template inclui: schema completo, políticas RLS, funções, webhooks de exemplo, documentação
- c) Tenant seleciona template no Provisioning Wizard ou na criação de novo projeto
- d) Contribuição de templates pela comunidade via GitHub

### 4.9 — Workflow Orchestration Multi-Step
**Dependências:** 4.8
**O quê:** Workflows com estado e múltiplos passos gerenciados pelo Cascata, sem exigir máquina de estados no código do tenant.
**Sub-etapas:**
- a) Interface visual de criação de workflow: nós de trigger, condição, ação, espera, ramificação
- b) Estado do workflow persistido no YugabyteDB com RLS por tenant
- c) Timeout e SLA por etapa com ação configurável em caso de violação
- d) Instâncias de workflow visíveis no painel: em andamento, concluídas, falhas
- e) SDK expõe `cascata.workflows.create()` e `cascata.workflows.advance()`

---

## ETAPA 5 — AI-First Avançado
*Além do que já definimos: protocolo A2A, cache semântico, e gestão de prompts como primitivos do sistema.*

### 5.1 — Protocolo A2A (Agent-to-Agent)
**Dependências:** Etapa 4 completa, blocos AI-First do SRS/SAD.
**O quê:** Protocolo padronizado para que agentes se comuniquem entre si dentro do Cascata, com identidade, permissão e rastreabilidade.
**Sub-etapas:**
- a) Definição do protocolo: como um agente descobre outros agentes no mesmo projeto
- b) Handshake de autorização A2A via Cedar ABAC: agente A pode delegar tarefa para agente B apenas se tenant autorizou essa relação
- c) Message passing estruturado entre agentes com schema validado
- d) Audit trail de toda comunicação A2A: quem delegou, para quem, o quê, resultado
- e) Loop detection: o sistema detecta ciclos de delegação e interrompe antes de loop infinito

### 5.2 — Semantic Caching para LLM
**Dependências:** 5.1, Qdrant
**O quê:** Quando dois prompts são semanticamente equivalentes, retornar o resultado cacheado.
**Sub-etapas:**
- a) Embedding da query gerado antes de chamar o modelo
- b) Busca no Qdrant por similaridade coseno com threshold configurável (ex: >0.97 = cache hit)
- c) Cache hit retorna resposta anterior sem chamar o modelo
- d) TTL configurável por coleção — respostas sobre dados voláteis expiram mais rápido
- e) Métrica de hit rate visível no painel do tenant

### 5.3 — Prompt Management e Versionamento
**Dependências:** 5.2
**O quê:** Prompts de sistema gerenciados como código — versionados, auditáveis, com rollback.
**Sub-etapas:**
- a) Interface no painel para criar e editar prompts por agente
- b) Versionamento semântico de prompts com histórico completo
- c) A/B testing de prompts: % do tráfego usa v1, % usa v2
- d) SDK expõe `cascata.agents.getPrompt('nome', 'v2.1')` — agente referencia prompt por nome, não por conteúdo hardcoded
- e) Rollback de prompt com um clique em caso de regressão

### 5.4 — Token Counting por Agente
**Dependências:** 5.3
**O quê:** Contagem e armazenamento de tokens consumidos por agente — útil para detectar providers com comportamento anômalo e para o tenant entender seus custos de IA.
**Sub-etapas:**
- a) Captura de tokens de input e output de cada call ao modelo (via response headers do provider)
- b) Armazenamento no ClickHouse por agente, por modelo, por período
- c) Visualização no painel: tokens/dia, tokens/operação, distribuição por agente
- d) Alert configurável: "me avise se consumo de tokens dobrar em relação à semana anterior"

---

## ETAPA 6 — Paridade com V0 e Features Complementares
*Features do MVP que não foram formalizadas nos documentos + features de médio prazo que completam o produto.*

### 6.1 — SQL Editor com AI
**Dependências:** Etapa 5 completa.
**O quê:** Console SQL no painel com assistência de IA integrada.
**Sub-etapas:**
- a) Editor com syntax highlighting, autocompletion de schema, e formatação automática
- b) Fix assistido por IA: "esta query tem um erro, sugestão de correção"
- c) Smart Refresh: detecta DDL commands (`CREATE/ALTER/DROP`) e auto-atualiza o schema tree
- d) Histórico de queries executadas com tempo de execução
- e) Export de resultado em CSV, JSON, XLSX diretamente do editor

### 6.2 — RLS Designer Visual
**Dependências:** 6.1
**O quê:** Builder visual de políticas RLS com topology graph e security score.
**Sub-etapas:**
- a) Grafo visual: usuário → role → acesso a tabela → policy
- b) Security Score: análise heurística do rigor das políticas configuradas
- c) Simulador: "como este usuário enxerga esta tabela?" — executa a query como o usuário escolhido
- d) Deploy de política direto do designer


### 6.3 — Tenant-to-Tenant Data Sharing
**Dependências:** 6.3
**O quê:** Um tenant pode expor dados para outro tenant da mesma instância com políticas ABAC controlando o acesso.
**Sub-etapas:**
- a) Tenant A declara tabelas ou views como "compartilháveis"
- b) Tenant B solicita acesso — Tenant A aprova no painel
- c) Cedar ABAC do Tenant A controla quais dados do B são visíveis
- d) Audit trail de todo acesso cross-tenant

### 6.4 — Multi-Region como Feature do Tenant
**Dependências:** 6.4
**O quê:** Tenant ENTERPRISE configura em qual região ficam seus dados — não só por DR, mas por latência e compliance simultâneos.
**Sub-etapas:**
- a) Seleção de regiões no Provisioning Wizard e nas configurações do projeto
- b) YugabyteDB tablespaces mapeados para regiões escolhidas pelo tenant
- c) Roteamento automático de reads para região mais próxima do cliente
- d) Pinning de dados específicos por regulação: "dados de usuários europeus apenas em Frankfurt"

### 6.5 — Offline-First no SDK
**Dependências:** 6.5
**O quê:** `@cascata/client` com suporte a operações offline que sincronizam quando a conexão retorna.
**Sub-etapas:**
- a) Queue local de operações pendentes quando offline
- b) Sync automático na reconexão com conflict resolution configurável
- c) Indicador de estado de sync exposto no SDK para o tenant mostrar na UI

### 6.6 — Comentários e Documentação Inline no Schema
**Dependências:** 6.1
**O quê:** Anotar tabelas e colunas com descrição em linguagem natural.
**Sub-etapas:**
- a) Interface no painel para adicionar comentários por tabela e coluna
- b) Armazenado como `COMMENT ON` no YugabyteDB
- c) Aparece na APIDocs gerada e no hover do SQL Editor
- d) AI Technical Writer pode gerar comentários automaticamente a partir do nome e tipo da coluna

---

## Resumo das Etapas

| Etapa | Nome | Itens | Dependência |
|-------|------|-------|-------------|
| 0 | Alicerce Invisível | 7 | Nenhuma — primeiro a implementar |
| 1 | Fundação do Desenvolvedor | 7 | Etapa 0 |
| 2 | API e Protocolos | 6 | Etapa 1 |
| 3 | Segurança e Compliance | 9 | Etapa 2 |
| 4 | Operacional e Produção | 9 | Etapa 3 |
| 5 | AI-First Avançado | 4 | Etapa 4 |
| 6 | Paridade e Complementares | 7 | Etapa 5 |
| **Total** | | **49** | |

> **Nota:** O número chegou a 49 na ordenação porque o item 2.1 (Três Formatos de Resposta) unificou REST + GraphQL + TOON em uma única implementação, absorvendo o que antes eram itens separados. O resultado final cobre os 48 temas acordados com uma implementação extra justificada pela coesão técnica.

---

## Princípios que guiaram a ordenação

1. **Segurança antes de features** — Etapa 0 fecha vulnerabilidades abertas antes de qualquer tenant externo usar o sistema
2. **Schema imutável antes de conteúdo** — Computed columns, validation rules e soft delete precisam estar no schema metadata desde o início. Adicionar depois exige migração de toda a base
3. **DX antes de enterprise** — O desenvolvedor precisa ter uma experiência boa antes de oferecer features enterprise. Um produto difícil de usar não é adotado independente de quão poderoso seja
4. **Compliance como pré-requisito de escala** — Sem Right to Erasure, Data Portability e Data Masking, o Cascata não pode ser instalado em ambiente regulado. Isso precisa existir antes de buscar adoção enterprise
5. **AI-First só depois que o sistema é sólido** — A2A, Semantic Caching e Prompt Management assumem que banco, auth, storage, webhooks e compliance já funcionam perfeitamente. Um sistema de IA sobre um sistema frágil só amplifica os problemas
