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
- docs/ADR ativos atualizados.

## Ainda aberto antes de arquivar

1. GitHub Actions macOS precisa executar e ficar verde no repositório real;
2. uma release real precisa ser instalada em ambiente mise isolado e validada;
3. o usuário precisa executar o cutover real/migração de dados quando decidir;
4. somente depois esta pasta vai para `docs/history/migrations/2026-09-nushell-to-go/` via `git mv`.

Não alterar contas, profiles, HOME, configuração mise ou repo real para fechar esses itens automaticamente.
