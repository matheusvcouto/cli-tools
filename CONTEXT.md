# Contexto atual — revisão do fluxo Go para v1.3.2

Data: 2026-10-01. Base: v1.3.0 / 14dfc95. Branch: codex/go-workflow-1.3.2.
Worktree: /Users/matheus/matheusvcouto/root/__worktrees/cli-tools/go-workflow.

## Escopo e integração

Regras, preparação de dependências, caches, geração de contratos e CI.
Outro agente altera media-get no checkout original para v1.3.1; suas mudanças
não foram copiadas nem alteradas. v1.3.2 é o destino autorizado, mas manifests,
changelog e tags só devem ser preparados após integrar v1.3.1 e revalidar.
Pontos de sobreposição prováveis: docs/testing.md, tools/release e regras de
vendoring/dependências. Preservar a funcionalidade nova de media-get na integração.
Não há autorização de commit/push/publicação nesta tarefa.

## Ambiente e regras vigentes

Go 1.27.1 disponível em /Users/matheus/.local/share/mise/installs/go/1.27.1/bin.
A instalação padrão em /usr/local/go tentou baixar outra toolchain; usar o
Go compatível já instalado no PATH, sem alterar configurações globais.
Preparação de módulos com rede é permitida. Checks reutilizam dist/go-cache
com downloads bloqueados e estado de aplicação isolado. Não gerar vendor por
limitação do ambiente nem snapshots ZIP/Base64/contextos separados de entrega.
Crashers de fuzz ficam na worktree persistente. Segurança e rollback mantidos.

## Evidência desta revisão

Preparação executada nesta base sem módulos externos. Regressão de módulos com
proxy sintético PASS: falha sem cache, preparação, duas execuções offline,
contratos com dependência preparada e manifests preservados.
scripts/check-safe.sh all PASS (fmt/test/vet/shuffle/race/API/contratos/changes).
scripts/check-safe.sh fuzz PASS nos oito alvos; nenhum crasher nesta rodada.
Actionlint oficial 1.7.12 darwin/arm64, SHA-256 upstream verificado: PASS.
Sintaxe shell e git diff --check PASS. CI remoto, runtime Linux/Windows desta
revisão e integração com v1.3.1 não executados. Não extrapolar evidência local.
Revisão adicional: CI verifica integridade dos módulos e rejeita alterações
ou criação de go.mod/go.sum durante preparação. Testes de tools/release PASS,
actionlint PASS e git diff --check PASS após o ajuste. Guard do workflow
executado em Git sintético local: aceita estado limpo e rejeita manifest
alterado/go.sum novo. CI remoto e runtime Windows continuam não executados.
Regra reforçada: manter github.com/matheusvcouto/cli-tools sem /vN. Mudança de
major/path exige pedido específico do usuário, nunca inferência de release ou
plano antigo. go.mod e imports preservados; ajuste apenas documental.
Revisão completa e comandos: docs/go-workflow.md.

O contexto anterior está preservado em docs/history/go-workflow/context-before-1.3.2.md
como histórico, sem valor normativo para snapshots ou disponibilidade atual.
