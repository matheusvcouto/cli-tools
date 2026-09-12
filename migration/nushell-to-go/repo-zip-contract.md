# `repo-zip` — contrato Go

## Seleção

Git é a fonte de verdade:

- tracked entra;
- untracked não ignorado entra;
- ignored não entra;
- output próprio é excluído;
- `.tmp/repo-zip/` é excluído.

## Guards

Recusar:

- source fora de Git repo;
- submodule/gitlink;
- sparse/skip-worktree;
- output tracked;
- output dentro de `.git`;
- destino symlink/special file;
- default output redirecionado por symlink;
- conflito de arquivo real do repositório com o namespace reservado `.repo-zip/` quando `--git`;
- `--git` sem nenhum commit alcançável, pois não há bundle Git restaurável a produzir;
- filename Git contendo `\`, porque não há representação portátil/inequívoca desse path em ZIP cross-platform.

## Archive

- `archive/zip` da stdlib;
- regular file é lido por safety root e revalidado;
- symlink é gravado como symlink, nunca como conteúdo do alvo;
- `--git` gera `.repo-zip/repository.bundle` com `git bundle create --all` e `.repo-zip/metadata.json`;
- o bundle é verificado pelo próprio Git antes de entrar no ZIP;
- `.git` bruto não é caminhado nem copiado;
- linked worktree usa exatamente o mesmo fluxo de bundle;
- entry especial do worktree é recusada;
- ZIP é verificado lendo todas as entries do mesmo descriptor temporário.

## Consistência

Antes da publicação:

- tracked guard é repetido;
- destino é repetidamente validado dentro da root aberta;
- conjunto de arquivos elegíveis é listado novamente e deve ser idêntico;
- com `--git`, token de HEAD + refs/reachable refs + status também deve continuar igual.

Isso reduz inconsistência; não é promessa de snapshot transacional contra todas as mutações simultâneas possíveis.

## Publicação Unix

- temp no mesmo parent;
- sem `--force`: hard-link do temp para o nome final; falha se destino apareceu;
- com `--force`: rename dentro da mesma safety root;
- nunca remove o destino antes de replace;
- cleanup toca apenas temp próprio.

## Plataformas

Criação/verificação do ZIP são portáteis. Apenas publicação final tem implementação específica onde necessário. Windows fica compile-only até receber semântica equivalente testada.
