# Especificação de Requisitos de Software (SRS)
# Cascata — "Koenigsegg"

> **Propósito:** O Cascata não é um BaaS convencional. É um orquestrador Multi-Tenant desenhado para operar a mesma API para um desenvolvedor solo em uma VPS de $20 e para um banco regulamentado em infraestrutura distribuída multi-cloud — sem mudança de código cliente, sem downtime de migração, sem comprometer a segurança em nenhum tier. O verdadeiro orquestrador é o usuário: o Cascata pavimenta os caminhos, o tenant decide como, onde e com quais ferramentas o projeto dele funciona.

### Modelo de Distribuição: Open Source

O Cascata é software livre e de código aberto, distribuído sob licença MIT. Não existe versão paga, plano premium, feature gate ou cobrança por uso. O modelo é: clone o repositório, instale na sua infraestrutura, opere.

Contribuições são aceitas via GitHub nas seguintes categorias:
- Correções de bugs com reprodução documentada
- Patches de segurança com disclosure responsável
- Melhorias de performance com benchmark comparativo
- Documentação e traduções

O projeto não aceita contribuições que introduzam dependências proprietárias, telemetria não declarada, ou que comprometam o modelo de soberania de dados que é a fundação do Cascata.

O operador que instala o Cascata é soberano sobre sua instância. O código que roda na sua infraestrutura é o mesmo que está no repositório público — sem backdoors, sem telemetria oculta, sem licença que expire.

---

## 1. Visão Geral do Sistema e Filosofia

### 1.1 Filosofia Central

O Cascata não impõe forma. Ele fornece estrutura.

A maioria dos BaaS existentes decide por você: qual modelo de auth usar, como seus dados são isolados, onde seus arquivos vão parar, qual formato de política de acesso você pode expressar. Isso cria um teto invisível — o produto cresce até o limite que o BaaS permite, e depois enfrenta uma migração dolorosa.

O Cascata inverte esse modelo. Cada tenant configura no painel os caminhos que seu projeto vai percorrer. O Cascata fornece as ferramentas, garante a segurança de cada caminho escolhido e escala a infraestrutura conforme o projeto cresce — sem que o desenvolvedor troque uma linha de código, sem downtime, sem contrato novo.

A plataforma abstrai sua estrutura distribuída através de uma **Interface Única** baseada no Control Plane. Tenants são classificados automaticamente em 5 tiers de isolamento. A promoção entre tiers pode ser automática ou acionada manualmente pelo operador, sempre sem downtime e sem alteração de código no cliente.

### 1.2 Os 5 Tiers de Isolamento

---

**Req-1.0 — NANO**
Perfil: Desenvolvedores, MVPs, projetos experimentais, protótipos.

- Alocação automática no YugabyteDB Shared Cluster
- Isolamento de dados via Row Level Security (RLS) nativo do YugabyteDB — cada tenant enxerga apenas seus próprios dados, sem código adicional no cliente
- Todos os serviços do Cascata rodam em containers no host único (modo Shelter)
- Sem custo de infra adicional para o operador do Cascata neste tier
- Acesso completo ao painel, SDK e todas as features de configuração — o limite é de infraestrutura, não de funcionalidade

---

**Req-1.1 — MICRO**
Perfil: Pequenos SaaS com usuários reais, produtos em fase de tração.

- Schema isolation dedicado dentro do cluster YugabyteDB compartilhado — banco lógico próprio
- Rate Limiting dedicado por tenant no Pingora Gateway
- Namespace ClickHouse próprio para logs, analytics e audit trail
- DragonflyDB com keyspace isolation — sem instância dedicada, mas sem contaminação entre tenants
- Backup agendado configurável pelo operador

---

**Req-1.2 — STANDARD**
Perfil: SaaS com SLA contratado, volume real de transações, dados sensíveis em crescimento.

- Database isolation completo — instância YugabyteDB exclusiva para o tenant
- Instância DragonflyDB dedicada para cache, session management e distributed locking
- Backup automático com Point-in-Time Recovery (PITR) ativo — restauração para qualquer ponto nos últimos N dias
- Audit trail completo com retenção configurável no ClickHouse
- Rate limiting granular por rota, por usuário e por IP

---

**Req-1.3 — ENTERPRISE**
Perfil: Fintechs, clínicas, startups com dado regulado, empresas com requisito de compliance formal.

- O Control Plane ejeta o projeto para Pods Kubernetes isolados no cloud provider do operador
- Network Policies via Cilium/eBPF — isolamento físico em nível de kernel entre tenants
- mTLS obrigatório em toda comunicação interna (in-transit)
- Nós Kubernetes dedicados — sem compartilhamento de hardware com outros tenants
- Audit trail imutável — logs transacionais não podem ser deletados por nenhum usuário, incluindo administradores do tenant
- SLA garantido com métricas de uptime disponíveis no painel do operador

---

**Req-1.4 — SOVEREIGN**
Perfil: Bancos tier-1, instituições de defesa, órgãos regulados com requisito de soberania total de dados.

- Deploy Air-Gap — execução inteiramente on-premise ou em VPC exclusiva do cliente
- BYOK (Bring Your Own Key) via OpenBao rodando no ambiente físico do cliente — as chaves nunca saem do perímetro do cliente em nenhuma circunstância
- Zero telemetria enviada para infraestrutura do Cascata
- Audit trail fisicamente isolado no ambiente do cliente — o operador do Cascata não tem acesso
- MinIO rodando dentro do VPC do cliente para todo object storage
- OpenBao rodando on-premise para todo gerenciamento de chaves e secrets
- Suporte a inspeção e auditoria independente por terceiros contratados pelo próprio cliente

---

## 2. Requisitos Funcionais

### 2.1 Orquestração — Control Plane

**Req-2.1.1** O Control Plane é o cérebro do Cascata. Governa o ambiente, classifica tenants, roteia para o Data Plane correto, gerencia billing, provisionamento e configuração. Não toca em dados brutos de tenant em nenhuma circunstância — essa fronteira é inviolável por design arquitetural.

**Req-2.1.2** A linguagem do Control Plane é **Go 1.26+**. Concorrência massiva via goroutines sem event loop, binário único com compilação estática, inicialização em milissegundos e zero dependência de ecossistema externo no runtime. O roteador HTTP é **Chi** — minimalista, middleware composável, previsível e performático para um serviço que é o backbone de roteamento de todo o sistema.

**Req-2.1.3** O Dashboard Web — a Developer Experience (DX) do Cascata — deve ser construído em **SvelteKit**. O que vai para produção é JavaScript compilado e otimizado, sem runtime do framework em execução no cliente. TypeScript é a linguagem do SvelteKit por padrão — não é uma camada adicional, é a linguagem nativa do projeto. O dashboard é a interface única através da qual o tenant opera todo o seu projeto: banco de dados, autenticação, storage, funções, logs, billing, compliance e configuração de infra.

**Req-2.1.4** O Dashboard deve renderizar visualizações analíticas pesadas — dados em tempo real do ClickHouse — via WebGL sem bloquear o frame rate da interface. Meta de estabilidade: 120fps em visualizações com bilhões de pontos de dados. Nenhuma ferramenta externa de visualização deve ser necessária — o operador vê tudo dentro do próprio Cascata.

**Req-2.1.5** O API Gateway do Data Plane é **Pingora**, construído em Rust sobre Tokio pela Cloudflare e disponível como projeto open source. Intercepta requests, valida JWT, aplica Rate Limiting consultando DragonflyDB, transforma payloads em SQL AST otimizado antes de encaminhar ao YugabyteDB. Testado em produção processando mais de 1 trilhão de requests por dia. Latência adicionada ao pipeline: <0.5ms p99.

---

### 2.2 Controle de Acesso e Identidade

**Req-2.2.1** O modelo de autorização do Cascata é **ABAC — Attribute-Based Access Control**, implementado com **Cedar Policy Language** (licença Apache 2.0). Cedar é compilado como biblioteca Rust dentro do Cascata, sem dependência de runtime externo. Permite verificação matemática formal de completude e consistência das políticas — uma política ambígua ou contraditória é detectada em tempo de compilação, não em produção. Para tráfego de alta escala de requests de um projeto (latency <0.5ms p99 via Pingora), as políticas ativas de cada tenant são compiladas e serializadas de antemão pelo Control Plane direto no **hot-cache do DragonflyDB**. O Pingora absorve a política resolvendo-a imediatamente da RAM central, não ferindo a consistência das rotas nem causando overload no banco. Mudanças de política pela aplicação causam a reinvalidação da flag no DragonflyDB.

**Req-2.2.2** Políticas de acesso suportam contexto completo e combinado:

```
"Usuário acessa recurso Y SE
  role = MEDICO
  AND paciente.clinica = usuario.clinica
  AND horario IN horario_comercial
  AND dispositivo.verificado = true
  AND ip IN rede_vpn_corporativa
  AND status_compliance = ATIVO"
```

Esses contextos são criados e gerenciados pelo próprio usuário no painel do Cascata, na seção de políticas de acesso.

**Req-2.2.3** O sistema de autenticação do Cascata não impõe modelo ao tenant. Cada projeto define, no painel, os métodos de autenticação que deseja oferecer aos seus usuários finais. Os métodos disponíveis são:

- **Passkeys / FIDO2** — autenticação biométrica ou por chave de hardware, sem senha. Método primário recomendado
- **Magic Link** — link de autenticação enviado por email, sem senha
- **OTP por Email** — código de uso único enviado por email
- **OTP por SMS / Telefone** — código de uso único enviado por SMS
- **CPF + Senha** — para contextos brasileiros onde CPF é o identificador natural
- **Usuário + Senha** — combinação clássica
- **OAuth Providers** — Google, GitHub, Apple, e qualquer provider OAuth2/OIDC configurável pelo tenant
- **TOTP/HOTP** — autenticador via app (Google Authenticator, Authy, etc.) como segundo fator
- **Combinações** — o tenant pode habilitar múltiplos métodos simultaneamente. O Cascata os orquestra de forma transparente

Senhas puras sem segundo fator são permitidas como opção configurável pelo tenant — a decisão de segurança é do operador do projeto, não do Cascata.

**Req-2.2.4 — Anonkeys por Projeto**

Cada tenant recebe uma Anonkey padrão no momento da criação do projeto. O tenant pode criar múltiplas Anonkeys adicionais, cada uma vinculada a um destino de redirecionamento específico (URL de um app web, deep link de um app mobile, etc.).

Quando um OAuth provider (Google, Apple, GitHub) recebe uma Anonkey específica e devolve o token ao Cascata, o sistema resolve automaticamente o redirect correto para o app ou site de origem — sem expor URLs internas, sem etapa intermediária de redirecionamento, sem o usuário final perceber a complexidade por baixo.

Isso é especialmente crítico em apps mobile nativos: o deep link configurado na Anonkey leva o usuário diretamente de volta ao app após autenticação OAuth sem passar por uma página web intermediária.

**Req-2.2.5 — Proteção Anti-Timing Attacks**

O sistema de autenticação deve implementar proteção ativa contra timing attacks como requisito obrigatório de segurança:

- **Magic Link / Email inexistente:** Quando um usuário solicita Magic Link ou OTP para um email que não existe na base, o sistema não retorna erro. Aplica um delay randômico computado e retorna resposta de sucesso sem enviar email. O atacante não consegue enumerar endereços de email válidos medindo tempo de resposta.
- **Validação de OTP:** A comparação entre o código enviado pelo usuário e o código armazenado deve ocorrer via comparação de buffers em tempo constante (`timingSafeEqual` em nível de kernel), garantindo que o tempo de resposta seja idêntico independente de quantos bytes do código estão corretos. Comparações por igualdade de string são proibidas neste contexto.
- **Rate limiting por tentativa de auth:** Bloqueio progressivo por IP e por identificador após N tentativas falhas, configurável por tenant.

---

### 2.3 Modelo de Identidade e Estrutura de Tabelas

**Req-2.3.1 — Identidades Separadas da Conta Mestre**

O Cascata separa o conceito de **conta** do conceito de **identidade**. Uma conta mestre pode ter múltiplas identidades vinculadas — cada identidade representa um método de autenticação distinto ou um provider externo.

Um usuário que se cadastrou via email pode posteriormente vincular seu Google, seu GitHub e um número de telefone à mesma conta. Cada vínculo é uma entrada na tabela `auth.identities`, desacoplada da conta principal. Se o usuário remover o login com Google, a conta continua existindo com os outros métodos — sem perda de dados.

**Req-2.3.2 — Concatenação de Tabelas (Perfis Compostos)**

O modelo de usuário do Cascata suporta perfis compostos. Um mesmo usuário pode ter múltiplos papéis ativos simultaneamente dentro do mesmo projeto:

- Um professor que também é coordenador na mesma instituição
- Um médico que também é paciente na mesma clínica
- Um motorista que também é passageiro na mesma plataforma

O tenant define no painel quais tabelas de perfil podem ser concatenadas a um usuário. O Cascata gerencia esse vínculo de forma nativa — sem colunas nulas na tabela principal, sem JOIN complexo no código do cliente, sem duplicação de registro base.

A tabela principal de usuários permanece limpa. Os perfis vivem em tabelas dedicadas com relacionamento gerenciado pelo Cascata. Uma query de usuário com perfil ativo retorna os dados combinados de forma transparente.

**Req-2.3.3 — RLS Handshake Atômico**

Nenhuma query de tenant atinge o YugabyteDB sem o RLS Handshake. O processo é atômico e obrigatório:

1. `BEGIN` — lock transacional
2. `SET LOCAL ROLE cascata_api_role` — o contexto de execução desce para um role com permissões mínimas
3. Injeção das claims do JWT como variáveis de sessão no YugabyteDB (`set_config('request.jwt.claim.sub', id_usuario)`)
4. Execução da query — as políticas RLS do YugabyteDB bloqueiam fisicamente acesso a dados de outros tenants com base nas variáveis de sessão injetadas
5. `COMMIT`

O vazamento de dado entre tenants é impossível neste modelo — não por código defensivo, mas por impossibilidade física na camada do banco.

**Nota Crítica de Desempenho (Requisição PR-11 - Regra de Ouro do RLS):** 
Para que o RLS não se torne um gargalo letal no planner sob carga do YugabyteDB, *toda política gerada ou aplicada aos tenants deve ser desenhada consumindo exclusivamente propriedades da sessão* via `current_setting(...)`. É expressamente proibida a utilização de correlações complexas, agregações, subqueries ou JOINs dentro das declarações de policy (ex: evitar `USING (tenant_id = (SELECT...))`). A quebra dessa norma anula completamente a eficácia do uso computacional para os modelos NANO/MICRO.

**Req-2.3.4 — Transport Híbrido de Credenciais**

O envio de tokens de autenticação (Magic Links, OTPs, convites) suporta três canais configuráveis pelo tenant:

- **SMTP local** — servidor de email configurado diretamente no painel do tenant
- **Provider externo** — Resend ou qualquer provider SMTP terceiro
- **Webhook customizado** — o Cascata envia o token via HTTP POST para uma URL configurada pelo tenant, assinada com `HMAC-SHA256` no header `X-Cascata-Signature`. O tenant pode usar essa URL para integrar qualquer sistema de envio: n8n, automações internas, SMS gateway, WhatsApp API, etc.

