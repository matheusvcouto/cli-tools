# `ai-profile` — gate funcional

O objetivo não é copiar a aparência antiga; é preservar capacidades e dados relevantes com uma implementação melhor em Go.

| Capacidade | Estado Go | Evidência local |
|---|---|---|
| Claude profiles | implementado | tests |
| Codex profiles | implementado | tests |
| `agy` excluído | preservado | registry |
| `list` | implementado | tests |
| machine output | `list --json` | tests |
| `new` | implementado | rollback tests |
| `rename` alias-only | implementado | tests |
| `delete` seguro | quarentena + rollback | tests |
| `run` | Unix `exec` | runtime synthetic test |
| `acp` | Unix `exec`, stdout limpo | runtime synthetic test |
| env isolation | roots nativos + auth/provider/workload identity | tests + E2E |
| passthrough argv | implementado | tests |
| Codex profile-native AGENTS | `CODEX_HOME/AGENTS.override.md` ou `AGENTS.md`; sem herança do default | tests + E2E |
| statusline Claude-only | built-in + custom dir | tests |
| shell completion | Fish/Nushell 0.114+/Bash/Zsh/PowerShell | generation + native conditional tests |
| schema legado | conversor transitório | tests |
| Windows | parcial/compile-only | cross-build |

## Dados que a migração deve preservar

- `tool`;
- alias;
- diretório físico existente;
- `created_at` exatamente como armazenado no legado.

Rename nunca move o diretório físico.

## Mudanças deliberadas

- mensagens/help/tabelas são UX nova;
- timestamps novos usam RFC3339 UTC;
- store final é JSON versionado;
- o runtime não depende de Nushell; a completion Nushell opcional requer Nu 0.114+;
- implementação Go pode ser chamada por qualquer shell/processo;
- context discovery nativo é preservado via cwd; profiles não copiam guidance do profile default;
- diretório físico novo continua opaco, sem obrigação de reproduzir formato histórico do ID.

## Gate restante

A implementação Unix está testada em Linux e cross-compila para Darwin. O cutover principal em macOS exige CI/runtime macOS verde antes de esta migração ser arquivada.
