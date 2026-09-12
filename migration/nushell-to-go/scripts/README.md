# Scripts transitórios

Não colocar lógica de negócio permanente aqui.

Único script transitório planejado: exportação explícita do índice legado `index.nuon` para o schema JSON da versão Go durante o cutover.

Requisitos desse script:

- receber `--source` e `--dest` explicitamente;
- nunca assumir `~/.ai-profiles` em testes;
- recusar sobrescrever destino sem flag explícita;
- validar estrutura antes de salvar;
- não apagar/modificar o NUON de origem;
- ser coberto por fixture sintética em `testdata/`;
- ser removível numa versão futura quando a janela de migração acabar.
