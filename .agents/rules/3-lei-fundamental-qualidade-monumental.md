---
trigger: always_on
---


## 3. LEI FUNDAMENTAL — QUALIDADE MONUMENTAL

### 3.1 O padrão de referência

O Cascata é um orquestrador que opera do desenvolvedor solo ao banco tier-1, **com o mesmo código**. Isso impõe que cada componente seja:

- **Correto** — funciona conforme o SRS define, sem edge cases não tratados
- **Seguro** — segue as decisões de segurança do SAD (mTLS, Cedar ABAC, RLS Handshake, magic bytes, timing attack protection)
- **Observável** — emite eventos para o Redpanda, métricas para VictoriaMetrics, logs para ClickHouse conforme o pipeline de observabilidade do SAD
- **Resiliente** — trata falhas explicitamente, não silencia erros, não deixa goroutines/async tasks órfãs
- **Eficiente em memória** — o modo Shelter ($20/mês VPS) é um requisito de design, não uma aspiração

### 3.2 O que não é aceitável

- Implementações `TODO: fix later` em código de caminho crítico
- `panic()` / `throw` sem recovery estruturado em serviços de longa duração
- Conexões de banco sem pool ou sem timeout
- Secrets hardcoded ou em variáveis de ambiente sem referência ao OpenBao
- Logs com dados sensíveis (PII, tokens, chaves)
- Queries SQL sem limite explícito em endpoints públicos
- Erros do PostgreSQL/YugabyteDB expostos diretamente ao cliente — **sempre** traduzidos para o formato `CascataValidationError`
- Race conditions em operações concorrentes sobre estado compartilhado
- Implementações que funcionam no tier STANDARD mas quebram no modo Shelter

### 3.3 Checklist antes de entregar qualquer implementação

```
□ Está alinhado com o SRS (seção relevante verificada)?
□ Está alinhado com o SAD (camada correta, tecnologia correta)?
□ Segue os padrões de código existentes no projeto?
□ Trata todos os casos de erro explicitamente?
□ Emite eventos/métricas/logs conforme o pipeline de observabilidade?
□ Funciona no modo Shelter (memória, recursos mínimos)?
□ Não expõe dados sensíveis em nenhum output?
□ Não cria efeito colateral em outros componentes do ecossistema?
```

Se qualquer item está marcado com dúvida, sinalize — não entregue com incerteza silenciosa.
