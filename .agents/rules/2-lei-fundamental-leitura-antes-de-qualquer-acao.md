---
trigger: always_on
---


2.Antes de escrever uma linha de código, corrigir um bug, ou propor uma arquitetura, você **obrigatoriamente**:

### 2.1 Leia os três arquivos de referência suprema

```
/home/cocorico/projetossz/cascataV1/Roadmap_48_Implementacoes_Cascata.md
/home/cocorico/projetossz/cascataV1/SRS_CascataV1.md
/home/cocorico/projetossz/cascataV1/SAD_CascataV1.md
```

Esses documentos são a **fonte de verdade arquitetural**. Qualquer implementação que contradiga o SRS ou SAD é uma implementação errada — independente de quão elegante seja tecnicamente.

Leia as seções relevantes para a tarefa. Não leia apenas o título da seção — leia o conteúdo completo, os requisitos numerados, e as notas de decisão. Se a tarefa envolve conexão com banco de dados, leia a seção 3.5 do SRS. Se envolve autenticação, leia a seção 2.2. Se envolve storage, leia a seção 2.6. E assim por diante.

### 2.2 Leia o código existente antes de escrever código novo

```bash
# Padrão obrigatório antes de qualquer implementação:
# 1. Mapeie a estrutura do diretório relevante
# 2. Leia os arquivos que serão modificados ou que o novo código vai interagir
# 3. Identifique padrões existentes (naming, error handling, logging, testes)
# 4. Verifique se já existe implementação parcial do que será criado
```

Nunca duplique código que já existe. Nunca crie uma abstração paralela ao que já está implementado. Encontre o padrão, siga o padrão, ou proponha explicitamente sua substituição com justificativa.

### 2.3 Cruzamento de linhas e arquivos

Para cada tarefa não trivial, seu raciocínio deve cruzar explicitamente:
- **O que o SRS exige** (requisito funcional ou não-funcional)
- **O que o SAD define** (arquitetura, camada responsável, tecnologia)
- **O que o código atual implementa** (o que já existe)
- **O que a tarefa pede** (o delta entre o atual e o desejado)

Se houver contradição entre qualquer um desses pontos, **sinalize antes de agir**.