---

### 2.4 Execução Isolada — Sandbox Functions (Removido)

---

### 2.5 Event Sourcing e Analytics

**Req-2.5.1** Toda operação transacional mutável no YugabyteDB concluída com sucesso deve disparar assincronamente um evento de Audit Trail para o barramento **Redpanda**. Esse disparo ocorre em paralelo com o retorno da resposta ao cliente — nunca bloqueia o RTT.

**Req-2.5.2** O pipeline Redpanda → ClickHouse processa logs de forma contínua via batch insert assíncrono. A latência de ingestão não impacta a performance das queries transacionais em nenhuma circunstância.

**Req-2.5.3** Logs analíticos transacionais brutos de instâncias ENTERPRISE e SOVEREIGN não podem ser deletados por nenhum usuário, incluindo administradores do tenant e operadores do Cascata. A única forma de tornar dados irrecuperáveis é via crypto-shredding — revogação da envelope key do tenant no OpenBao, que torna os dados matematicamente ininteligíveis sem operação de DELETE.

**Req-2.5.4** O tenant tem acesso a seus logs e analytics em tempo real pelo painel do Cascata. Queries sobre bilhões de eventos devem retornar em segundos. O painel expõe uma interface de query direta sobre o ClickHouse para tenants ENTERPRISE e SOVEREIGN.

**Req-2.5.5** Logs são particionados por tenant no ClickHouse. Um tenant nunca enxerga dados de outro tenant na camada de analytics, mesmo em tiers que compartilham cluster.

---

### 2.6 Storage — Orquestração de Arquivos

**Req-2.6.1** O Cascata não é um lugar para guardar arquivo. É um orquestrador de storage que decide dinamicamente onde cada tipo de dado vai parar, com validação de segurança executada antes de qualquer escrita.

**Req-2.6.2 — Classificação por Setor**

Todo arquivo enviado ao Cascata é classificado automaticamente em um setor lógico baseado na extensão e nos magic bytes do arquivo:

- `visual` — imagens (jpg, png, webp, avif, heic, svg, etc.)
- `motion` — vídeos (mp4, webm, mov, etc.)
- `audio` — áudios (mp3, flac, ogg, wav, etc.)
- `structured` — dados tabulares e serializados (csv, json, parquet, avro, etc.)
- `docs` — documentos (pdf, docx, xlsx, etc.)
- `exec` — executáveis e scripts (recebem hard-block por padrão)
- `telemetry` — dados de telemetria e séries temporais
- `simulation` — assets 3D e simulação

**Req-2.6.3 — Validação de Magic Bytes**

A extensão do arquivo e o MIME type declarado pelo cliente nunca são confiados como fonte de verdade. O Cascata intercepta a requisição operando estritamente em **modo stream pass-through (zero-copy)** — para nunca acumular buffers perigosos na RAM do Pingora — e lê os primeiros bytes do header binário real, cruzando com o dicionário oficial de assinaturas. Se um arquivo declarado como `imagem.jpg` contiver o header binário de um executável PHP, a stream é abortada imediatamente, o upload é bloqueado e o evento de segurança é registrado no audit trail. Essa validação ocorre *on-the-fly* no Pingora Gateway antes de repassar qualquer byte aos providers de storage final.

**Req-2.6.4 — Gestão de Cota com Reserva Preventiva**

Antes de qualquer upload ser aceito, o Cascata reserva preventivamente os bytes no DragonflyDB. Se o upload ultrapassar a cota configurada para o setor ou para o tenant, é cancelado antes de completar — sem cobrar pelo storage de um arquivo que será descartado.

**Req-2.6.5 — Storage Governance por Tenant**

Cada tenant configura no painel as regras de storage governance: qual setor vai para qual provider. O endpoint público permanece sempre o mesmo (`seudominio.io/storage/...`). O roteamento, as credenciais e a lógica de provider são completamente transparentes para o usuário final do tenant — o Cascata atua como túnel soberano, sem expor chaves ou URLs de providers externos.

Os providers disponíveis são:

- **MinIO** — backbone self-hosted soberano, obrigatório para ENTERPRISE e SOVEREIGN
- **AWS S3** — para tenants com governança apontada para AWS
- **Cloudflare R2** — S3-compatible para edge global com custo reduzido de egress
- **Wasabi** — S3-compatible de alta capacidade e baixo custo
- **Cloudinary** — CDN de mídia com otimização automática de imagem e vídeo
- **ImageKit** — CDN de imagem com transformações em tempo real
- **Google Drive** — integração via OAuth do tenant
- **OneDrive** — integração via Microsoft Graph API
- **Dropbox** — integração via OAuth do tenant

Um tenant pode ter regras diferentes por setor: vídeos vão para Cloudinary, documentos ficam no MinIO local, imagens passam pelo ImageKit, dados estruturados ficam no S3.


### 2.7 Inteligência Artificial — Cidadão de Primeira Classe

O Cascata não integra IA. O Cascata nasce com IA.

A diferença é arquitetural e permanente. Integrar significa encaixar um agente num sistema que não foi pensado para ele — adaptadores, workarounds, permissões genéricas, audit trail ausente, contexto limitado. Nascer com IA significa que o sistema já sabe, desde a primeira linha de código, que agentes autônomos são operadores legítimos — com identidade, com permissões granulares, com rastreabilidade completa, com acesso fluido a cada camada do sistema.

O tenant não "ativa uma feature de IA". Ele configura no painel quais agentes operam no projeto, com qual identidade, com qual escopo de permissão, com qual modelo, com qual provider — e o Cascata garante que esse agente transita pelo sistema com o mesmo nível de segurança, isolamento e auditabilidade que qualquer operador humano.

---

**Req-2.7.1 — Identidade de Agente como Entidade Nativa**

O sistema de identidade do Cascata suporta agentes autônomos como entidade nativa, distinta de usuários humanos. Uma identidade de agente carrega:

- `agent.id` — identificador único do agente no projeto
- `agent.model` — modelo de linguagem em uso (ex: `claude-sonnet-4-5`, `gpt-4o`, modelo local)
- `agent.provider_url` — endpoint do modelo, configurado pelo tenant
- `agent.api_key` — gerenciada pelo OpenBao, nunca exposta em logs ou audit trail. Para maximizar o desempenho (latência p99 <0.5ms nas tool calls MCP), a API Key é cacheada na RAM quente do Pingora com base no TTL (Time-To-Live) emitido pelo OpenBao, sendo renovada assincronamente próximo à expiração.
- `agent.scope` — conjunto explícito de permissões ABAC que o agente possui
- `agent.tenant_owner` — qual usuário humano do tenant é responsável por este agente
- `agent.created_at`, `agent.last_active` — ciclo de vida auditável

Agentes são criados, configurados, suspensos e revogados pelo tenant no painel — na mesma seção de identidades, ao lado de usuários humanos. Um agente revogado perde acesso imediato a todos os recursos do projeto, com o mesmo mecanismo de revogação de sessão humana.

---

**Req-2.7.2 — ABAC para Agentes**

As políticas Cedar ABAC do Cascata expressam permissões para agentes com a mesma gramática que expressam permissões para humanos:

```
"Agente pode executar operação Y SE
  agent.scope CONTAINS 'vendas:read'
  AND agent.model IN modelos_aprovados_pelo_tenant
  AND horario IN janela_de_operacao_configurada
  AND agent.tenant_owner.status = ATIVO"
```

O tenant define no painel quais operações cada agente pode executar, em qual janela de tempo, com qual escopo de dados. Um agente de análise financeira não acessa dados de saúde do mesmo projeto — mesmo que tecnicamente ambos existam no banco. O Cedar garante isso na camada de autorização, antes de qualquer query chegar ao YugabyteDB.

---

**Req-2.7.3 — MCP Server Nativo**

O Cascata expõe um MCP Server (Model Context Protocol) nativo por projeto. MCP é o protocolo aberto padrão que permite que modelos de linguagem usem ferramentas de forma estruturada e segura.

Com o MCP Server do Cascata, qualquer agente configurado pelo tenant — independente do modelo ou provider — acessa nativamente:

- **Banco de dados** — queries SQL no YugabyteDB do projeto, com RLS e ABAC aplicados automaticamente
- **Storage** — leitura e escrita de arquivos via Storage Router, com validação de magic bytes e governance de provider
- **Funções** — invocação de qualquer Sandbox Function do projeto
- **Eventos** — publicação de eventos no Redpanda do projeto
- **Logs e Analytics** — queries sobre o ClickHouse do projeto, com particionamento por tenant garantido
- **Auth** — criação e gerenciamento de identidades, dentro do escopo permitido ao agente

O MCP Server do Cascata não é uma camada externa — é gerado automaticamente a partir da configuração do projeto. Quando o tenant cria uma tabela, essa tabela já é uma ferramenta MCP disponível para os agentes do projeto com as permissões corretas.

O tenant configura no painel quais ferramentas MCP cada agente pode acessar. O Cascata não expõe nada além do que foi explicitamente autorizado.

---

**Req-2.7.4 — Event Subscription para Agentes**

Agentes podem subscrever a tópicos do Redpanda do projeto em tempo real. Quando um evento ocorre no sistema — uma transação concluída, um usuário cadastrado, um arquivo enviado, um threshold atingido, um erro crítico — o agente é notificado imediatamente e pode reagir com autonomia, dentro do escopo de permissão que o tenant configurou.

Isso elimina polling, elimina webhooks frágeis, elimina latência de reação. Um agente de monitoramento financeiro que detecta anomalia em tempo real. Um agente de onboarding que reage ao cadastro de um novo usuário. Um agente de infraestrutura que responde a um evento de threshold de storage. Tudo configurado no painel — o tenant define o tópico, o agente, e o escopo de ação permitido na reação.

---

**Req-2.7.5 — Audit Trail de Agente**

Toda operação executada por um agente é registrada no ClickHouse com contexto completo:

- Qual agente executou
- Qual modelo estava em uso no momento
- Qual operação foi executada
- Sobre quais dados
- Em qual timestamp
- Com qual latência
- Com qual resultado (sucesso, erro, bloqueio por ABAC)

O tenant visualiza o audit trail de agentes no painel separado do audit trail de usuários humanos — mas com a mesma interface e o mesmo nível de detalhe. Em tiers ENTERPRISE e SOVEREIGN, o audit trail de agentes é imutável e não pode ser deletado por nenhuma identidade, humana ou artificial.

---

**Req-2.7.6 — Configuração de Agente no Painel**

O tenant configura agentes no painel do Cascata sem escrever código de integração. A configuração inclui:

- Nome e descrição do agente
- Provider URL do modelo (qualquer endpoint OpenAI-compatible)
- API Key (armazenada no OpenBao — nunca visível após salva)
- Escopo de permissões ABAC
- Ferramentas MCP habilitadas
- Tópicos Redpanda que o agente pode subscrever
- Janela de operação (horários, dias, condições)
- Limite de operações por período (rate limiting para agentes)
- Usuário humano responsável (agent.tenant_owner)

O agente recebe um endpoint MCP único e autenticado que qualquer orquestrador de agentes (LangChain, LangGraph, CrewAI, AutoGen, ou código próprio do tenant) pode usar sem configuração adicional.



### 2.8 Realtime — Subscriptions em Tempo Real

O Cascata não faz polling. Não existe mecanismo de busca periódica de atualizações em nenhuma camada do sistema. Todo dado que muda chega ao cliente conectado em tempo real, sem latência de verificação, sem overhead de requests desnecessárias.

Isso é possível porque o Redpanda já é o barramento central de todos os eventos do sistema. O Realtime Engine não cria uma nova fonte de verdade — ele entrega ao cliente o que o Redpanda já sabe, no momento em que sabe.

---

**Req-2.8.1 — Realtime Engine: Centrifugo**

O motor de realtime do Cascata é **Centrifugo** — escrito em Go, open source (MIT), projetado especificamente para escala massiva de conexões simultâneas. Consome tópicos do Redpanda nativamente e entrega eventos aos clientes conectados via protocolo configurado pelo SDK.

Centrifugo opera como serviço dedicado dentro do Data Plane. Não compartilha processo com o Pingora Gateway nem com o Control Plane — fronteira de responsabilidade preservada.

**Req-2.8.2 — Protocolos de Transporte**

O Cascata suporta três protocolos de transporte para realtime, em ordem de preferência por performance:

- **WebSocket** — protocolo primário. Conexão bidirecional persistente, menor overhead por mensagem, suportado universalmente por browsers e SDKs mobile
- **SSE (Server-Sent Events)** — fallback automático quando WebSocket não está disponível. Unidirecional servidor→cliente, mas suficiente para a maioria dos casos de uso de realtime
- **HTTP-Streaming** — fallback de último recurso para ambientes com restrições de rede corporativa

O SDK do Cascata negocia automaticamente o melhor protocolo disponível. O tenant e o usuário final nunca interagem com essa decisão.

**WebTransport (QUIC)** está em roadmap como protocolo primário futuro — latência inferior ao WebSocket em redes móveis instáveis, multiplexação nativa de streams. A arquitetura do Centrifugo já suporta WebTransport. Será habilitado como opção adicional quando o suporte browser atingir maturidade de produção.

**Req-2.8.3 — Modelo de Canais e Subscriptions**

O sistema de realtime opera com um modelo de canais. Todo canal é identificado por um namespace e um identificador de recurso:

```
{namespace}:{identificador}

Exemplos:
  public:notifications          → canal público, qualquer cliente conectado
  private:{user_id}:inbox       → canal privado por usuário
  tenant:{tenant_id}:orders     → canal de recurso do tenant
  table:orders:changes          → canal de mudanças em uma tabela específica
  agent:{agent_id}:events       → canal de eventos de um agente
```

O tenant configura no painel quais tabelas e eventos disparam atualizações em quais canais. A configuração é declarativa — sem código de infraestrutura, sem lógica de publish manual no backend do tenant.

**Req-2.8.4 — Autorização de Canal via Cedar ABAC**

Todo subscribe a um canal passa pela validação Cedar ABAC antes de ser aceito. Um cliente não pode ouvir um canal para o qual não tem permissão — mesmo que conheça o nome do canal. A validação ocorre no momento da conexão e é reavaliada periodicamente durante a sessão.

```
"Cliente pode subscrever canal private:{user_id}:inbox SE
  jwt.sub = user_id
  AND tenant.realtime_enabled = true
  AND user.status = ATIVO"
```

**Req-2.8.5 — Presença e Histórico com Recovery**

Para canais configurados pelo tenant como presença ativa:

- **Presence:** lista de clientes conectados ao canal em tempo real. Útil para features de "usuários online", colaboração em tempo real, notificações de digitação
- **Histórico com recovery:** mensagens recentes do canal são armazenadas temporariamente no DragonflyDB. Quando um cliente reconecta após queda de rede, recebe automaticamente os eventos que perdeu durante a desconexão — sem precisar fazer query ao banco para reconciliar estado

O período de retenção do histórico por canal é configurável pelo tenant no painel.

**Req-2.8.6 — Integração com Redpanda**

O Centrifugo consome tópicos do Redpanda diretamente. Todo evento publicado no Redpanda por qualquer operação do Cascata (transação SQL, upload de arquivo, execução de função, operação de agente, evento de auth) pode ser roteado para canais realtime configurados pelo tenant.

