# repo-zip

Módulo Nushell para criar snapshots ZIP de repositórios Git sem incluir arquivos ignorados pelo Git.

## Estrutura

```text
modules/repo_zip/
├── mod.nu       # API pública: repo-zip
├── git.nu       # leitura/validação do repositório Git
├── archive.nu   # criação e verificação do ZIP
├── naming.nu    # nomes, sufixos e paths
├── common.nu    # erros/helpers compartilhados
└── README.md
```

Somente `repo-zip` é exportado pelo `mod.nu`; os demais arquivos são implementação interna.

## Uso

```nu
repo-zip
repo-zip .
repo-zip minha-pasta -o aqui.zip
repo-zip . --name snapshot
repo-zip . -v v1.2.0
repo-zip . --git
repo-zip . --git -v v2
repo-zip . --git -o aqui.zip
```

### Saída padrão

```text
<repo>/.tmp/unzip/<nome-do-repo>.zip
```

### `-o` / `--output`

Define exatamente o destino, relativo ao diretório atual quando for um path relativo:

```nu
repo-zip minha-pasta -o aqui.zip
```

Gera `./aqui.zip`.

Se o destino já existir, o comando aborta. Para substituí-lo explicitamente:

```nu
repo-zip . -o aqui.zip --force
```

Mesmo com `--force`, um destino tracked pelo Git ou dentro de `.git` é recusado.

### `--git`

Inclui o diretório `.git` no ZIP. No nome automático, adiciona o hash curto do `HEAD`:

```nu
repo-zip . --git
```

Exemplo:

```text
.tmp/unzip/meu-repo-a83c91e201bf.zip
```

Com mudanças não commitadas:

```text
.tmp/unzip/meu-repo-a83c91e201bf-dirty.zip
```

Com `-o`, o nome informado é respeitado exatamente:

```nu
repo-zip . --git -o backup.zip
```

### Versão / sufixo

```nu
repo-zip . -v v1.2.0
repo-zip . --version v1.2.0
repo-zip . -s backup
repo-zip . --suffix backup
```

`--version` e `--suffix` são alternativas; não podem ser usados juntos.

Com `--git`:

```nu
repo-zip . --git -v v2
```

pode gerar:

```text
meu-repo-a83c91e201bf-dirty-v2.zip
```

## O que entra

A lista normal vem de:

```text
git ls-files --cached --others --exclude-standard
```

Portanto respeita `.gitignore`, `.git/info/exclude`, excludes globais, `.gitignore` aninhados, `!` e `**` usando o próprio Git.

Arquivos tracked continuam entrando mesmo que atualmente correspondam a uma regra de ignore, que é o comportamento normal do Git.

`.tmp/unzip/` nunca é incluído no próprio snapshot.

## Proteções

- não remove nem modifica arquivos do repositório;
- o único `rm` possível é do ZIP temporário criado pela própria execução;
- cria o ZIP temporário primeiro e só publica depois de `unzip -t` passar;
- não sobrescreve ZIP existente sem `--force`;
- não sobrescreve arquivo tracked, mesmo com `--force`;
- recusa destino dentro de `.git`;
- recusa diretório/symlink como destino final;
- preserva symlinks no ZIP (`zip -y`);
- recusa submodules por enquanto, evitando snapshot parcial silencioso;
- recusa sparse checkout/`skip-worktree`, evitando omitir tracked files ausentes;
- `--git` recusa worktrees com `.git` como arquivo;
- `--git` recusa object alternates externos;
- `--git` verifica locks comuns do Git;
- `--git` compara HEAD/status antes e depois e descarta o temporário se o estado mudar durante a criação.

## Plataforma

Atualmente marcado como suportado apenas no macOS. Linux e Windows retornam erro explícito até serem implementados/testados.

Depende de:

- `git`
- `zip`
- `unzip`
- `../platform/mod.nu` do seu config Nushell

## Importação

Mantendo a pasta em `modules/repo_zip/`:

```nu
use modules/repo_zip/mod.nu *
```
