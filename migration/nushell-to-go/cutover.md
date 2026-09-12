# Cutover

Não executar automaticamente.

## Pré-condições

- CI Linux/macOS verde;
- release criada;
- release instalada com mise em sandbox e validada;
- usuário aprovou o cutover.

## Passos

1. fazer backup do estado legado sem removê-lo;
2. se existir `index.nuon`, executar o conversor transitório com `--from` e `--to-root` explícitos;
3. confirmar `index.json` e profiles com operações somente leitura;
4. instalar a suite Go no ambiente normal;
5. validar `ai-profile <tool> list` e um `run` escolhido pelo usuário;
6. validar `repo-zip` em um repositório descartável ou escolhido pelo usuário;
7. retirar imports/functions antigos do shell config;
8. manter arquivos antigos por uma janela de rollback sem auto-import.

## Rollback

- remover/desativar a entrada mise nova;
- restaurar o shell config anterior;
- manter `index.nuon` e diretórios físicos originais intactos até encerrar a janela.

O conversor não apaga `index.nuon` nem move diretórios.