O pipeline é:

```
Operação no Cascata
  → Redpanda (evento publicado)
    → Centrifugo (consome do tópico)
      → Canal do tenant (filtrado por ABAC)
        → Clientes conectados (WebSocket/SSE)
```

A latência fim-a-fim desde a operação até o cliente receber o evento: **<50ms p99** em condições normais de rede.

**Req-2.8.7 — Escalabilidade de Conexões**

Em modo Shelter (VPS única), o Centrifugo suporta até **50.000 conexões simultâneas** com footprint de ~100MB RAM. Para tiers ENTERPRISE e SOVEREIGN, múltiplas instâncias do Centrifugo são provisionadas pelo Kubernetes Operator do Cascata com load balancing automático — escala horizontal linear.



### 2.9 SDK — Developer Experience

O desenvolvedor que usa o Cascata nunca deve sentir que está operando infraestrutura. Ele deve sentir que está descrevendo o comportamento do produto que quer construir — e o Cascata executa.

O SDK é a interface direta dessa promessa. É o ponto de contato entre o projeto do tenant e toda a potência da arquitetura Cascata. Por isso, o SDK não é uma camada fina de HTTP — é o produto do ponto de vista do desenvolvedor.

---

**Req-2.9.1 — Estratégia Dual de SDK**

O Cascata mantém dois SDKs com responsabilidades e públicos distintos:

**`@cascata/client` — SDK Nativo**
O SDK completo do Cascata. Expõe 100% das funcionalidades da plataforma: banco de dados, auth com todos os métodos, storage com governance, funções, realtime, agentes, MCP, perfis compostos, configuração de ABAC, analytics. TypeScript-first com tipos gerados automaticamente a partir do schema do projeto. É o SDK que novos projetos devem usar.

**`@cascata/compat` — Camada de Compatibilidade Supabase**
Implementa a interface pública do Supabase JS client exatamente — mesmos métodos, mesmas assinaturas, mesmo comportamento observável. Por baixo, roteia todas as chamadas para o Cascata. Projetos existentes que usam Supabase migram sem alterar uma linha de código. Ferramentas no-code que suportam Supabase funcionam com o Cascata automaticamente.

O `@cascata/compat` não tem acesso a funcionalidades exclusivas do Cascata — ele é uma ponte de compatibilidade, não o SDK principal. Quando o tenant precisar de recursos além do que o Supabase oferece (agentes, MCP, storage governance, perfis compostos), a migração para `@cascata/client` é o caminho natural — e o código base existente permanece funcionando durante a transição.

**Req-2.9.2 — Linguagens Suportadas**

O Cascata suporta SDKs oficiais nas seguintes linguagens, em ordem de prioridade de lançamento:

| SDK | Linguagem | Plataforma alvo | Prioridade |
|-----|-----------|-----------------|-----------|
| `@cascata/client` | TypeScript | Web, Node.js, Bun, edge runtimes | Fase 1 |
| `cascata-swift` | Swift | iOS, macOS, visionOS | Fase 1 |
| `cascata-kotlin` | Kotlin | Android, JVM | Fase 1 |
| `cascata-python` | Python | Backend, dados, IA/ML, scripts | Fase 2 |
| `cascata-dart` | Dart | Flutter (iOS + Android unificado) | Fase 2 |
| `cascata-rust` | Rust | Backend, edge, sistemas | Fase 2 |
| `cascata-go` | Go | Backend, microserviços | Fase 3 |

SDKs de Fase 1 são lançados junto com o produto. SDKs de Fase 2 e 3 seguem o roadmap de adoção de plataforma.

**Req-2.9.3 — Geração Automática de Tipos**

O `@cascata/client` gera automaticamente tipos TypeScript a partir do schema do projeto no YugabyteDB. O tenant executa um comando na CLI e recebe um arquivo de tipos que reflete exatamente a estrutura atual do banco — tabelas, colunas, relacionamentos, perfis compostos, políticas ABAC.

```typescript
// Tipos gerados automaticamente a partir do schema
import type { Database } from './cascata-types'

const cascata = createClient<Database>(url, anonkey)

// Autocompletion e type-safety em toda query
const { data } = await cascata
  .from('pedidos')          // ✓ tabela validada em compile time
  .select('id, valor, usuario(nome, email)')  // ✓ colunas validadas
  .eq('status', 'aprovado') // ✓ tipo do campo verificado
```

Mudança no schema do banco → regenerar tipos → erros de tipo aparecem no IDE antes de chegar em produção.

**Req-2.9.4 — API do SDK Nativo**

O `@cascata/client` expõe namespaces organizados por domínio:

```typescript
const cascata = createClient(url, anonkey)

// Banco de dados
cascata.from('tabela').select().eq().insert().update().delete()
cascata.rpc('nome_da_funcao', params)

// Auth
cascata.auth.signIn({ provider: 'google' })
cascata.auth.signIn({ method: 'magic_link', email })
cascata.auth.signIn({ method: 'passkey' })
cascata.auth.signOut()
cascata.auth.getSession()
cascata.auth.onAuthStateChange(callback)

// Storage
cascata.storage.from('bucket').upload(path, file)
cascata.storage.from('bucket').download(path)
cascata.storage.from('bucket').getPublicUrl(path)

// Realtime
cascata.realtime
  .channel('private:pedidos')
  .on('INSERT', callback)
  .on('UPDATE', callback)
  .subscribe()

// Agentes (exclusivo @cascata/client)
cascata.agents.get('nome_do_agente').invoke(prompt, context)

// Analytics (exclusivo @cascata/client)
cascata.analytics.query(sql)
```

**Req-2.9.5 — CLI do Cascata**

A CLI é parte do SDK e cobre o ciclo completo de desenvolvimento:

```bash
# Setup e desenvolvimento local
cascata init           # cria projeto, configura ambiente local
cascata dev            # levanta o Cascata completo localmente
cascata login          # autentica na instância do Cascata

# Banco de dados
cascata db diff        # gera migration a partir de mudanças no schema
cascata db push        # aplica migrations pendentes
cascata db pull        # sincroniza schema local com o banco remoto
cascata db reset       # reseta banco local para estado limpo
cascata types generate # gera arquivo de tipos TypeScript

# Agentes
cascata agents deploy  # publica configuração de agente

# Geral
cascata status         # saúde do projeto
cascata logs           # stream de logs em tempo real
```

**Req-2.9.6 — Desenvolvimento Local**
    
Todo desenvolvedor que usa o Cascata deve conseguir rodar o ambiente completo localmente — sem conta, sem cloud, sem internet. O comando `cascata dev` levanta via Docker Compose todos os serviços necessários na máquina do desenvolvedor com hot-reload, logs agregados e painel acessível em `localhost:3000`.

O ambiente local é funcionalmente idêntico ao ambiente de produção — sem mocks, sem stubs. O que funciona localmente funciona em produção. Essa garantia é inviolável.

**Req-2.9.7 — Compatibilidade Supabase Verificada**

O `@cascata/compat` deve passar na suite de testes de integração do Supabase JS client como critério de release. Toda versão publicada do `@cascata/compat` foi verificada contra os casos de uso documentados do Supabase: auth com todos os providers OAuth, queries com filtros e joins, realtime subscriptions, storage upload/download, RPC.



### 2.10 Disaster Recovery e Failover

O Cascata não trata DR como funcionalidade adicional. DR é uma propriedade do sistema — cada componente com estado tem garantias formais de recuperação, e essas garantias são automaticamente provisionadas pelo tier do tenant. O operador não configura DR: ele escolhe o tier, e o DR vem incluso nas garantias daquele tier.

A distinção central:

- **Backup** é o que você usa quando tudo falhou — restauração lenta, perda de dados, intervenção manual
- **DR do Cascata** é o sistema nunca parar, ou recuperar antes que o usuário final perceba

---

**Req-2.10.1 — Garantias por Tier**

Cada tier tem garantias formais de RPO (Recovery Point Objective — quanto dado pode ser perdido) e RTO (Recovery Time Objective — quanto tempo leva para recuperar):

| Tier | RPO | RTO | Modelo de Resiliência |
|------|-----|-----|-----------------------|
| NANO | até 24h | ~30 min | Snapshot diário automatizado. Single node. Adequado para dev e MVPs |
| MICRO | até 6h | ~15 min | Snapshots a cada 6h. Single node com backup contínuo para MinIO |
| STANDARD | < 1 min | < 5 min | Cluster de 3 nós com Raft. Failover automático sem intervenção humana |
| ENTERPRISE | 0 | < 30 s | Multi-AZ síncrono. Perda de zona inteira não afeta disponibilidade |
| SOVEREIGN | 0 | < 15 s | Multi-region síncrono para dados transacionais. DR site também air-gap |

**Req-2.10.2 — Cascata DR Orchestrator**

Um serviço dedicado em Go — o **Cascata DR Orchestrator** — monitora a saúde de todos os componentes com estado do sistema e executa failover automaticamente sem intervenção humana:

- Monitora: YugabyteDB, ClickHouse, Redpanda, DragonflyDB, MinIO, OpenBao e Qdrant
- Detecta falha via health checks ativos com timeout agressivo por componente
- Executa runbooks de failover pré-definidos por tier quando falha é detectada
- Atualiza o Tenant Router automaticamente para redirecionar tráfego para nós saudáveis
- Notifica o operador via dashboard e canais configurados (webhook, email) com contexto completo da falha
- Registra todo evento de DR no ClickHouse com audit trail completo: o quê falhou, quando, qual runbook executou, qual foi o resultado, quanto tempo levou

O DR Orchestrator não toma decisões de negócio. Executa runbooks — sequências determinísticas de ações pré-validadas. Nenhuma lógica de IA, nenhuma heurística. Previsibilidade total em um momento de crise.

**Req-2.10.3 — Resiliência por Componente**

*YugabyteDB*
Raft consensus nativo — sobrevive à perda de minoria de nós automaticamente sem intervenção. Para STANDARD: cluster de 3 nós, tolerância à perda de 1 nó. Para ENTERPRISE: deploy multi-AZ com replicação síncrona — perda de uma zona de disponibilidade inteira não interrompe operação. Para SOVEREIGN: replicação síncrona multi-region com PITR ativo — RPO zero para dados transacionais.

*Redpanda*
Raft consensus nativo com fator de replicação 3 em STANDARD+. Sobrevive à perda de 1 broker sem perda de mensagem. Para SOVEREIGN: geo-replication entre sites. O Redpanda nunca perde um evento publicado com confirmação de commit.

*ClickHouse*
ReplicatedMergeTree com 2+ réplicas em STANDARD+. Logs e analytics têm tolerância a perda de réplica — consistência eventual é aceitável para dados analíticos históricos. Para ENTERPRISE e SOVEREIGN: replicação síncrona de partições críticas.

*DragonflyDB*
Cache é por natureza reconstituível — não é fonte de verdade. Em STANDARD+, replicação ativa mantém continuidade de sessão durante failover. Em caso de perda total do cache, o sistema reconstrói automaticamente a partir do YugabyteDB. O impacto é degradação de performance temporária, não perda de dados.

*MinIO*
Erasure coding nativo protege contra perda de N/2 nós dentro de um site. Para ENTERPRISE: site replication ativa-ativa entre zonas. Para SOVEREIGN: site replication entre data centers do cliente — nenhum arquivo fica em apenas um local físico.

*OpenBao*
Backend de armazenamento Raft — HA nativo. Cluster de 3 nós em STANDARD+. Para SOVEREIGN: instância dedicada no ambiente do cliente com seu próprio cluster Raft — a perda do Cascata não afeta as chaves do cliente.

*Qdrant*
Coleções com fator de replicação ≥ 2 em STANDARD+. Snapshots periódicos armazenados no MinIO do projeto. Para ENTERPRISE e SOVEREIGN: nós dedicados com replicação síncrona.

**Req-2.10.4 — Snapshots e PITR**

Para tiers STANDARD e acima, Point-in-Time Recovery (PITR) está ativo por padrão:

- YugabyteDB: WAL (Write-Ahead Log) contínuo — restauração para qualquer segundo nos últimos N dias, configurável por tier
- ClickHouse: snapshots incrementais diários para MinIO + WAL para recovery granular
- Qdrant: snapshots diários para MinIO com retenção configurável
- MinIO: versionamento de objetos ativo — toda versão de arquivo é recuperável

O tenant acessa PITR pelo painel — seleciona o ponto no tempo desejado, visualiza o que será restaurado, confirma. O DR Orchestrator executa a restauração sem intervenção manual adicional.

**Req-2.10.5 — Testes de DR Automatizados**

Em tiers ENTERPRISE e SOVEREIGN, o DR Orchestrator executa **chaos drills** automatizados em janelas de baixo tráfego — desliga componentes propositalmente e verifica se o failover ocorre dentro das garantias do tier. O resultado de cada drill é registrado no audit trail e disponível no painel do operador.

Um sistema de DR que nunca foi testado não tem garantias reais. O Cascata testa DR continuamente, não apenas quando o desastre acontece.




### 2.11 Banco Vetorial — Qdrant

O banco vetorial do Cascata não é uma feature de IA. É uma camada de persistência de primeira classe — com as mesmas garantias de isolamento por tenant, autorização via Cedar ABAC, audit trail no ClickHouse e DR integrado ao Orchestrator que qualquer outro componente do sistema.

Isso significa que qualquer operação vetorial no Cascata é rastreável, auditável, isolada por tenant e protegida pelas mesmas políticas de acesso que protegem dados relacionais. Um agente que faz busca vetorial não tem acesso privilegiado — tem exatamente o escopo que o tenant configurou.

---

**Req-2.11.1 — Motor Vetorial: Qdrant**

O banco vetorial do Cascata é **Qdrant** — escrito em Rust (alinhado com a filosofia de zero GC no caminho crítico), licença Apache 2.0, self-hosted com modo distribuído nativo.

Razões técnicas objetivas:

- **HNSW com quantização escalar e binária** — melhor relação performance/memória disponível. Quantização reduz footprint de vetores em até 32x com perda mínima de precisão — crítico para o modo Shelter ($20/mês)
- **Payload filtering nativo** — filtragem por metadados ocorre dentro do índice vetorial, não após a busca. Isso é fundamental para multi-tenancy: a query retorna apenas vetores do tenant correto sem overhead de pós-filtragem
- **Sparse vectors + dense vectors** — suporta busca híbrida (semântica + keyword) nativamente, sem combinar dois sistemas diferentes
- **Snapshots para MinIO** — integração direta com o objeto de storage do Cascata para DR
- **Modo distribuído** — sharding e replicação nativos para ENTERPRISE e SOVEREIGN

**Req-2.11.2 — Multi-tenancy Vetorial**

O isolamento de dados vetoriais segue o mesmo modelo de tiers do sistema:

- **NANO / MICRO**: coleções compartilhadas com isolamento por payload filter (`tenant_id`). RLS vetorial aplicado via Cedar ABAC antes de toda query — um tenant nunca enxerga vetores de outro
- **STANDARD**: coleções dedicadas por tenant dentro do cluster Qdrant compartilhado
- **ENTERPRISE / SOVEREIGN**: instância Qdrant dedicada por tenant, com nós próprios e replicação independente

