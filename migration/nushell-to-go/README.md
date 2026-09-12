# Migração para Go — status ativo

A implementação Go já existe e **não depende de Nushell em runtime**. O código antigo permanece nesta pasta apenas como referência congelada até o cutover final.

Evidências reproduzíveis da última validação local estão em [`validation.md`](validation.md).

## Fechado

- monorepo Go e arquitetura extensível;
- `ai-profile` funcional em Go;
- `repo-zip` funcional em Go puro + Git;
- store JSON/migração transitória;
- testes sintéticos, race, vet e cross-build;
- CI/release/mise design;
- CI real verde em macOS e Linux com Go 1.27.1;
- release `v0.1.1` publicada e instalada em ambiente mise isolado;
- docs e ADRs da suíte/CLIs ativos atualizados.

## Ainda aberto antes de arquivar

1. o usuário precisa executar o cutover real/migração de dados quando decidir;
2. somente depois esta pasta vai para `docs/history/migrations/2026-09-nushell-to-go/` via `git mv`.

Não alterar contas, profiles, HOME, configuração mise ou repo real para fechar esses itens automaticamente.
