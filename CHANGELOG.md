# Changelog

Todas as mudanças relevantes da suíte são registradas aqui. O formato é
baseado em [Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/) e as
versões seguem [Semantic Versioning](https://semver.org/lang/pt-BR/).

## [Unreleased]

## [1.3.0] - 2026-10-01

### Adicionado

- **media-get:** Introduce experimental media-get with declarative CLI Core, optional Referer, video/audio/SRT selections, per-operation system dependency checks, concurrent cached transfer-size previews before selection, loading counters and staged progress, cancellable Unix process groups and confined no-clobber publication with partial-download recovery
- **module:** Allow explicitly requested forward suite-version skips within the computed major while preserving compatibility guards, independent product versions and transactional release preparation
- **module:** Include the new experimental media-get product in automatic tool discovery without changing the public Go CLI Core API

## [1.1.0] - 2026-09-27

### Adicionado

- **ai-profile:** Add and harden Grok Build profiles: isolated GROK_HOME, documented Codex skills/hooks defaults, inherited auth/compat override filtering that preserves administrative security restrictions, safe config identity checks, bounded profile metadata reads and writes, rejection of duplicate profile directories and orphaned profile state, safe feedback/trace routing, native ACP, and bounded fail-closed Windows npm resolution; fix invalid Claude settings causing panic
- **ai-profile:** Implement Windows locking, safe profile commits, and Job Object-contained native process execution
- **module:** Keep the v1 module path and raise the suite baseline to Go 1.27.1 with confined os.Root-only storage; build once, native-test all six published targets, and harden GitHub Actions provenance
- **repo-zip:** Implement confined Windows force and no-clobber archive publication

### Corrigido

- **ai-profile:** Prevent interactive profile deletion from removing an alias reassigned during confirmation by checking the originally displayed profile under the store lock; reject canceled launches before exec on Unix and process startup on Windows
- **ai-profile:** Reject all residual profile-root entries when index.json is missing, add validated explicit import of legacy NUON directories (including original index.nuon in the target root), bound and revalidate migration source, and preserve Windows PATH order when resolving official npm shims
- **module:** Check out text as LF so Windows CI gofmt does not treat autocrlf CRLF conversions as unformatted Go sources
- **module:** Declare the GitHub-hosted windows-11-vs2026-arm runner to actionlint 1.7.12 so workflow lint accepts the published Windows ARM64 label
- **module:** Fail closed on invalid or unprepared release tags before the six-platform native matrix; align shell tag validation with the stable Go release builder and test pending change-record rejection
- **repo-zip:** Open Windows Git bundle temp files with WRITE_DAC before applying the protected DACL

## [1.0.1] - 2026-09-14

### Corrigido

- **ai-profile:** Fix generated Nushell completions
- **module:** Fix Nushell completion cursor coordinates and validate the native completion request
- **repo-zip:** Fix generated Nushell completions

## [1.0.0] - 2026-09-14

### Adicionado

- **ai-profile:** Migrate to CLI Core with generated shell completion and introspection
- **module:** Add a compatibility lock and external-consumer tests for the reusable public Go CLI API
- **module:** Harden release preparation with rollback and explicit product stability promotion for v1

### Alterado

- **ai-profile:** Expose ACP only when the tool registry declares it and retire apply-statusline from the active ai-profile implementation while preserving the historical Nushell reference
- **module:** Introduce the public declarative CLI Core with shared parsing, completion, schema, contracts, diagnostics and runtime infrastructure
- **repo-zip:** Migrate to CLI Core and reserve --version for product version; use --suffix for archive suffixes

## [0.1.1] - 2026-09-12

### Alterado

- Esta é a primeira release com assets publicáveis; `v0.1.0` executou todos os
  gates nativos, mas parou antes da publicação por causa do smoke-test de
  checksum.

### Corrigido

- O smoke-test de release agora valida `SHA256SUMS` a partir do diretório
  `dist`, onde os nomes relativos dos assets são resolvidos corretamente.
- Um teste de política impede que o workflow volte a verificar um manifesto
  relativo a partir do diretório pai.

## [0.1.0] - 2026-09-12

### Adicionado

- `ai-profile` para criar, listar, renomear, remover e executar profiles
  isolados de Claude Code e Codex, incluindo adapters ACP.
- `repo-zip` para gerar snapshots ZIP a partir da seleção do Git, com opção
  `--git` baseada em bundle restaurável e suporte a linked worktrees.
- Binários standalone para macOS e Linux em arquiteturas AMD64 e ARM64.
- Instalação e atualização da suíte pelo backend GitHub do mise.
- Checksums SHA-256 e attestations dos archives publicados.

### Alterado

- O contexto global do Codex e do Claude passa a pertencer ao profile
  selecionado, enquanto o contexto do projeto continua sendo descoberto pelo
  diretório de trabalho.
- Decisões técnicas específicas ficam nos ADRs individuais de `ai-profile` e
  `repo-zip`; decisões compartilhadas permanecem no ADR da suíte.

### Corrigido

- Comparações de diretório no macOS agora usam identidade de filesystem quando
  `/var` e `/private/var` representam o mesmo local.
- Criação concorrente do lock inicial de profiles não falha quando dois
  processos tentam criar o mesmo alias.
- Variáveis herdadas de autenticação, provider e workload identity que poderiam
  furar o isolamento do profile são removidas antes de `run` e `acp`.

### Segurança

- Testes executam somente com HOME, temporários, caches, Git e executáveis
  sintéticos; nenhuma conta, credencial ou configuração global real é usada.
- Operações sensíveis de filesystem são confinadas, recusam symlinks perigosos
  e evitam sequências destrutivas de remove-then-rename.
- O gerador de releases nunca remove conteúdo de um diretório de saída já
  existente; diretórios não vazios são recusados.