**Req-2.11.3 — Autorização Vetorial via Cedar ABAC**

Toda operação no Qdrant — upsert, busca, deleção — passa pela validação Cedar ABAC antes de chegar ao banco. As políticas expressam permissões vetoriais com a mesma gramática das demais políticas do sistema:

```
"Agente pode executar vector.search() SE
  agent.scope CONTAINS 'knowledge_base:read'
  AND collection.owner = agent.tenant_id
  AND horario IN janela_de_operacao"
```

Não existe acesso direto ao Qdrant que bypasse o Cedar. O Pingora intercepta toda operação vetorial, valida as políticas, e só então encaminha ao Qdrant.

**Req-2.11.4 — API Vetorial no SDK**

O `@cascata/client` expõe operações vetoriais como namespace dedicado:

```typescript
// Upsert de vetores com payload de metadados
await cascata.vectors
  .collection('knowledge_base')
  .upsert([{
    id: 'doc-123',
    vector: embeddingArray,          // vetor denso
    payload: {
      tenant_id: 'abc',
      source: 'manual_tecnico.pdf',
      page: 42,
      created_at: '2026-01-01'
    }
  }])

// Busca semântica com filtro de payload
const results = await cascata.vectors
  .collection('knowledge_base')
  .search({
    vector: queryEmbedding,
    filter: { source: 'manual_tecnico.pdf' },
    limit: 10,
    with_payload: true
  })

// Busca híbrida (semântica + keyword)
const hybrid = await cascata.vectors
  .collection('knowledge_base')
  .searchHybrid({
    dense: queryEmbedding,
    sparse: sparseVector,
    limit: 10
  })
```

**Req-2.11.5 — Qdrant como Ferramenta MCP**

O MCP Server do Cascata expõe operações vetoriais como ferramentas nativas para agentes:

```
vector.search(collection, query_vector, filter, limit)
vector.upsert(collection, points)
vector.delete(collection, ids)
vector.collections.list()
```

Um agente com escopo `knowledge_base:read` pode fazer busca semântica diretamente via tool call MCP — sem precisar de código customizado de integração. O Cedar ABAC valida o escopo, o Pingora encaminha ao Qdrant, o resultado retorna como JSON estruturado para o orquestrador do agente.

**Req-2.11.6 — Geração de Embeddings**

O Cascata não hospeda modelos de embedding. A geração de vetores é responsabilidade do tenant — usando qualquer provider ou modelo local que escolher. O Cascata recebe o vetor pronto e gerencia persistência, busca, isolamento e DR.

Isso é intencional: modelos de embedding evoluem rapidamente. O tenant escolhe o modelo mais adequado para seu caso de uso (text-embedding-3-large, nomic-embed, modelos locais via Ollama) e o Cascata garante a infraestrutura vetorial independente da escolha.

**Req-2.11.7 — Audit Trail Vetorial**

Toda operação no Qdrant é registrada no ClickHouse com contexto completo:
- Qual identidade executou (humano ou agente)
- Qual collection foi acessada
- Qual operação (search, upsert, delete)
- Quantos vetores foram afetados
- Qual latência
- Qual resultado (sucesso, bloqueio por ABAC, erro)

Em tiers ENTERPRISE e SOVEREIGN, o audit trail vetorial é imutável — mesma garantia do audit trail transacional.


### 2.12 Webhooks e Integrações Externas

O sistema de webhooks do Cascata não é um disparador de eventos. É um **motor de automação com inteligência na origem** — o evento é avaliado, filtrado, assinado e entregue com garantias de resiliência antes de chegar ao destino externo. A complexidade que outros sistemas terceirizam para Lambda, filas externas ou scripts customizados mora diretamente no núcleo do Cascata.

---

**Req-2.12.1 — Arquitetura de Fila via Redpanda**

O Cascata não introduz um sistema de filas separado para webhooks. O **Redpanda já é o barramento central** do sistema — o disparo de webhooks é um consumer dedicado do Redpanda, sem nova dependência de infraestrutura.

O pipeline é:

```
Operação no Cascata (INSERT / UPDATE / DELETE / READ / AUTH / STORAGE / FUNCTION)
  → Evento publicado no Redpanda (já ocorre para audit trail)
    → Webhook Consumer avalia filtros configurados pelo tenant
      → SE filtros satisfeitos: encaminha para fila de entrega
        → Worker de entrega HTTP executa o disparo externo
```

O disparo nunca bloqueia a operação original. O RTT do cliente não é afetado em nenhuma circunstância.

**Req-2.12.2 — Cobertura de Eventos**

Todo tipo de evento do Cascata pode disparar um webhook. O tenant seleciona no painel:

- **Banco de dados:** `INSERT`, `UPDATE`, `DELETE`, `READ` — por tabela específica ou global
- **Auth:** login, logout, cadastro, falha de autenticação, revogação de sessão
- **Storage:** upload concluído, download, deleção, violação de magic bytes bloqueada
- **Functions:** execução concluída, timeout, erro
- **Agentes:** operação executada, bloqueio por ABAC, erro
- **Sistema:** promoção de tier, threshold de cota atingido, evento de DR

O evento `READ` — ausente em muitas plataformas — está presente no Cascata. Um tenant que precisa auditar acesso a dados sensíveis pode disparar webhook a cada leitura em tabelas específicas.

**Req-2.12.3 — Filter Engine (Rust, avaliação na origem)**

O Cascata injeta inteligência antes do disparo — não depois. O filter engine é executado em Rust dentro do Pingora antes de qualquer chamada HTTP externa, sem overhead de runtime de alto nível.

O tenant configura filtros no painel com operadores visuais:

| Operador | Descrição | Exemplo |
|----------|-----------|---------|
| `eq` | igual | `status = "aprovado"` |
| `neq` | diferente | `status != "rascunho"` |
| `gt` | maior que | `valor > 1000` |
| `lt` | menor que | `estoque < 5` |
| `gte` | maior ou igual | `score >= 90` |
| `lte` | menor ou igual | `prioridade <= 2` |
| `contains` | contém substring | `email contains "@empresa.com"` |
| `starts_with` | começa com | `cpf starts_with "123"` |
| `in` | dentro de lista | `categoria in ["urgente", "critico"]` |
| `is_null` | campo ausente | `aprovado_por is_null` |

Filtros são combinados com `AND` / `OR` e agrupados visualmente no painel. Um webhook que dispara apenas quando `status = "aguardando_pagamento" AND valor > 500` não consome recursos de fila para os demais casos — a decisão é tomada no filter engine, não no destino.

**Req-2.12.4 — Políticas de Retentativa**

Quando o destino externo retorna erro (timeout, 5xx), o Cascata aplica a política de retentativa configurada pelo tenant:

- **Exponencial (padrão):** 10 tentativas com backoff exponencial. Delay dobrado progressivamente. Ideal para destinos instáveis ou com janelas de manutenção
- **Linear:** 5 tentativas com delay fixo configurável. Para anomalias de rede de curta duração
- **Strict (idempotente):** 1 tentativa, sem retry. Obrigatório para webhooks de pagamento, faturamento e qualquer operação onde duplicação causa dano. O tenant assume a responsabilidade de tratar a falha

A política é selecionada por webhook individual — diferentes integrações têm comportamentos diferentes no mesmo projeto.

**Req-2.12.5 — Dead Letter e Fallback URL**

Quando todas as tentativas de entrega esgotam sem sucesso, o evento entra no estado Dead Letter. O Cascata executa duas ações:

1. **Registra no audit trail** do ClickHouse com contexto completo: evento original, payload, todas as tentativas, timestamps, respostas recebidas
2. **Dispara a Fallback URL** — uma URL de alerta configurada pelo tenant que recebe uma notificação estruturada informando que o webhook principal falhou definitivamente

A Fallback URL é projetada para canais de alerta operacional — Slack, Teams, Telegram, PagerDuty, n8n, ou qualquer endpoint que o tenant opere. O dado nunca "morre" silenciosamente: o responsável é notificado para intervenção manual.

**Req-2.12.6 — Segurança: Assinatura HMAC-SHA256**

Todo webhook dispara com o header `X-Cascata-Signature` contendo a assinatura HMAC-SHA256 do payload calculada com o secret do projeto. O destino externo verifica a assinatura antes de processar — garantindo que a requisição veio genuinamente do Cascata e não de um atacante que conhece a URL.

O secret é gerenciado pelo OpenBao — nunca exposto em logs, nunca visível no audit trail. O painel exibe o secret com ofuscação visual e permite rotação a qualquer momento sem downtime nos webhooks existentes (período de transição com aceitação de ambas as assinaturas).

**Req-2.12.7 — Proteção contra SSRF**

Antes de qualquer chamada HTTP externa, o Cascata valida a URL de destino contra uma lista de bloqueio de endereços internos:

- `localhost`, `127.0.0.1`, `::1`
- Ranges privados RFC 1918: `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`
- Link-local: `169.254.0.0/16`
- Nomes de serviços internos do cluster (`db`, `redis`, `minio`, etc.)
- Metadados de cloud providers (`169.254.169.254` — AWS/GCP/Azure metadata endpoint)

Um atacante que configure um webhook apontando para um endpoint interno do Cascata tem a requisição bloqueada antes do primeiro byte enviado. A tentativa é registrada no audit trail como evento de segurança.

**Req-2.12.8 — Ping Test**

O tenant pode executar um disparo de teste a qualquer momento pelo painel sem precisar gerar um evento real no banco. O Cascata envia um payload de teste `{"event": "ping", "source": "cascata_test"}` para a URL configurada e exibe o resultado em tempo real no painel — status HTTP recebido, tempo de resposta, headers devolvidos. Útil para verificar conectividade, configuração de firewall e autenticidade da assinatura no destino antes de ativar o webhook em produção.

**Req-2.12.9 — Webhooks como Ferramenta MCP**

Agentes podem criar, modificar e consultar webhooks dentro do escopo de permissão configurado pelo tenant. Um agente de automação pode registrar um novo endpoint externo como reação a um evento de negócio — sem intervenção humana, dentro das políticas ABAC.



### 2.13 Usuários Internos e Gestão de Equipe do Operador

O Cascata distingue três categorias de identidade com escopos e fronteiras distintas:

- **Usuários finais do tenant** — pessoas que usam o produto construído no Cascata
- **Agentes** — identidades de IA que operam recursos do projeto
- **Membros da equipe do operador** — pessoas que acessam o painel do Cascata para gerir projetos

Esta seção trata da terceira categoria: a equipe interna do operador.

---

**Req-2.13.1 — Modelo de Permissão de Equipe**

O operador do Cascata pode convidar membros de equipe para o painel com escopos granulares de acesso. O modelo usa o mesmo Cedar ABAC do sistema — a diferença é que as políticas operam no nível do painel do operador, não no nível do projeto do tenant.

Um membro de equipe tem:

- `member.role` — papel na organização (owner, admin, developer, analyst, viewer, custom)
- `member.scope` — conjunto explícito de permissões ABAC no painel
- `member.projects` — lista de projetos/tenants que este membro pode acessar
- `member.created_by` — quem convidou este membro
- `member.last_active` — ciclo de vida auditável

**Req-2.13.2 — Papéis Pré-definidos**

O Cascata fornece papéis pré-definidos como ponto de partida. O operador pode usá-los diretamente ou criar papéis customizados a partir deles:

| Papel | Acesso padrão |
|-------|--------------|
| `owner` | Acesso total — único que pode deletar a instância, transferir ownership, ver billing de infra |
| `admin` | Acesso total exceto operações destrutivas irreversíveis. Pode convidar e remover membros |
| `developer` | Pode criar e modificar projetos, schemas, funções, webhooks. Não pode modificar configurações de infra ou compliance |
| `analyst` | Acesso read-only a logs, analytics e audit trail. Não pode modificar nada |
| `viewer` | Acesso read-only ao dashboard de um ou mais projetos específicos. Sem acesso a logs sensíveis |
| `custom` | Escopo definido explicitamente pelo operador via Cedar ABAC |

**Req-2.13.3 — Isolamento por Projeto**

Um membro de equipe pode ter acesso a um subconjunto dos projetos do operador. O estagiário de frontend vê apenas o projeto de desenvolvimento. O DBA vê apenas os projetos onde precisa atuar. O analista de dados vê os logs de analytics de todos os projetos mas não pode modificar nada.

O acesso a um projeto não implica acesso aos dados dos usuários finais daquele projeto — a menos que o membro tenha permissão explícita de acesso a dados, que é separada da permissão de acesso ao painel.

**Req-2.13.4 — Audit Trail de Equipe**

Toda ação de um membro de equipe no painel é registrada no ClickHouse com contexto completo: quem fez, o que fez, em qual projeto, quando, de qual IP, com qual resultado. O owner pode auditar a atividade de qualquer membro a qualquer momento.

Ações destrutivas — deleção de tabela, rollback de schema, revogação de chave — requerem confirmação explícita e são registradas com timestamp e identidade do executor de forma imutável.

**Req-2.13.5 — Autenticação de Membros de Equipe**

Membros de equipe autenticam no painel via:

- **Passkey/FIDO2** — método primário recomendado
- **Email + MFA** — TOTP obrigatório para papéis `admin` e `owner`
- **SSO via OAuth** — o operador pode configurar Google Workspace, GitHub, ou qualquer provider OIDC como método de login para a equipe

Sessões de membros de equipe têm timeout configurável pelo owner. Sessões inativas por mais de N horas são encerradas automaticamente.



### 2.14 Central de Comunicação Externa

O Cascata unifica todos os canais de comunicação de saída em uma única interface: a **Central de Comunicação**. No projeto do tenant, toda notificação ao mundo externo — seja um webhook para um sistema de pagamentos, um push para o app mobile do usuário, um SMS de verificação, ou um email transacional — é configurada, monitorada e gerenciada no mesmo lugar.

Essa unificação não é apenas organizacional. É arquitetural: todos os canais compartilham o mesmo pipeline de entrega assíncrona via Redpanda, o mesmo modelo de retry, o mesmo audit trail no ClickHouse, e o mesmo sistema de templates. O tenant aprende uma interface e opera todos os canais com o mesmo vocabulário.

---

**Req-2.14.1 — Canais Disponíveis**

A Central de Comunicação agrupa os canais em seções dentro de uma única página. Cada seção é colapsável e expansível — nenhuma seção cresce além do que cabe na tela. Quando uma configuração é grande demais (múltiplos providers, templates complexos, regras de filtro), ela abre em modal dedicado.

**Seção: Webhooks**
Descrita em detalhes na seção 2.12. Disparo baseado em eventos do sistema com filter engine, retry policies, dead letter e fallback URL.

**Seção: Push Notifications**
Notificações nativas para dispositivos móveis e web. O Cascata suporta os três protocolos nativos:
- **APNs (Apple Push Notification Service)** — iOS, macOS, visionOS
- **FCM (Firebase Cloud Messaging)** — Android, Chrome
- **Web Push (VAPID)** — browsers modernos sem app nativo

O tenant configura as credenciais de cada provider no painel (certificados APNs, chave FCM, par VAPID) uma única vez. A partir daí, o SDK do Cascata expõe `cascata.push.send()` com o Cascata roteando automaticamente para o provider correto baseado no dispositivo do destinatário.

