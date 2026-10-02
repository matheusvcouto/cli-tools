# Provider ativo: `grok` no `ai-profile`

Os relatórios abaixo preservam decisões e evidências das respectivas revisões.
Menções a snapshots, /v2 ou indisponibilidade de Go/rede são históricas; usar
AGENTS.md, go.mod, docs/platforms.md e CONTEXT.md para o fluxo e estado atuais.
Não gerar snapshots nem presumir que o ambiente continua sem toolchain.

1. **Decisões de contrato:** `CODE_REVIEW_003.md` revisa e supera detalhes de `IMPLEMENTATION_REVALIDATION.md` e `VALIDATION.md` do SNAPSHOT-002.
2. Leia `REPORT_AND_PLAN.md` para a investigação original e a seção final de implementação/revalidação.
3. Leia `VALIDATION.md` antes de promover qualquer gate de compatibilidade Grok real.
4. **Evidência Grok real:** execução do binário oficial, login e ACP autenticado exigem validação própria. Consultar docs/platforms.md para evidência atual dos launchers nativos; não extrapolar revisões antigas como estado do ambiente atual.
5. A suíte padrão usa probes apenas para validar o launcher (argv/env/exit status). Esses probes nunca são evidência de compatibilidade funcional com Grok.
6. Não habilitar `--always-approve`/`--yolo` automaticamente, não executar `.cmd` via shell e não copiar credenciais de `~/.grok`.
