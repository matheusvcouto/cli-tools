# repo-zip

Cria snapshots ZIP de repositórios Git.

Decisões técnicas: [`ADR.md`](ADR.md).

## Uso

Criar um snapshot do repositório atual:

```sh
repo-zip .
```

Incluir um bundle verificável do histórico Git:

```sh
repo-zip . --git
```

Definir nome e sufixo:

```sh
repo-zip . --name snapshot
repo-zip . --suffix v1.0.0
```

Definir o caminho de saída:

```sh
repo-zip . --output /path/to/snapshot.zip
```

Veja todas as opções com:

```sh
repo-zip --help
```

## Snapshot Git

Com `--git`, o archive inclui:

```text
.repo-zip/
├── repository.bundle
└── metadata.json
```

O `.git` bruto não é copiado. O bundle pode ser validado ou usado para
criar um clone:

```sh
git bundle verify .repo-zip/repository.bundle
git clone .repo-zip/repository.bundle restored-repo
```

O estado sujo e os arquivos não rastreados do momento do snapshot continuam
presentes na raiz extraída, mas não são aplicados automaticamente ao clone.

## Saída padrão

Sem `--output`, o archive é criado em:

```text
<repo>/.tmp/repo-zip/<nome>.zip
```

O programa cria e verifica o ZIP antes de publicá-lo e evita sobrescrever
conteúdo rastreado pelo Git.