Templates de notificação são criados no painel com preview em tempo real — título, corpo, ícone, ação ao toque, badge, som. O mesmo template pode ser enviado para múltiplos providers simultaneamente sem duplicação de configuração.

**Seção: Email**
Envio de emails transacionais e de automação. O tenant configura o provider de envio (SMTP local, Resend, ou qualquer provider SMTP) e gerencia templates diretamente no painel com editor WYSIWYG ou modo código. Todo email enviado pelo Cascata — seja de autenticação (Magic Link, OTP) ou de automação — usa a mesma infraestrutura de templates e o mesmo pipeline de entrega.

**Seção: SMS**
Envio de mensagens SMS via providers configuráveis pelo tenant (Twilio, Vonage, ou qualquer API SMS via webhook adapter). Utilizado primariamente para OTP por telefone e notificações críticas. O mesmo sistema de templates e retry se aplica.

**Req-2.14.2 — Pipeline de Entrega Unificado**

Todos os canais da Central de Comunicação compartilham o mesmo pipeline:

```
Trigger (evento do sistema, automação, SDK call)
  → Filter Engine (Rust/Pingora) — avalia condições configuradas
    → Redpanda (fila de entrega assíncrona)
      → Worker dedicado por canal
        → Provider externo (APNs / FCM / WebPush / SMTP / SMS / HTTP)
          → ClickHouse (audit trail: entregue / falhou / retry)
```

Um evento que dispara múltiplos canais simultaneamente — por exemplo, um push E um email E um webhook — publica três mensagens no Redpanda e cada worker processa independentemente. A falha em um canal não afeta os outros.

**Req-2.14.3 — Automação de Campanhas**

O tenant pode configurar automações baseadas em eventos do sistema que disparam comunicação para segmentos de usuários:

- "Quando um usuário não fez login há 7 dias, enviar push de reengajamento"
- "Quando estoque cair abaixo de 5 unidades, enviar email para o gerente"
- "Quando um pedido atingir status 'aprovado', enviar push + email para o comprador"

As regras de automação são configuradas no painel com a mesma interface de filtros dos webhooks. O Cascata avalia as condições e despacha os canais configurados sem código adicional no backend do tenant.

**Req-2.14.4 — Observabilidade de Comunicação**

A Central de Comunicação exibe em tempo real:
- Taxa de entrega por canal (últimas 24h, 7d, 30d)
- Falhas e motivo (timeout, credencial inválida, dispositivo inativo)
- Dead letters pendentes de revisão
- Volume de envio por canal com breakdown por template



### 2.15 Key Groups e Rate Limiting Granular

O Cascata oferece controle granular de tráfego que vai além de rate limiting simples. O tenant define políticas que se aplicam a três dimensões distintas: grupos de chaves API, chaves individuais, e usuários autenticados via login. Cada dimensão tem seu próprio conjunto de limites e comportamentos de resposta quando os limites são atingidos.

---

**Req-2.15.1 — Key Groups**

Key Groups são agrupamentos lógicos de chaves API com políticas compartilhadas. O tenant cria grupos que representam diferentes perfis de consumo do projeto:

- `free` — desenvolvedores usando o produto gratuitamente, com limites conservadores
- `pro` — clientes pagantes com limites ampliados
- `enterprise` — clientes com SLA, sem limite prático
- `internal` — chamadas internas do próprio sistema do tenant, sem throttle

Cada grupo define:
- Limite de requests por segundo / minuto / hora / dia
- Limite por operação (INSERT, SELECT, UPDATE, DELETE separadamente)
- Comportamento ao atingir limite: **block** ou **nerf**
- Chaves de acesso alocadas a este grupo

**Req-2.15.2 — Políticas por Chave Individual**

Além dos grupos, o tenant pode definir políticas específicas para chaves individuais que sobrescrevem as configurações do grupo. Uma chave de parceiro estratégico pode ter limites diferentes das demais chaves do mesmo grupo, sem precisar criar um grupo exclusivo para ela.

**Req-2.15.3 — Políticas por Usuário Autenticado**

A dimensão mais granular: limites que se aplicam a usuários individuais autenticados via login, não à chave API que eles usam. A política é avaliada pelo Cedar ABAC e pode ser baseada em atributos do próprio usuário:

```
"Usuário com role = FREE recebe limite de 100 requests/hora
 Usuário com role = PRO recebe limite de 10.000 requests/hora
 Usuário com status = SUSPENSO recebe block imediato"
```

Isso permite que o tenant construa modelos de monetização diretamente na infraestrutura do Cascata — sem código de controle de acesso no backend da aplicação.

**Req-2.15.4 — Comportamentos ao Atingir Limite**

Quando um limite é atingido, o Cascata aplica um de dois comportamentos, configurável por grupo ou por chave:

**Block:** A request é rejeitada com HTTP 429 (Too Many Requests) e headers indicando quando o limite será resetado. Zero processamento adicional. Adequado para proteger recursos críticos e prevenir abuso.

**Nerf (degradação progressiva):** Em vez de rejeitar, o Cascata reduz a qualidade da resposta de forma configurável. O tenant define o percentual de degradação:

- `nerf: 50%` — retorna apenas 50% das linhas de um SELECT (as mais recentes)
- `nerf: 25%` — retorna apenas 25% das linhas com aviso no header `X-Cascata-Nerf: active`
- `nerf: fields` — retorna apenas campos não-sensíveis, omitindo colunas marcadas como premium

O Nerf é especialmente útil para produtos freemium: o usuário gratuito continua usando o produto mas com dados limitados, percebendo organicamente o valor de fazer upgrade sem ser bloqueado de forma abrupta.

**Req-2.15.5 — Migração de Chave sem Downtime**

Quando o tenant precisa rotacionar chaves API ativas (por comprometimento ou renovação de ciclo), o Cascata oferece migração com período de transição configurável: a chave antiga e a nova são válidas simultaneamente por N dias, com o sistema aceitando ambas e registrando qual percentual do tráfego já migrou. Quando 100% do tráfego está usando a nova chave, a antiga é desativada automaticamente.

**Req-2.15.6 — Painel de Traffic Guard**

O tenant visualiza em tempo real no painel:
- Requests por segundo atual vs limite configurado, por grupo
- Percentual de utilização do limite por chave ativa
- Eventos de block e nerf nas últimas 24h
- Top chaves por volume de tráfego
- Alertas configuráveis quando grupos atingem N% do limite


### 2.16 Response Rules — Middleware de Resposta no Nível de Infraestrutura

Response Rules é uma das features mais inovadoras do Cascata. Ela permite que o tenant defina, no painel, regras que interceptam o retorno de uma operação e substituem — ou enriquecem — a resposta com dados de uma fonte externa antes de devolver ao cliente.

A lógica de negócio vive na infraestrutura. O código do cliente fica simples.

---

**Req-2.16.1 — Princípio de Funcionamento**

Sem Response Rules, o fluxo padrão é:

```
Cliente → INSERT pedido → Cascata → YugabyteDB → retorna row inserida → Cliente
```

Com Response Rules configurada para esta operação:

```
Cliente → INSERT pedido → Cascata → YugabyteDB commit
  → Response Rule intercepta o retorno
  → Cascata chama o webhook configurado com os dados do evento
  → Resposta do webhook é recebida pelo Cascata
  → Cliente recebe a resposta do webhook, não a row do banco
```

O cliente fez um INSERT e recebeu uma chave Pix. Sem saber que houve uma chamada intermediária. Sem código de orquestração no backend da aplicação. Sem estado para gerenciar.

**Req-2.16.2 — Tipos de Interceptação**

Response Rules podem interceptar qualquer tipo de operação:

| Operação | Caso de uso típico |
|----------|--------------------|
| `INSERT` | Retornar confirmação de pagamento, código gerado externamente, ID de rastreamento logístico |
| `UPDATE` | Retornar estado calculado após atualização, confirmação de terceiro |
| `DELETE` | Retornar confirmação de cancelamento do sistema externo |
| `SELECT` | Enriquecer dados com informações de API externa (preço em tempo real, status de entrega, score de crédito) |
| `RPC` | Substituir completamente a resposta do banco pela resposta de um serviço externo |

**Req-2.16.3 — Configuração no Painel**

O tenant configura Response Rules no painel por tabela e por tipo de operação:

```
Tabela: pedidos
Operação: INSERT
Condição: status = "criado"  (opcional — sem condição aplica a todos os INSERTs)

Webhook de interceptação: https://gateway-pix.minhaaplicacao.com/gerar
Payload enviado ao webhook: { evento, row_inserida, tenant_id, timestamp }

Comportamento em falha do webhook:
  ├── retornar row original (fallback suave)
  ├── retornar erro 503 ao cliente (fail-hard)
  └── retornar resposta cached da última chamada bem-sucedida (TTL configurável)

Timeout do webhook: 5000ms (após isso, aplica comportamento de falha)
```

**Req-2.16.4 — Enriquecimento vs Substituição**

O tenant escolhe o modo da Response Rule:

**Substituição completa:** A resposta ao cliente é exatamente o que o webhook devolveu. A row do banco existe e foi commitada normalmente, mas o cliente nunca a vê diretamente.

**Enriquecimento:** A resposta ao cliente é a row do banco com campos adicionais injetados pela resposta do webhook. Por exemplo: row do pedido + campo `chave_pix` + campo `expiracao_pix` adicionados pelo gateway externo.

**Req-2.16.5 — Atomicidade e Consistência**

O commit no YugabyteDB ocorre antes da chamada ao webhook. A Response Rule não é transacional com o webhook — ela intercepta o retorno, não a escrita.

Se o webhook falhar após o commit, o banco está consistente. A política de falha configurada pelo tenant determina o que o cliente recebe neste caso. O evento de falha é registrado no ClickHouse com o ID da row commitada — o tenant pode reconciliar manualmente se necessário, ou configurar um mecanismo de compensação via automação.

**Req-2.16.6 — Response Rules e Agentes**

Agentes que operam via MCP também passam pelas Response Rules configuradas para cada tabela. Um agente que faz INSERT em `pedidos` recebe a chave Pix como tool result — sem configuração especial para o agente. A infraestrutura é transparente para qualquer cliente, humano ou artificial.

**Req-2.16.7 — Casos de Uso**

Response Rules resolve uma classe inteira de problemas que hoje exigem backend customizado:

- **Gateway de pagamento:** INSERT em `transacoes` retorna chave Pix, QR Code, ou link de checkout
- **Logística:** INSERT em `pedidos` retorna código de rastreamento gerado pelo transportador
- **Score de crédito:** SELECT em `clientes` retorna dados do cliente enriquecidos com score calculado por bureau externo
- **Notificação fiscal:** INSERT em `notas_fiscais` retorna número NF-e emitido pela SEFAZ
- **Validação externa:** UPDATE em `contratos` retorna confirmação assinada por sistema jurídico externo

Em todos os casos: o cliente faz uma operação simples no Cascata e recebe uma resposta rica. A complexidade da integração vive na infraestrutura do cascata, não no app/app do cliente final final.


### 2.17 Soft Delete e Recycle Bin

Nenhuma tabela no Cascata é destruída permanentemente por uma operação simples de delete. O sistema impõe uma camada de proteção obrigatória entre a intenção de deletar e a destruição irreversível de dados — independente do tier, independente de quem executa a operação.

Isso não é uma feature de conveniência. É uma decisão arquitetural que precisa existir desde o início porque o schema metadata do Cascata — usado pelo TableCreator, APIDocs, SDK type generator, MCP Server, e Protocol Cascata — precisa representar tabelas recicladas desde a primeira linha de código. Adicionar soft delete depois exige migrar toda a estrutura de metadados existente.

---

**Req-2.17.1 — Schema `_recycled` Dedicado**

Quando o operador ou tenant deleta uma tabela, ela não é removida. É movida para um schema dedicado `_recycled` dentro do mesmo banco YugabyteDB do projeto. A movimentação é atômica — executada em uma única transação DDL que garante que ou a tabela está no schema original ou está no `_recycled`, nunca em estado intermediário.

O nome da tabela no schema `_recycled` segue o padrão:

```
_recycled.{nome_original}__{timestamp_unix}__{hash_curto}

Exemplo:
  public.pedidos  →  _recycled.pedidos__1735689600__a3f7
```

O timestamp e hash curto garantem unicidade — é possível deletar e restaurar a mesma tabela múltiplas vezes sem colisão de nomes.

**Por que schema dedicado e não prefixo no schema original:**
- O schema `public` do projeto permanece limpo e representa apenas o estado atual da aplicação
- Políticas RLS configuradas para `public` não se aplicam acidentalmente a tabelas recicladas
- Qualquer cliente PostgreSQL externo enxerga a separação visualmente sem ambiguidade
- O YSQL CM bloqueia acesso ao schema `_recycled` no nível de conexão — nenhuma query de tenant chega a tabelas recicladas via SDK

**Req-2.17.2 — Proteção de Dados Durante Reciclagem**

A tabela no schema `_recycled` mantém todos os dados, todos os índices, e todas as constraints internas intactas. Nada é removido. O que muda é o isolamento:

- RLS ativado na tabela reciclada com política `DENY ALL` — zero acesso direto por qualquer role de tenant
- A tabela reciclada não aparece em nenhuma query ao schema `public`|SchemaOriginal
- O SDK não expõe tabelas recicladas em nenhuma operação — `cascata.from()` não enxerga o schema `_recycled`
- O MCP Server não expõe tabelas recicladas como ferramentas

A única interface de acesso a tabelas recicladas é o painel do Cascata na seção Recycle Bin.

**Req-2.17.3 — Protocol Cascata: Análise de Impacto Antes do Delete**

Antes de executar qualquer delete de tabela, o Cascata executa o Protocol Cascata — uma análise de impacto em cascata que identifica:

- Foreign keys em outras tabelas que referenciam a tabela sendo deletada seja qualquer schema
- Políticas RLS que mencionam esta tabela em suas expressões
- Comentários e documentação inline associados

O resultado é exibido no painel em um modal de confirmação que lista todos os impactos identificados. O operador precisa confirmar explicitamente que entende cada impacto antes do delete ser executado.

Foreign keys que referenciam a tabela deletada são automaticamente suspensas (não removidas) durante a reciclagem. Na restauração, são reativadas. No purge permanente, são removidas com aviso explícito.

**Req-2.17.4 — Retenção por Tier**

Cada tabela reciclada tem um prazo de retenção após o qual o purge automático é executado:

| Tier | Retenção default | Retenção máxima configurável |
|------|------------------|------------------------------|
| NANO | 7 dias | 7 dias (fixo) |
| MICRO | 14 dias | 30 dias |
| STANDARD | 30 dias | 90 dias |
| ENTERPRISE | 90 dias | 365 dias |
| SOVEREIGN | 365 dias | Ilimitado |

O prazo de retenção é exibido na interface da Recycle Bin com countdown visual. O operador recebe notificação via Central de Comunicação 48h antes de um purge automático.

**Req-2.17.5 — Restauração**

A restauração move a tabela de volta do schema `_recycled` para o schema `public`|'schemaOrininal' em operação atômica. Se já existe uma tabela com o mesmo nome no `public`|'SchemaOriginal' (foi recriada após o delete), o sistema oferece duas opções:

