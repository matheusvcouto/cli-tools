# ADR — `repo-zip`

Decisões técnicas vigentes da CLI `repo-zip`. Decisões compartilhadas pela
suíte permanecem no [`ADR.md` da raiz](../../ADR.md).

## RZ001 — Git é a fonte de verdade da seleção — Accepted

Não reimplementar `.gitignore`, index ou status semantics. A CLI usa o Git
instalado com argv, cwd e ambiente controlados para selecionar arquivos
tracked/untracked e excluir ignored.

## RZ002 — Criação e verificação usam `archive/zip` — Accepted

O runtime não depende de `zip` ou `unzip` externos. Symlinks são armazenados
como links e nunca dereferenciados; arquivos regulares são revalidados durante
a leitura e todas as entries são verificadas antes da publicação.

## RZ003 — Snapshot detecta mudança, mas não promete transação do repositório — Accepted

O conjunto elegível e o estado Git são capturados antes e rechecados depois da
criação. Mudança observada aborta a publicação. Isso reduz snapshots híbridos,
mas não transforma o filesystem inteiro do repositório em uma transação.

## RZ004 — `--git` usa bundle verificável, não `.git` bruto — Accepted

`--git` inclui `.repo-zip/repository.bundle` e metadata mínima. O próprio Git
produz e verifica o bundle, inclusive em linked worktrees, sem copiar config,
hooks, reflogs ou outros internals de `.git`. `.repo-zip/` é namespace
reservado do archive.

## RZ005 — Publicação é confinada e específica de plataforma — Accepted

O temporário fica no mesmo diretório lógico/filesystem do destino. No-clobber
e force usam primitivas diferentes; o destino nunca é apagado para facilitar
replace. Capabilities sem garantia equivalente recebem stub/erro explícito e
não são chamadas de suportadas apenas por cross-compilar.

## RZ006 — Identidade de diretório não depende da grafia do caminho — Accepted

Git e o sistema operacional podem devolver grafias diferentes para o mesmo
diretório, como `/var` e `/private/var` no macOS. A raiz devolvida pelo Git pode
reutilizar a grafia de um ancestral real fornecido pelo chamador somente quando
`os.SameFile` comprova a identidade e o ancestral final não é symlink. Guards
de containment continuam baseados em paths relativos e APIs confinadas, nunca
em prefixo textual.

## RZ007 — `--version` é versão do produto; sufixo é `--suffix` — Accepted

O antigo uso `-v/--version TEXT` para sufixo foi removido deliberadamente.
`--version` pertence ao contrato comum do CLI Core e mostra a versão individual
do produto; nomeação de archive usa apenas `--suffix`. A quebra é registrada no
contract/change record.
