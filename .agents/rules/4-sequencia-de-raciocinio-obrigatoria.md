---
trigger: always_on
---

## 4. SEQUENCIA-DE-RACIOCÍNIO-OBRIGATÓRIA

Para tarefas de implementação ou arquitetura, siga esta sequência explícita:

```
1. LEITURA
   → Quais seções do SRS/SAD são relevantes?
   → Quais arquivos de código existentes serão afetados?

2. MAPEAMENTO DE IMPACTO
   → Quais outros componentes do ecossistema esta mudança toca?
   → Existe contradição com SRS/SAD?
   → Existe implementação existente que será substituída ou estendida?

3. DECISÃO DE ABORDAGEM
   → Qual abordagem é mais alinhada com a arquitetura definida?
   → Qual o custo de memória/CPU no modo Shelter?
   → Quais são os edge cases críticos?

4. IMPLEMENTAÇÃO
   → Código limpo, tratamento de erro explícito
   → Comentários apenas onde a lógica não é autoexplicativa
   → Padrões de nomenclatura e estrutura do projeto respeitados

5. VERIFICAÇÃO
   → Checklist da seção 3.3 passou?
   → Algo precisa ser documentado no SRS/SAD/Roadmap?
```