- Restaurar com nome alternativo: `{nome_original}_restored_{timestamp}`
- Sobrescrever a tabela atual (requer confirmação adicional com password do operador)

Foreign keys suspensas são reativadas após a restauração. O schema metadata é atualizado para refletir o estado `active`. O SDK e MCP Server voltam a enxergar a tabela imediatamente após a restauração.

**Req-2.17.6 — Purge Permanente**

O purge remove a tabela e todos os dados de forma irreversível. Requer:

1. Confirmação explícita digitando o nome da tabela no painel
2. Autenticação adicional do operador (senha ou Passkey)
3. Confirmação final com aviso em destaque: "Esta ação é irreversível"

Após o purge, um registro imutável é criado no ClickHouse documentando: quem executou, quando, qual schema, qual tabela, quantas linhas foram destruídas. Esse registro não pode ser deletado — é a prova de auditoria de que o purge foi intencional e autorizado.

**Req-2.17.7 — Representação no Schema Metadata**

O schema metadata do Cascata representa tabelas recicladas com os seguintes campos adicionais:

```json
{
  "table_id": "uuid",
  "original_name": "pedidos",
  "original_schema": "public",
  "recycled_name": "pedidos__1735689600__a3f7",
  "status": "recycled",
  "deleted_at": "2026-01-01T00:00:00Z",
  "deleted_by": "member_id ou agent_id",
  "scheduled_purge_at": "2026-01-31T00:00:00Z",
  "row_count_at_deletion": 15420,
  "suspended_foreign_keys": ["orders_customer_id_fkey", "..."],
  "impact_analysis": { "foreign_keys": 2, "rls_policies": 1 }
}
```

### 2.18 Computed e Virtual Columns

O Cascata suporta colunas cujo valor é calculado a partir de uma expressão — não armazenado diretamente pelo código do tenant. Isso move lógica de negócio simples para a infraestrutura, reduzindo código cliente e garantindo consistência de dados independente de qual SDK ou agente escreve no banco.

Existem dois tipos com características distintas, executados em camadas diferentes do sistema:

---

**Req-2.18.1 — Stored Generated Columns (Camada de Banco)**

Implementadas como PostgreSQL Generated Columns nativos no YugabyteDB. O valor é calculado automaticamente em cada INSERT ou UPDATE e armazenado fisicamente em disco.

Características:
- Calculada e armazenada no momento da escrita
- Zero overhead de leitura — é uma coluna normal para queries
- Pode ser indexada — queries de filtro e ordenação são tão eficientes quanto colunas regulares
- A expressão é validada pelo YugabyteDB em tempo de criação — erros de expressão são detectados antes de qualquer dado ser inserido
- Imutável do ponto de vista do SDK — tentativas de INSERT ou UPDATE em stored generated columns retornam erro tipado em compile time

Casos de uso:
```sql
-- Preço total calculado automaticamente
total_price NUMERIC GENERATED ALWAYS AS (quantity * unit_price) STORED

-- Slug gerado a partir do título
slug TEXT GENERATED ALWAYS AS (lower(regexp_replace(title, '[^a-z0-9]+', '-', 'g'))) STORED

-- Ano extraído de timestamp
year INTEGER GENERATED ALWAYS AS (EXTRACT(YEAR FROM created_at)) STORED

-- Concatenação de campos
full_name TEXT GENERATED ALWAYS AS (first_name || ' ' || last_name) STORED
```

**Req-2.18.2 — API Computed Columns (Camada Pingora)**

Para expressões que requerem contexto além do que o banco tem acesso — contexto do JWT, dados de outras tabelas via subquery controlada, ou transformações que precisam de lógica condicional complexa — o Cascata implementa API Computed Columns executadas no Pingora no momento da resposta.

A coluna não existe fisicamente no banco. O Pingora injeta o valor calculado na resposta antes de retornar ao cliente.

Características:
- Calculada em cada leitura — overhead presente mas controlado (Rust, sem GC)
- Pode acessar claims do JWT do usuário que fez a request (`jwt.sub`, `jwt.role`, etc.)
- Pode referenciar o valor de outras colunas na mesma linha
- Não pode ser usada em WHERE, ORDER BY, ou índices — não existe no banco
- Marcada claramente na APIDocs como `computed: api` para o tenant entender a limitação

Casos de uso:
```
-- Preço personalizado baseado no role do usuário
discounted_price = unit_price * (jwt.role == "vip" ? 0.85 : 1.0)

-- Flag de propriedade baseada no JWT
is_owner = (owner_id == jwt.sub)

-- Formatação contextual
formatted_date = format(created_at, tenant.timezone)
```

**Req-2.18.3 — Schema Metadata para Computed Columns**

O schema metadata do Cascata representa ambos os tipos com campos explícitos que os diferenciam:

```json
{
  "column_name": "total_price",
  "type": "numeric",
  "is_writable": false,
  "computation": {
    "kind": "stored_generated",
    "expression": "quantity * unit_price",
    "layer": "database"
  }
}

{
  "column_name": "is_owner",
  "type": "boolean",
  "is_writable": false,
  "computation": {
    "kind": "api_computed",
    "expression": "owner_id == jwt.sub",
    "layer": "api",
    "jwt_claims_required": ["sub"],
    "filterable": false,
    "sortable": false
  }
}
```

Esses campos são lidos pelo SDK type generator para garantir que:
- Colunas computed são tipadas como read-only em TypeScript
- Tentativas de escrita em colunas computed falham em compile time
- APIDocs exibe o tipo, a expressão, e a camada de computação
- MCP Server não expõe computed columns como campos escritáveis em tool calls

**Req-2.18.4 — Protocol Cascata para Computed Columns**

Antes de renomear ou deletar uma coluna que é referenciada na expressão de uma computed column, o Protocol Cascata identifica o impacto:

```
Tentativa de deletar coluna "unit_price"
  → Protocol Cascata detecta:
    - Coluna "total_price" (stored generated) depende de "unit_price"
    - Coluna "discounted_price" (api computed) depende de "unit_price"
  → Modal de impacto exibido antes de qualquer ação
  → Operador deve resolver dependências antes do delete ser permitido
```


### 2.19 Data Validation Rules na Camada API

O Cascata executa validações de dados no Pingora, antes de qualquer escrita chegar ao YugabyteDB. Essa camada de validação existe em complemento às constraints do banco — não em substituição — e resolve três limitações que constraints de banco não conseguem endereçar:

1. **Mensagens de erro legíveis por humanos** — constraints de banco retornam erros técnicos do PostgreSQL. O Cascata retorna mensagens configuradas pelo tenant no idioma e tom que o produto dele exige
2. **Contexto do JWT** — validações que dependem de quem está escrevendo (role, plano, atributos do usuário) precisam do token de autenticação que existe no Pingora, não no banco
3. **Validação cross-field** — validar que `end_date > start_date` ou que `total == quantity * unit_price` exige acesso ao payload completo da request, disponível no Pingora antes de qualquer query ao banco

---

**Req-2.19.1 — Tipos de Validação**

O tenant configura regras de validação por coluna no painel. Os tipos disponíveis:

| Tipo | Descrição | Parâmetros |
|------|-----------|------------|
| `required` | Campo obrigatório em INSERT | — |
| `regex` | Valor deve corresponder ao padrão | `pattern: string` |
| `range` | Valor numérico dentro de limites | `min?: number, max?: number` |
| `length` | Tamanho de string dentro de limites | `min?: number, max?: number` |
| `enum` | Valor deve estar na lista permitida | `values: string[]` |
| `cross_field` | Expressão envolvendo outros campos | `expression: string` |
| `jwt_context` | Validação baseada em claims do JWT | `expression: string` |
| `unique_soft` | Unicidade verificada via query antes de escrever | `scope?: string` |

**Req-2.19.2 — Cross-Field Validation**

A validação cross-field é executada com acesso ao payload completo da request. A expressão é avaliada em Rust no Pingora com um evaluator de expressões simples e seguro — sem eval de código arbitrário, sem injeção possível:

```
Exemplos de expressões cross-field válidas:

  end_date > start_date
  → "Data de término deve ser posterior à data de início"

  total == quantity * unit_price
  → "Total deve ser igual a quantidade multiplicada pelo preço unitário"

  password == password_confirmation
  → "Confirmação de senha não corresponde"

  discount_percent <= 100
  → "Desconto não pode ser superior a 100%"

  cpf_matches_regex AND age >= 18
  → "CPF inválido ou usuário menor de idade"
```

**Req-2.19.3 — JWT Context Validation**

Validações que dependem de quem está escrevendo:

```
Exemplos de expressões com contexto JWT:

  jwt.role IN ["admin", "manager"]
  → Apenas admins e managers podem escrever nesta tabela

  value <= jwt.claims.credit_limit
  → Valor não pode exceder o limite de crédito do usuário

  owner_id == jwt.sub
  → Apenas o próprio usuário pode criar registros com seu ID
  (complementa o RLS, com mensagem de erro legível)
```

**Req-2.19.4 — Configuração no Schema Metadata**

Cada coluna no schema metadata do Cascata carrega suas validações em um array de regras ordenadas:

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
      "pattern": "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$",
      "message": "Formato de email inválido",
      "severity": "error"
    }
  ]
}

{
  "column_name": "end_date",
  "type": "timestamptz",
  "validations": [
    {
      "type": "cross_field",
      "expression": "end_date > start_date",
      "fields_referenced": ["start_date"],
      "message": "Data de término deve ser posterior à data de início",
      "severity": "error"
    }
  ]
}
```

A severidade pode ser `error` (bloqueia a escrita) ou `warning` (escreve mas retorna aviso no header `X-Cascata-Warnings`).

**Req-2.19.5 — Resposta de Erro Estruturada**

Quando uma ou mais validações falham, o Cascata retorna HTTP 422 (Unprocessable Entity) com payload estruturado:

```json
{
  "error": "validation_failed",
  "violations": [
    {
      "field": "email",
      "rule": "regex",
      "message": "Formato de email inválido",
      "value_received": "nao-e-um-email"
    },
    {
      "field": "end_date",
      "rule": "cross_field",
      "message": "Data de término deve ser posterior à data de início",
      "expression": "end_date > start_date"
    }
  ]
}
```

Múltiplas violações são retornadas na mesma resposta — o tenant não precisa corrigir um campo de cada vez. O SDK tipifica o erro como `CascataValidationError` com array de `violations` acessível de forma type-safe.

**Req-2.19.6 — Exposição no SDK como Constraints em Compile Time**

O SDK type generator lê as regras de validação do schema metadata e as expressa como tipos TypeScript que fornecem feedback no IDE antes de qualquer request ser feita:

```typescript
// Gerado automaticamente pelo cascata types generate
type PedidoInsert = {
  email: string          // required + regex:email
  end_date: Date         // cross_field: end_date > start_date
  quantity: number       // range: min=1, max=999
  status: "pendente" | "aprovado" | "cancelado"  // enum
}

