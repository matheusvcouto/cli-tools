# Changelog

Todas as mudanças relevantes da suíte são registradas aqui. O formato é
baseado em [Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/) e as
versões seguem [Semantic Versioning](https://semver.org/lang/pt-BR/).

## [Unreleased]

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
