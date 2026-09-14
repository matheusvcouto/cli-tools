# Changelog

Todas as mudanças relevantes da suíte são registradas aqui. O formato é
baseado em [Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/) e as
versões seguem [Semantic Versioning](https://semver.org/lang/pt-BR/).

## [Unreleased]

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