// No código do tenant — erro visível no IDE antes de executar:
await cascata.from('pedidos').insert({
  email: 'invalido',    // ⚠ IDE: "Formato de email inválido"
  quantity: 0,          // ⚠ IDE: "Valor mínimo: 1"
  status: 'expirado'    // ✗ Erro de tipo — não está no enum
})
```

**Req-2.19.7 — Relação com Constraints do YugabyteDB**

As validações do Pingora e as constraints do banco são camadas complementares:

- **Validação Pingora:** executada antes da query, retorna erro legível, cobre lógica cross-field e contexto JWT
- **Constraint do banco:** executada na query, é a garantia de última linha de integridade dos dados

Se uma validação Pingora falha, nenhuma query chega ao banco. Se por alguma razão a query chega ao banco e a constraint do banco falha, o Cascata captura o erro do YugabyteDB e o traduz para o mesmo formato estruturado `CascataValidationError` antes de retornar ao cliente.

O tenant nunca vê mensagens de erro cruas do PostgreSQL — em nenhuma circunstância.



### 2.20 Arquitetura de Extensões — Extension Profile System

Extensões do YugabyteDB que requerem arquivos binários compilados (`.so`) precisam existir em **todos os tserver nodes** do cluster simultaneamente. Não podem ser injetadas em um nó enquanto outros operam — isso exigiria restart coordenado de todo o cluster. A consequência arquitetural é direta: **a presença de uma extensão é uma propriedade do cluster, não do tenant**. O binário vive na imagem Docker. O tenant não instala uma extensão — ele habilita o uso de uma extensão que o cluster já possui.

Essa distinção elimina o problema de desperdício por multiplicação: 200 tenants em um cluster compartilhado que usam PostGIS compartilham um único binário compilado na imagem. Não há 200 instalações, não há 200 processos de compilação, não há 200 superfícies de ataque adicionais.

---

**Req-2.20.1 — Duas Imagens, Dois Contextos**

O Cascata mantém exatamente duas variantes da imagem YugabyteDB:

**`cascata/yugabytedb:shared`**
Imagem lean para o cluster compartilhado NANO/MICRO. Contém apenas extensões nativas do YugabyteDB e `pg_cron` como base do sistema de Cron Jobs. Nenhuma extensão pesada — PostGIS e similares não estão presentes. O objetivo é minimizar o footprint do cluster compartilhado, onde cada MB de RAM é recurso de todos os tenants simultaneamente.

Usada em: cluster compartilhado NANO/MICRO em modo Shelter.

**`cascata/yugabytedb:full`**
Imagem completa para toda instância dedicada. Contém todos os Extension Profiles compilados — PostGIS, PostGIS Tiger Geocoder, PostGIS Topology, e quaisquer extensões adicionadas ao ecossistema Cascata em releases futuros. O desenvolvedor com instância dedicada tem acesso a qualquer extensão sem escolha prévia, sem upgrade friction, sem rolling upgrade de profile.

Usada em: toda instância STANDARD, ENTERPRISE e SOVEREIGN.

A divisão é binária e permanente. Não existe imagem intermediária, não existe escolha de profile no provisionamento de instância dedicada.

---

**Req-2.20.2 — Classificação de Extensões**

**Categoria 1 — Nativas (sempre disponíveis em ambas as imagens)**

Extensões que fazem parte do binário do YugabyteDB sem nenhuma ação de instalação:

| Extensão | Uso |
|----------|-----|
| `uuid-ossp` | Geração de UUIDs |
| `pgcrypto` | Funções de criptografia |
| `pg_trgm` | Similaridade de texto, busca fuzzy |
| `hstore` | Pares chave-valor |
| `citext` | Texto case-insensitive |
| `ltree` | Hierarquias em árvore |
| `intarray` | Operações em arrays de inteiros |
| `unaccent` | Remoção de acentos para busca |
| `fuzzystrmatch` | Algoritmos de similaridade fonética |
| `pg_stat_statements` | Análise de performance de queries |
| `pgaudit` | Auditoria de operações SQL |
| `postgres_fdw` | Foreign Data Wrapper para PostgreSQL externo |
| `dblink` | Conexões a bancos externos |
| `plpgsql` | Linguagem procedural padrão |

**Categoria 2 — Pré-compiladas na imagem base (disponíveis em ambas as imagens)**

| Extensão | Motivo |
|----------|--------|
| `pg_cron` | Base do sistema de Cron Jobs do Cascata. Não opera nativamente em YugabyteDB distribuído sem wrapper — o Control Plane gerencia o isolamento por tenant usando pg_cron como executor interno |

**Categoria 3 — Compiladas apenas na imagem `:full`**

| Extensão | Dependências compiladas | Uso |
|----------|------------------------|-----|
| PostGIS | libgeos, libproj, libgdal | Dados geoespaciais, logística, mapas |
| PostGIS Tiger Geocoder | idem | Geocodificação endereços |
| PostGIS Topology | idem | Topologia geoespacial |

Compatibilidade YugabyteDB documentada por extensão no Extension Marketplace do painel — features com limitações conhecidas são sinalizadas com aviso antes da habilitação.

**Categoria 4 — Bloqueadas (nenhuma das imagens)**

| Extensão | Motivo |
|----------|---------| 
| `pgvector` | Substituído pelo Qdrant — banco vetorial dedicado com isolamento multi-tenant correto, performance superior e DR integrado |
| Extensões que executam código arbitrário externo | Conflito com o modelo de segurança do Cascata |

---

**Req-2.20.3 — Habilitação de Extensão por Tenant**

O tenant habilita uma extensão via Extension Marketplace no painel. A habilitação executa `CREATE EXTENSION` no schema do projeto — nenhuma mudança de infraestrutura, nenhum restart, nenhuma interação com o operador.

A habilitação segue o padrão de escrita em duas fases: a intenção é registrada no Control Plane com status pending antes da execução do CREATE EXTENSION. A confirmação para active ocorre apenas após sucesso confirmado no banco do tenant. Registros pending com mais de 5 minutos são tratados pelo job de reconciliação.

Para cluster compartilhado (NANO/MICRO com imagem `:shared`): apenas extensões das Categorias 1 e 2 estão disponíveis. Extensões da Categoria 3 não aparecem como opção — a imagem não as contém.

Para instâncias dedicadas (STANDARD/ENTERPRISE/SOVEREIGN com imagem `:full`): todas as extensões das Categorias 1, 2 e 3 estão disponíveis imediatamente. O tenant habilita qualquer extensão com um clique sem nenhum processo adicional.

---

**Req-2.20.4 — Isolamento de Extensão por Schema**

Mesmo que o binário seja compartilhado no cluster, cada tenant habilita e usa a extensão dentro do próprio schema. PostGIS habilitado pelo tenant A não interfere com o tenant B — as funções e types da extensão são objetos do schema do tenant, não globais.

Para `pg_cron`, que cria objetos no schema `cron` compartilhado: o Cascata expõe um wrapper no schema do tenant que delega para o agendador central gerenciado pelo Control Plane. O tenant usa a API de Cron Jobs do Cascata — nunca o schema `cron` diretamente. Isso mantém o isolamento e preserva o Control Plane como único gestor do agendamento.

---

**Req-2.20.5 — Extension Marketplace no Painel**

Interface no painel que lista todas as extensões disponíveis para o cluster do projeto. Para cada extensão:

- Status: `disponível` / `habilitada neste projeto` / `indisponível nesta imagem` / `bloqueada` / `pending`
- Compatibilidade YugabyteDB: `total` / `parcial` (com limitações documentadas inline)
- Impacto estimado: overhead de storage e performance quando conhecido
- Snippet de uso: exemplo SQL imediato após habilitação

Habilitação de extensão disponível: imediata, sem confirmação adicional além de um clique.
Extensão indisponível na imagem `:shared`: o painel informa que a extensão requer instância dedicada (upgrade de tier), sem botão de habilitação.


---

## 3. Requisitos Não Funcionais

### 3.1 Desempenho

| Métrica | Target | Mecanismo |
|---------|--------|-----------|
| Latência adicionada pelo Gateway Pingora | < 0.5ms p99 | Rust nativo, XDP bypass via Cilium |
| Latência do cache DragonflyDB | < 0.1ms | C++ multi-thread, sem GC |
| Cold boot de Sandbox (Deno) | < 5ms | V8 snapshots pré-compilados |
| RTT total para request transacional simples | < 5ms p99 | Pipeline completo otimizado |
| RTT total para cache hit | < 1ms p99 | Short-circuit no DragonflyDB |
| Throughput de ingestão de eventos | > 2M eventos/segundo | Redpanda C++ sem JVM |
| Throughput de writes transacionais | > 500K/segundo (cluster base) | YugabyteDB distributed SQL |
| Throughput de reads com cache | > 5M/segundo (cluster base) | DragonflyDB multi-thread |

Escala horizontal linear: dobrar nós deve dobrar throughput. Qualquer gargalo não-linear é tratado como bug arquitetural.

### 3.2 Escalabilidade

**Req-3.2.1 — Modo Shelter (Day 1)**

O sistema completo para tiers NANO e MICRO deve operar em uma única VPS de $20/mês rodando todos os binários via Docker Compose sem crash por OOM. Isso é obrigação de design — não é aspiração. O Cascata deve ser utilizável e completo em funcionalidade desde o primeiro container levantado.

Estimativa de footprint em modo Shelter:
- Pingora Gateway: ~30MB RAM
- Control Plane (Go): ~20MB RAM
- YugabyteDB Shared Cluster: ~800MB RAM
- DragonflyDB: ~100MB RAM
- Redpanda (1 broker): ~200MB RAM
- ClickHouse (modo single node): ~300MB RAM
- OpenBao: ~50MB RAM
- Dashboard SvelteKit (build estático servido pelo Gateway): ~0MB RAM adicional

Total estimado: ~1.8GB RAM. Operacional em VPS de 3GB com margem.

**Req-3.2.2 — Escala Planetária**

Quando um tenant percebe que seu projeto MVP precisa de escala real, ele acessa o painel do Cascata e altera um único parâmetro apontando para clusters externos (AWS, GCP, bare metal). O resultado:

- Zero downtime
- Zero edição manual de configuração de infraestrutura
- Zero mudança de código no cliente do tenant
- Zero nova curva de aprendizado — é a mesma interface, o mesmo SDK

O Control Plane provisiona, migra e atualiza o routing automaticamente.

**Req-3.2.3** A promoção de tier deve ser transparente para o usuário final do tenant. O endpoint de API permanece idêntico em todos os tiers. A promoção pode ser iniciada pelo operador do Cascata, pelo tenant (se habilitado) ou acionada automaticamente por thresholds configuráveis.

### 3.3 Compliance por Design

O compliance no Cascata não é uma camada adicionada — é a fundação. Cada decisão de storage, networking, criptografia e audit foi tomada com os requisitos regulatórios como restrição de primeira classe, não como feature adicional.

**LGPD / GDPR**

- Data residency nativo: tablespaces YugabyteDB mapeados por país. Dado brasileiro permanece em nós brasileiros por design de storage, não por middleware
- Deleção por crypto-shredding: revogar a envelope key do tenant no OpenBao torna matematicamente impossível reconstruir qualquer dado nos clusters YugabyteDB ou ClickHouse. Não existe `DELETE CASCADE` em dado de produção — a deleção lógica é sempre criptográfica
- Data lineage completo: todo dado tem audit trail imutável desde a criação
- Consentimento gerenciado como recurso de primeira classe na API do Cascata — registrado, versionado e auditável

**PCI-DSS**

- Network segmentation via Cilium: isolamento físico OSI Layer 4 entre tenants
- Encryption in-transit: mTLS obrigatório em toda comunicação interna
- Encryption at-rest: AES-256-GCM com BYOK gerenciado via OpenBao
- Audit logs imutáveis, assinados digitalmente, armazenados em ClickHouse com política WORM (Write Once Read Many)
- Zero trust entre todos os serviços — nenhuma comunicação interna ocorre sem autenticação mútua

**HIPAA**

- PHI (Protected Health Information) detectado automaticamente por tag e isolado em namespace dedicado no momento da criação
- Todo acesso a dado sensível é logado com contexto completo: quem acessou, quando, de qual dispositivo, de qual IP, com qual justificativa declarada
- Network Policies Kubernetes via eBPF criam barreira física de visibilidade entre tenants — impossível de contornar por código da aplicação
- Break-glass access para emergências com audit trail especial e notificação imediata ao administrador do tenant
- BAA (Business Associate Agreement) gerado automaticamente no onboarding de tenants com dados de saúde

### 3.4 Segurança

**Req-3.4.1** Toda request que chega ao Cascata passa pela validação de Cedar ABAC antes de qualquer acesso a dado. Não existe bypass autorizado.

**Req-3.4.2** Toda comunicação service-to-service dentro do Cascata usa mTLS. Não existe comunicação interna em plaintext.

**Req-3.4.3** Secrets, chaves de API e tokens de integração são armazenados exclusivamente no OpenBao, com rotação automática e audit trail de todo acesso.

**Req-3.4.4** O isolamento de tenant em tiers ENTERPRISE e SOVEREIGN é garantido em nível de kernel via Cilium/eBPF. IP spoofing entre tenants é fisicamente impossível — não é prevenido por código, é prevenido pela rede.

**Req-3.4.5** Magic bytes validation em todo upload é obrigatória e não pode ser desabilitada por nenhum tenant em nenhum tier.

**Req-3.4.6** Toda autenticação que envolva código ou token comparado deve usar comparação em tempo constante. Comparações de string por igualdade em contexto de autenticação são proibidas no codebase.



### 3.5 Connection Pooling — Arquitetura de Conexão

O Cascata opera com centenas a milhares de tenants simultâneos contra um conjunto finito de instâncias YugabyteDB. Sem uma arquitetura de conexão corretamente projetada, o sistema satura antes de ser útil para qualquer cenário de produção real.

O problema não é simples. Cada conexão PostgreSQL no YugabyteDB é um processo do sistema operacional com overhead de ~8MB de RAM, stack próprio, e buffers dedicados. Em YugabyteDB distribuído, cada conexão mantém adicionalmente estado interno sobre qual tablet server é líder para quais shards. O limite prático de conexões simultâneas antes de degradação severa é de 300-500 por instância em hardware moderado — após isso, o scheduler do OS gasta mais tempo em context switching entre processos de conexão do que executando queries.

O Cascata resolve isso com duas camadas complementares de pooling, cada uma com responsabilidade distinta e não-sobreponível.

---

**Req-3.5.1 — Duas Camadas com Responsabilidades Distintas**

```
Pingora (Rust) — request validada, JWT verificado, ABAC aprovado
        ↓
YSQL CM (Connection Manager) — POOLER NATIVO
  Responsabilidade: roteamento multi-tenant, load balancing
  de réplicas, pool por instância de banco
        ↓
YSQL Connection Manager — CAMADA INTERNA
  Responsabilidade: multiplexing de conexões físicas,
  roteamento interno de tablets e shards do YugabyteDB
        ↓
YugabyteDB (conexões físicas reais — dezenas, não milhares)
```

As duas camadas resolvem problemas que individualmente nenhuma consegue resolver sozinha:

- **YSQL CM** sabe sobre tenants, sobre réplicas, sobre qual instância de banco pertence a qual projeto
- **YSQL CM** sabe sobre a topologia interna do YugabyteDB — tablets, shards, qual node tem os dados mais "quentes"

Remover qualquer uma das duas cria um gargalo que a outra não consegue compensar.

---

**Req-3.5.2 — YSQL CM: Connection Pooler Nativo Multi-Tenant**

**YSQL Connection Manager** é o pooler embutido no YugabyteDB 2.25+. Diferente de ferramentas externas, ele opera na camada wire do PostgreSQL e do YugabyteDB simultaneamente, compreendendo nativamente os estados de transação, prepared statements e variáveis `SET`.

*Responsabilidades do YSQL CM no Cascata:*

**Pool por instância de banco:** O YSQL CM mantém um pool de conexões dedicado internamente. O roteamento é feito pelo Control Plane / Gateway que injeta o tenant correto.

**Read/Write Splitting automático:** O driver nativo e o YSQL CM roteiam leituras consistentes para nós secundários sem necessitar de parse SQL na camada externa.

**Server Reset Query transacional:** Antes de devolver qualquer conexão ao pool, o estado é totalmente zerado. Isso permite executar o *RLS Handshake* (`SET LOCAL app.tenant_id`) com total segurança entre locatários utilizando a mesma conexão física. Isso garante matematicamente que nenhum estado de sessão de um tenant vaza para a próxima transação. A limpeza é inviolável independente do que aconteça no código.

**Prepared Statements Cache:** Em virtude da integração nativa, prepared statements são retidos na conexão sem limitação do protocolo estendido, superando limitações clássicas de poolers externos em transaction mode.

**Isolamento de tenant no pool:** Um tenant NANO com limite enfileira na camada TCP do próprio pooler. Excesso não derruba os processos principais do banco de dados (tservers).

---

**Req-3.5.3 — YSQL Connection Manager: Multiplexer Nativo YugabyteDB**

O **YSQL Connection Manager** é o pooler embutido no YugabyteDB desde a versão 2.18. É baseado no Odyssey mas reescrito para entender a arquitetura interna do YugabyteDB.

*Por que um pooler nativo ao banco importa:*

O YugabyteDB mantém internamente um mapa de qual tablet server é o líder para cada shard de cada tabela — a topologia de tablets. Uma conexão externa, incluindo o pgcat, não tem acesso a esse mapa. Ela fala com qualquer node e o node faz o roteamento interno, adicionando um hop de latência.

O YSQL CM embutido multiplexa requests transacionais em um número muito menor de conexões físicas ("backends" lógicos). O resultado:

Gateway → YSQL CM: 100 conexões lógicas (virtuais)
YSQL CM → YugabyteDB internals: 10-20 conexões físicas reais

Overhead de RAM das conexões físicas: 20 × 8MB = 160MB
vs
Sem pooling: 100 × 8MB = 800MB

Apenas nesta camada: economia de 640MB por 100 tenants ativos
```

*Compatibilidade com RLS Handshake:*

O YSQL CM foi construído sabendo que `SET LOCAL ROLE` e `set_config()` dentro de transações são padrões de uso esperados. Não tem os bugs históricos do PgBouncer com `SET` commands em transaction mode. A combinação `BEGIN → SET LOCAL → query → COMMIT` passa pelo YSQL CM sem nenhuma modificação ou workaround.

*Footprint:* ~0MB RAM adicional — é embutido no processo do YugabyteDB, não um processo separado.

---

**Req-3.5.4 — mTLS em Toda a Cadeia de Conexão**

A regra de zero trust se aplica à camada de pooling:

```
Pingora ←── mTLS ──→ YSQL CM ←── mTLS ──→ YugabyteDB Backend Engine

**Pinos de Autenticação:**
- Certificado do Banco emitido pela CA do projetonBao com rotação automática. A cadeia de confiança é:
- CA raiz do projeto armazenada no OpenBao
- Certificado do YSQL CM emitido pela CA do projeto
- Certificado do YugabyteDB emitido pela CA do projeto
- Rotação automática antes da expiração sem downtime de conexão
```

Nenhuma comunicação entre componentes de pooling ocorre em plaintext em nenhum tier, incluindo NANO em modo Shelter.

---

**Req-3.5.5 — Impacto Real em RAM — Modo Shelter**

