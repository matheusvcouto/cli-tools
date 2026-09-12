# AGENTS.md — migração Nushell → Go

Aplica-se somente enquanto esta migração estiver ativa.

## Antes de implementar

Leia nesta ordem:

1. `reanalysis.md`
2. `current-state-audit.md`
3. `../../docs/platforms.md`
4. contrato/paridade da CLI alvo
5. `plan.md`
6. `tasks.md`
7. `acceptance.md`

## Regras adicionais

- `reference/` é somente leitura e serve para comparação de comportamento.
- Não copiar nomes, paths, contas, credenciais ou estado real para fixtures.
- macOS e Linux usam backend Unix onde a semântica é comprovadamente a mesma; Windows permanece compile-only/unsupported nas capabilities ainda sem backend seguro.
- Plataforma sem backend seguro deve continuar explicitamente `untested`/`unsupported`.
- Não mover/remover o código Nushell original antes dos gates de cutover.
- Não executar migração real de `~/.ai-profiles` durante testes.
- O legado não define UX. Diferenças de capacidade/invariante exigem contrato/ADR; melhorias de mensagens/apresentação não.
- Marque task somente com teste, diff ou outra evidência reproduzível.

## Encerramento

Quando todos os gates passarem, este arquivo é movido junto com a pasta inteira para `docs/history/migrations/2026-09-nushell-to-go/`. Ele não permanece no caminho ativo do repo.
