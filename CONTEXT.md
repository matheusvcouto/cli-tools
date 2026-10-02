# Contexto atual — revisão do fluxo Go para v1.3.2

Data: 2026-10-01. Base inicial: v1.3.0 / 14dfc95. Integração: v1.3.1 / 5ba6009. Branch: codex/go-workflow-1.3.2.
Worktree: /Users/matheus/matheusvcouto/root/__worktrees/cli-tools/go-workflow.

## Escopo e integração

Regras, preparação de dependências, caches, geração de contratos e CI.
v1.3.1 / 5ba6009 integrada nesta worktree, preservando a UX, edição de texto,
cancelamento, cleanup, prefetch e notices do media-get. Contrato/produto
media-get 0.2.1 preservados. Usuário autorizou commit, push e publicação de
v1.3.2 após gates verdes; autorização recebida nesta conversa.
Release v1.3.1 publicada; workflow 36945709150 success, seis assets e
SHA256SUMS confirmados em https://github.com/matheusvcouto/cli-tools/releases/tag/v1.3.1. Não mover/reutilizar sua tag nem contornar seus gates.

Conflitos documentais resolvidos: prevalece preparação online e cache dedicado,
sem vendor como requisito. 381 arquivos de vendor comparados byte a byte com
módulos oficiais; vendor/modules.txt corresponde às versões pinadas. Os 382
arquivos foram movidos para dist/recovery/_vendor-1.3.1, não apagados. Lista em
dist/dependency-audit/vendor-paths.txt. Notices/licenças permanecem nos archives.

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

## Validação da integração

Preparação real de x/sys v0.48.0 e x/term v0.46.0 via proxy oficial e checksum
database PASS; go mod verify PASS. Gates da árvore integrada PASS: fmt/test/vet/shuffle/race/API/contratos/changes.
Actionlint PASS; builds nativos das três CLIs e smokes isolados PASS.
Builds reconstruídos a partir da árvore integrada antes do push.
Depois observar CI nativo, preparar v1.3.2, repetir CI e enviar tag/publicação.
Histórico do agente anterior preservado em
 docs/history/go-workflow/context-v1.3.1.md; não restaura regras de snapshots.