```
Cenário: 200 tenants ativos, média 5 requests simultâneas por tenant

SEM pooling:
  1000 conexões físicas × 8MB = 8.000MB de RAM em handles
  Resultado: sistema inutilizável — 8GB apenas para conexões

COM YSQL CM NATIVO:
  YSQL CM (1 Thread per CPU): Multiplexa sem consumo adicional externo.
  YSQL CM: 200 conexões lógicas → 15-20 conexões físicas reais
  Conexões físicas: 20 × 8MB = 160MB

  Total pooling: ~185MB
  Economia: ~7.815MB de RAM

  Esses ~7.8GB liberados vão diretamente para o cache de páginas
  do YugabyteDB → mais cache hits → menos I/O → menos latência
```

A economia de RAM não é sobre economizar memória — é sobre converter memória desperdiçada em handles de conexão em cache de dados ativo. Cada MB liberado do pool de conexões vira MB disponível para o YugabyteDB cachear páginas de tabela. Isso reduz diretamente a latência de queries para todos os tenants.

---

**Req-3.5.6 — Observabilidade do Pipeline de Conexão**

O pgcat expõe métricas Prometheus nativas. O pipeline de observabilidade do Cascata captura:

- Conexões ativas por pool (por tenant, por tier)
- Tempo médio de espera na fila de conexões
- Taxa de reuso de conexões (pool hit rate)
- Read/Write split ratio — % de queries roteadas para réplica vs primário
- Server reset query execution time
- Conexões físicas abertas no YSQL CM

Todas as métricas fluem via OTel → VictoriaMetrics → Dashboard do Cascata. O operador vê em tempo real a saúde do pool de conexões por tenant e pode identificar tenants com comportamento anômalo antes que causem degradação.

---

**Req-3.5.7 — O Control Plane como Gestor de Recursos**

O Control Plane em Go é o cérebro que opera as regras de negócio de tenant:

1. Atualiza e aplica metadados de pools virtuais
2. Gerencia a criação e upgrade de tiers
3. Coordena métricas Prometheus direto da engine (15433).

Na promoção MICRO → STANDARD, o Control Plane ajusta automaticamente as quotas de conexões simultâneas permitidas pela base de dados.

O Control Plane inspeciona a engine ativa através de portas HTTP métricas e fallbacks.
1. DR Orchestrator é notificado imediatamente
2. Pingora é instruído a usar conexão direta com YSQL CM temporariamente (fallback degradado)
O YSQL CM mantém conexões de backend pré-aquecidas — sem essa configuração, o primeiro request paga o custo de fork de processo. Com pooler nativo, esse custo é zero para o usuário final.


**Req-3.5.8 — Thundering Herd: Staggered Pool Warming**

*O problema:*
Quando múltiplos tenants hibernados acordam simultaneamente — como ocorre toda manhã em instâncias com dezenas de projetos inativos à noite — cada um precisa reconectar, executar TLS handshake com o YSQL CM, autenticar, e aquecer buffers internos do YugabyteDB. Esse spike de reconexão simultânea cria um momento de contenção no YSQL CM que degrada a latência do primeiro request de todos os tenants envolvidos. O problema escala proporcionalmente ao número de tenants — inaceitável para o Cascata que pode operar centenas de projetos NANO/MICRO em uma única instância.

*A solução — Staggered Pool Warming pelo Control Plane:*

O Control Plane monitora continuamente os timestamps de última atividade de todos os tenants. Quando detecta que múltiplos tenants estão próximos do threshold de hibernação — ou que múltiplos tenants hibernados vão receber tráfego em janela próxima (via padrão histórico de horário) — ele distribui o aquecimento de pools com jitter intencional calculado:

```
Função de distribuição de warming:
  base_time = horário de pico histórico do tenant (extraído do ClickHouse)
  jitter = hash(tenant_id) % warming_window_ms
  wake_time = base_time - pre_warm_lead_time + jitter

  Exemplo com 50 tenants e warming_window de 10 segundos:
    Tenant A: aquece pool às 05:59:847
    Tenant B: aquece pool às 05:59:934
    Tenant C: aquece pool às 06:00:012
    ...
    Tenant 50: aquece pool às 06:00:743
```

O jitter é determinístico por `tenant_id` — não aleatório a cada ciclo. Isso garante que o mesmo tenant acorda sempre no mesmo offset relativo, tornando o comportamento previsível e testável. O hash do `tenant_id` distribui os tenants uniformemente pela janela sem necessidade de coordenação central em tempo real.

Resultado: cada tenant tem conexões pré-aquecidas e validadas antes do primeiro request chegar. O thundering herd é eliminado na origem — não tratado depois que já está causando contenção. A latência do primeiro request de cada tenant após hibernação é idêntica à latência de qualquer outro request.

*Pre-warm lead time por tier:*

| Tier | Lead time de aquecimento | Justificativa |
|------|--------------------------|---------------|
| NANO | 60 segundos antes do pico | Pool mínimo, reconexão rápida |
| MICRO | 90 segundos antes do pico | Pool ligeiramente maior |
| STANDARD | 120 segundos antes do pico | Instância dedicada — TLS handshake adicional |
| ENTERPRISE | 180 segundos antes do pico | Múltiplos nós, certificados mTLS por nó |
| SOVEREIGN | Configurável pelo operador | Ambiente on-premise pode ter latências distintas |

---

**Req-3.5.9 — Statement Timeout por Tier: Proteção de Pool**

*O problema:*
Em transaction pooling mode, uma transação que não executa COMMIT ou ROLLBACK segura uma conexão do pool indefinidamente. Uma função mal-escrita, uma query com deadlock não resolvido, ou um cliente que desconecta sem fechar a transação transformam uma conexão do pool em um recurso perdido para sempre — até o operador intervir manualmente. Em um cluster compartilhado com pool limitado, uma única transação travada pode esgotar conexões disponíveis para outros tenants.

*A solução — Statement Timeout integrado na base de dados:*

O YSQL CM e o PostgreSQL engine finalizam instruções que ultrapassam a alocação de tempo sem derrubar o processo YugabyteDB principal:

| Tier | Statement Timeout | Idle in Transaction Timeout |
|------|-------------------|----------------------------|
| NANO | 10 segundos | 5 segundos |
| MICRO | 30 segundos | 15 segundos |
| STANDARD | 60 segundos | 30 segundos |
| ENTERPRISE | 300 segundos | 120 segundos |
| SOVEREIGN | Configurável | Configurável |

**Idle in Transaction Timeout** é distinto do Statement Timeout: enquanto o Statement Timeout encerra queries lentas, o Idle in Transaction Timeout encerra transações que foram abertas (BEGIN) mas ficaram sem atividade — o caso clássico de cliente que desconecta sem fechar a transação explicitamente.

*Ao cancelar a transação no banco:*

O que acontece com centenas de requests? Elas formam filas virtuais. 

Quando um limite de fila configurado pelo tserver é atingido, o Yugabyte rejeita (backpressure).

```
Estágio 1 — Fila entre 0% e 70% da capacidade:
  Comportamento normal. Request enfileirada e aguarda conexão.
  Header de resposta: X-Cascata-Queue-Depth: {n}

Estágio 2 — Fila entre 70% e 90% da capacidade:
  Request enfileirada com timeout reduzido pela metade.
  Header de resposta: X-Cascata-Queue-Pressure: moderate
  Control Plane é notificado — avalia se promoção de tier é necessária

Estágio 3 — Fila acima de 90% da capacidade:
  Request rejeitada imediatamente com HTTP 503 (Service Unavailable)
  Header: X-Cascata-Queue-Pressure: critical
  Header: Retry-After: {estimativa em segundos até fila liberar}
  Control Plane recebe alerta crítico — tenant candidato a promoção automática
  Evento no ClickHouse com contexto completo
```

*Tamanhos de fila por tier:*

| Tier | Pool Size | Queue Limit | Justificativa |
|------|-----------|-------------|---------------|
| NANO | 5 conexões | 50 requests | 10× o pool — suficiente para spikes curtos |
| MICRO | 10 conexões | 100 requests | 10× o pool |
| STANDARD | 25 conexões | 500 requests | 20× o pool — SLA contratado exige mais buffer |
| ENTERPRISE | 100 conexões | 2000 requests | 20× o pool |
| SOVEREIGN | Configurável | Configurável | Tenant define baseado em seu SLA |

O backpressure graduated serve dois propósitos: protege o sistema de exaustão de memória E fornece sinais ao Control Plane para decisões de promoção de tier antes que o tenant experimente degradação severa.

---

**Req-3.5.11 — Circuit Breaker: Proteção contra Falha do YSQL CM**

*O problema:*
Se a conexão transacional periga e oscila, os Gateways de ponta do Pingora e os Control Planes reagem com backoff. O Pingora responde mais rápido que um TCP close tradicional.

*A solução — Circuit Breaker nativo no Gateway Control:*

O pgcat implementa o padrão Circuit Breaker por pool de tenant com três estados:

```
CLOSED (estado normal):
  Conexões fluem normalmente para o YSQL CM.
  pgcat monitora taxa de erros e latência de conexão.
  Threshold de abertura: >50% de falhas de conexão em janela de 10s
  OU latência de conexão >500ms por 5s consecutivos.

OPEN (downstream indisponível):
  pgcat para de tentar conectar ao YSQL CM.
  Retorna HTTP 503 imediatamente para novas requests — sem fila, sem espera.
  Notifica o Control Plane via canal dedicado (não via Redpanda — canal direto
  para garantir que a notificação não depende do sistema que pode estar falhando).
  DR Orchestrator é acionado imediatamente.
  Duração do estado OPEN: 30 segundos (configurable por tier).

HALF-OPEN (verificando recuperação):
  Após 30s, pgcat tenta uma conexão de probe para o YSQL CM.
  SE bem-sucedida: transiciona para CLOSED, retoma tráfego normalmente.
  SE falha: retorna para OPEN por mais 30s.
  Lógica de probe: exponential backoff — 30s, 60s, 120s, até DR Orchestrator
  confirmar que o YSQL CM está saudável.
```

*Coordenação com o Pingora:*

Quando o pgcat transiciona para OPEN, notifica o Pingora via interface de controle local (Unix socket — zero latência de rede, zero dependência de infraestrutura externa). O Pingora atualiza o health check do pool afetado e para de rotear para ele. O tenant afetado recebe HTTP 503 com `Retry-After` calculado pelo DR Orchestrator — não um timeout silencioso.

*Granularidade do circuit breaker:*

O circuit breaker opera por instância de banco, não por pool de tenant. Se o YSQL CM da instância dedicada do tenant A falha, o circuit breaker do tenant A abre. Os outros tenants no mesmo pgcat não são afetados. Cada pool tem seu próprio circuit breaker independente.

---

**Req-3.5.12 — Pool Size Dinâmico: Fórmula de Dimensionamento**

*O problema implícito não resolvido:*
Como o Control Plane sabe qual `pool_size` atribuir a cada tenant? Definir um valor fixo por tier é conservador demais para tenants com tráfego alto ou permissivo demais para tenants com tráfego baixo. Um sistema verdadeiramente eficiente precisa dimensionar o pool baseado em dados reais.

*A solução — Pool Size Adaptativo via Control Plane:*

O Control Plane calcula e ajusta `pool_size` por tenant usando a seguinte fórmula:

```
pool_size = max(
  tier_minimum,
  ceil(
    p95_concurrent_transactions_last_7d
    × safety_factor_by_tier
    × (1 + growth_trend_coefficient)
  )
)

Onde:
  p95_concurrent_transactions_last_7d:
    extraído do ClickHouse — percentil 95 de transações simultâneas
    nos últimos 7 dias para este tenant

  safety_factor_by_tier:
    NANO:       1.2  (20% de margem)
    MICRO:      1.3
    STANDARD:   1.5  (50% de margem — SLA contratado)
    ENTERPRISE: 2.0
    SOVEREIGN:  configurável

  growth_trend_coefficient:
    slope da regressão linear de crescimento de tráfego nos últimos 30 dias
    tenant crescendo 10%/semana → coefficient = 0.10
    tenant estável → coefficient = 0.0
    tenant decrescendo → coefficient negativo (mas limitado pelo tier_minimum)

  tier_minimum:
    NANO:       3
    MICRO:      5
    STANDARD:   10
    ENTERPRISE: 25
    SOVEREIGN:  configurável
```

O recálculo ocorre a cada 24h pelo Control Plane via batch job no ClickHouse. Ajustes são aplicados no pgcat via reconfiguration a quente — sem fechar conexões ativas, sem downtime, sem intervenção manual.



---

## 4. Decisões Permanentes de Tecnologia

As seguintes decisões são permanentes e não estão sujeitas a revisão por preferência individual. Revisão exige ADR (Architecture Decision Record) com benchmark documentado e aprovação explícita.

| Decisão | Escolha Final | Motivo |
|---------|--------------|--------|
| Runtime do Control Plane | Go 1.26+ | Concorrência nativa via goroutines, binário único, zero dependência de ecossistema externo, inicialização em milissegundos |
| API Gateway / WAF | Pingora (Rust) | Construído sobre Tokio pela Cloudflare, testado em 1 trilhão de requests/dia, resolve connection pooling, TLS e load balancing com latência <0.5ms p99 |
| Dashboard DX | SvelteKit + TypeScript | Codebase compilada, zero runtime do framework em produção, surface de ataque reduzida, TypeScript nativo, performance superior para dashboards com estado complexo |
| Auth Model | Cedar ABAC | Único modelo que suporta políticas contextuais enterprise com verificação matemática formal. Permite combinar role, IP, horário, dispositivo e status de compliance em uma única expressão verificável |
| OLTP / Banco Principal | YugabyteDB | PostgreSQL distribuído nativo, sharding automático, ACID completo, multi-region com consistência forte, tablespaces por país para data residency nativo |
| Cache | DragonflyDB | C++ multi-thread, API 100% Redis-compatible, latência <0.1ms, escala vertical aproveitando todos os núcleos sem configuração adicional |
| Streaming | Redpanda | C++ sem JVM, latência p99 <1ms, API 100% Kafka-compatible, sem dependências externas de coordenação |
| KMS | OpenBao (Linux Foundation, MPL 2.0) | Fork open source do Vault, API idêntica, sem licença proprietária, mantido pela Linux Foundation |
| Networking | Cilium / eBPF | Isolamento em nível de kernel, zero overhead de sidecar proxy, mTLS transparente, IP spoofing entre tenants fisicamente impossível |
| Observabilidade | OpenTelemetry → VictoriaMetrics + ClickHouse | Single binary, pipeline vendor-neutral, visualização integrada no próprio dashboard do Cascata — sem dependência de ferramenta externa |
| Logs / Analytics | ClickHouse | Motor colunar OLAP, compressão 1TB→80GB, queries em bilhões de linhas em segundos, TTL e tiered storage por tenant nativos |
| Object Storage | MinIO + Camada de Adaptadores | MinIO como backbone soberano self-hosted; camada de adaptadores para S3, R2, Wasabi, Cloudinary, ImageKit, Google Drive, OneDrive, Dropbox |
| Orquestração de Containers | Kubernetes | Namespace por tenant para ENTERPRISE/SOVEREIGN, Operators customizados para provisionamento automático, HPA nativo, multi-cloud por definição |
