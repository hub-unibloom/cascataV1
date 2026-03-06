# TASK — Fase 1: A Forma do Castelo (Fundação DX e Dashboard)
# Cascata "Koenigsegg" — O Ambiente Visual

> **Objetivo:** O "Alicerce Invisível" da Fase 0 construiu a fundação (Control Plane, Pingora, DBMS). Agora, chegou a hora de dar **"Forma ao Castelo"**. Ninguém consegue morar num terreno só com encanamento — é preciso paredes, cômodos, interruptores e janelas. Esta fase materializa a Developer Experience (DX) do Cascata por meio do Dashboard SvelteKit. Apenas através desta interface o Tenant Provisioning, o Database Explorer, a configuração do Storage e do Cedar ABAC ganharão vida real para o Operador.

> **Referências supremas:**
> - [Roadmap](file:///home/cocorico/projetossz/cascataV1/Roadmap_48_Implementacoes_Cascata.md) — Etapa 1 (Fundação da DX) e itens de Interface
> - [SRS](file:///home/cocorico/projetossz/cascataV1/SRS_CascataV1.md) — Dashboard SvelteKit, WebGL FPS, Configurações de Tenant
> - [SAD](file:///home/cocorico/projetossz/cascataV1/SAD_CascataV1.md) — DX Compilada, Zero Framework Runtime em produção

---

## 1. Scaffold e Conectividade Base (SvelteKit)

Antes de construir os cômodos, precisamos da argamassa visual e das rodovias que ligam o front-end ao Control Plane.

### 1.1 — Setup do SvelteKit e Framework Visual
**Dependências:** Arquitetura de diretórios `dashboard/` da Fase 0.
- [ ] Inicializar SvelteKit estrito (TypeScript nativo, zero `any`, SSR configurado para static/hybrid dependendo da rota).
- [ ] Configurar TailwindCSS e o Design System (Cores de escala, Dark Mode first, interações micro-animadas).
- [ ] Configurar layout base modular: `Sidebar` de navegação contextual, `Header` global (Breadcrumbs e seleção de projeto), `Main Content Area`.
- [ ] Configurar cliente isolado de API (`fetch` wrapper unificado) para conversar com a porta `9090` (Control Plane) repassando tokens de autorização seguros.
  - O painel nunca fala direto com o YugabyteDB; toda mutação estrutural flui via Control Plane.

**Ref:** SRS Req-2.1.3 (SvelteKit TypeScript-first)

### 1.2 — Autenticação do Operador (O Portão do Castelo)
**Dependências:** 1.1
- [ ] UI de Login (Design impecável, suporte a Magic Link / Passkey / Password).
- [ ] `hooks.server.ts`: Implementar Server-Side Route Guard. Se o cookie JWT do Operador/Tenant não existir ou for inválido, block na navegação antes do layout renderizar.
- [ ] Tela de "Overview" ou "Meus Projetos" após login.
- [ ] Implementar Svelte Stores (`projectStore`, `userStore`) para reatividade centralizada do estado atual sem prop-drilling.

---

## 2. O Coração do Castelo: Database Explorer & SQL Editor

O desenvolvedor passará 70% do seu tempo aqui. A experiência deve ser visceralmente superior a ferramentas desktop legadas.

### 2.1 — Navegador de Schema (Sidebar Esq.)
- [ ] Fetch do endpoint `/api/v1/projects/{id}/schema` do Control Plane.
- [ ] Renderizar árvore expansível: `Tables`, `Views`, `Functions`, `Triggers`.
- [ ] Indicadores visuais: Ícone especial para `Computed Columns`, ícone de cadeado para tabelas com RLS ativo.
- [ ] Suporte a busca em tempo real na árvore de schema.

### 2.2 — Table Data Grid (Visualização e CRUD)
- [ ] Componente `DataGrid` de altíssima performance (Virtualização de linhas/colunas para não engasgar o DOM com 10.000 registros).
- [ ] Inline Editing: duplo clique numa célula salva o dado via PATCH automático.
- [ ] Botão "New Row" abrindo painel à direita (Drawer) ou inserção direta no último row da tabela.
- [ ] Suporte a FK (Foreign Key) Lookups: Ao editar uma coluna que é FK, exibir dropdown com registros da tabela estrangeira.
- [ ] Menu de "Recycle Bin" (Soft Delete definido na Fase 0) para restaurar linhas expurgadas dinamicamente.

### 2.3 — AI SQL Editor & Query Performance
**Dependências:** Roadmap Etapa 6.1 e 1.7.
- [ ] Incorporar editor Monaco (VSCode-like) para SQL com syntax highlighting.
- [ ] Auto-complete puxando do schema context (nomes de tabela e colunas).
- [ ] Console de Resultados (Table View + JSON View).
- [ ] **EXPLAIN ANALYZE Visual:** Renderização de gráfico de árvore com o custo (cost) dos nós de execução do YugabyteDB.

---

## 3. As Torres de Segurança: Configurações de Auth & Cedar ABAC

A segurança precisa ser visualizada para ser verificada.

### 3.1 — Gestão de Provedores de Auth
- [ ] UI tipo "Switches" para habilitar/desabilitar: Email/Password, Passkeys, Google, GitHub.
- [ ] Formulários seguros (senha ofuscada + salvamento) para keys OAuth gerando payload para o OpenBao.
- [ ] UI de personalização de templates de Email (SMTP settings provisionados na Fase 0).

### 3.2 — Visual ABAC Designer (O Mapa Tático)
**Dependências:** Roadmap Etapa 6.2 (RLS Designer Visual).
- [ ] Renderizador visual de Políticas Cedar.
- [ ] Modal de "Nova Política": Dropdowns fáceis traduzindo a lógica ("SE role = X E resource = Y ENTÃO PERMITIR").
- [ ] Visualização em Grafo (Nodes e Edges) mapeando `Actor -> Action -> Resource`.
- [ ] **Simulador de Acesso:** Um painel onde o operador seleciona um "Usuário Mock" e tenta dar um `GET` no recurso. A UI exibe exatamente em qual linha do arquivo Cedar a request foi bloqueada ou permitida.

---

## 4. O Cofre: Storage Router e Gestão de Arquivos

Arquivos não ficam no banco, mas a gestão deles sim. A interface precisa mostrar a magia do Pingora trabalhando.

### 4.1 — Storage Buckets Management
- [ ] Tela de gerenciamento de setores lógicos (Visual, Audio, Docs, Exec).
- [ ] UI de Provider Routing (Storage Governance): Selecionar no painel Dropdown que vídeos vão para AWS S3 e imagens para o Cloudinary.
- [ ] Toggle nativo: `"Forçar validação de Magic Bytes e bloquear incompatibilidade (Sempre Ativo)"` (Visual, inalterável pelos tenants, reforçando o Req-3.4.5).

### 4.2 — File Browser Explorer
- [ ] UI estilo Finder/Explorer corporativo para navegar nos arquivos enviados.
- [ ] Upload Drag & Drop com progress bar (consumindo APIs do Pingora).
- [ ] Visualizador integrado de mídias (Preview de imagem, player de vídeo nativo).

---

## 5. A Sala das Máquinas: Observabilidade, Logs e Crons

Métricas de bilhões de linhas processadas pelo ClickHouse devem reinar aqui em WebGL.

### 5.1 — Dashboard Analítico Principal (Telemetry)
**Dependências:** Vector.dev + ClickHouse + VictoriaMetrics da Fase 0.
- [ ] Painel principal renderizando gráficos WebGL (Canvas/ECharts) com 60FPS+.
- [ ] Gráficos: `Latência p99 (Roteada pelo Pingora)`, `Conexões Ativas (YSQL CM / pgcat)`, `Consumo de Memória Shelter`.
- [ ] Seção de "Logs em Tempo Real": Terminal streaming dos últimos logs formatados via ClickHouse.

### 5.2 — Extensions Marketplace
**Dependências:** Fase 0 módulo Extensions (PR-8).
- [ ] Galeria de Extensões do YugabyteDB (Cards com ícone, nome e "Ativar/Desativar").
- [ ] Botão `+ Enable` que dispara a API do Control Plane para executar o `CREATE EXTENSION`.

### 5.3 — Cron Jobs Scheduler UI
**Dependências:** Módulo Anti-SPOF de Cron Jobs do CP.
- [ ] Tela de Cron Jobs: Tabela listando jobs agendados, com status (Running, Failed, Active).
- [ ] Wizard "New Cron": Componente visual de agendamento (Ex: `"Todo dia às 15h"`) convertendo em expressão CRON formatada.
- [ ] Botão de "Run Now" (Disparo manual) e histórico das últimas execuções lendo da DAG do Control Plane.

---

## Orientações de Desenvolvimento (Leis da Parede de Vidro)

1. **Estado Central, UI Desacoplada:** O Dashboard é estúpido em relação a banco de dados. Ele nunca executa SQL direto para mutar schemas. Se o botão "Apagar Tabela" for clicado, ele submete um `DELETE /api/v1/projects/{P}/schema/tables/{t}` para o Control Plane, e aguarda o HTTP 200 para remover do DOM.
2. **Reatividade Cirúrgica:** No Svelte, use stores derivativos. Quando a árvore de schema atualiza, todo componente atrelado àquela store deve re-renderizar sozinhos, sem `reload()`.
3. **Resiliência a Quedas:** A UI deve estar preparada para timeouts do Pingora ou do CP. Se o Control Plane estiver efetuando reboot ou reinstalando Crons (SPOF Reconciler), a UI exibe loading states não bloqueantes (`Skeleton Loaders`), evitando pânico do usuário.

---

> **Checklist Final do Documento:** Criado com base no SRS e SAD para garantir isolamento e integridade total. Próximo passo é iniciar as rotas do dashboard no SvelteKit.
