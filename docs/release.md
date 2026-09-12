# Release e mise

## Suite

Uma tag versiona todos os binários em `cmd/*`. Releases estáveis usam Semantic
Versioning no formato exato `vX.Y.Z`; o prefixo `v` é aceito nativamente pelo
backend GitHub do mise.

Não há arquivo de versão para editar. Um push de tag no formato `vX.Y.Z`
dispara `.github/workflows/release.yml`; a tag é injetada em todos os binários
e também nomeia a GitHub Release. A primeira entrega pública instalável desta
suíte é `v0.1.1`; a tag `v0.1.0` é histórica e não possui GitHub Release porque
seu smoke-test falhou antes da publicação.

## Changelog e descrição obrigatória

`CHANGELOG.md` é a fonte canônica das notas. Antes de criar `vX.Y.Z`, mover os
itens pertinentes de `Unreleased` para:

```markdown
## [X.Y.Z] - YYYY-MM-DD

### Adicionado

- Capacidade nova descrita objetivamente.

### Corrigido

- Correção relevante descrita objetivamente.
```

Use somente as categorias que fizerem sentido: `Adicionado`, `Alterado`,
`Corrigido`, `Segurança`, `Descontinuado` e `Removido`. Cada categoria presente
deve ter itens em lista. Não criar seção vazia nem depender de release notes
geradas automaticamente como histórico canônico.

O tooling recusa:

- tag fora do formato `vX.Y.Z` ou com zeros à esquerda;
- seção ausente ou duplicada;
- data fora de `YYYY-MM-DD`;
- seção sem item Markdown;
- diretório de output não vazio, arquivo ou symlink.

O workflow extrai somente a seção da versão e a publica como descrição da
GitHub Release, com título `CLI Tools vX.Y.Z`.

## Tooling

`tools/release`:

1. descobre diretórios imediatos de `cmd/`;
2. exige `package main`;
3. rejeita colisão por case-folding;
4. exige Go >= 1.27.1;
5. builda todos os binários para cada target publicado;
6. cria archive com `bin/`;
7. gera `SHA256SUMS`.

O gerador nunca limpa o diretório informado por `--out`. Ele cria um diretório
ausente com filesystem confinado ou aceita um diretório vazio; qualquer conteúdo
preexistente causa erro sem ser alterado.

`SHA256SUMS` contém nomes portáveis relativos ao diretório em que o manifesto
fica. Ferramentas como `sha256sum -c` resolvem esses nomes contra o cwd, não
contra o path do manifesto. Portanto a verificação correta é:

```sh
(cd dist && sha256sum -c SHA256SUMS)
```

Executar `sha256sum -c dist/SHA256SUMS` a partir do diretório pai procura os
assets no lugar errado e falha mesmo quando os hashes e arquivos estão corretos.

## Targets publicados

```text
macOS (GOOS=darwin)/amd64
macOS (GOOS=darwin)/arm64
linux/amd64
linux/arm64
```

Windows é compile-only até implementação + runtime tests das capabilities nativas restantes.

## Artifact

```text
cli-tools_<version>_<os>_<arch>.tar.gz
└── bin/
    ├── ai-profile
    └── repo-zip
```

## Gate de release

A própria workflow de tag executa testes nativos em Ubuntu e macOS antes de
`publish`. Depois de gerar os artifacts, extrai o Linux amd64, percorre
dinamicamente todos os executáveis em `bin/*`, exige que cada um responda
`--version` com a versão da tag e valida `SHA256SUMS`.

Ordem obrigatória:

1. atualizar e revisar `CHANGELOG.md`;
2. executar os gates locais isolados;
3. commitar código, docs, changelog e workflow juntos;
4. enviar o commit e aguardar o CI de `main` ficar verde;
5. criar a tag no commit aprovado e enviá-la sem mover/reutilizar tag antiga;
6. aguardar `verify` e `publish` da workflow de release;
7. conferir título, descrição, assets, checksums e attestation;
8. instalar a release via mise em HOME/MISE_* isolados e executar `--version`
   de todos os binários.

## mise

Instalação:

```sh
mise use -g github:matheusvcouto/cli-tools@latest
```

Versão específica:

```sh
mise use -g github:matheusvcouto/cli-tools@0.1.1
```

Configuração equivalente:

```toml
[tools]
"github:matheusvcouto/cli-tools" = "latest"
```

Atualização quando a configuração usa `latest`:

```sh
mise upgrade github:matheusvcouto/cli-tools
```

Os assets usam `macos` no nome (embora o GOOS de build seja `darwin`) e
`linux`, combinados com `amd64` ou `arm64`, para a autodetecção do backend
GitHub do mise. Cada archive contém `bin/`, que o mise procura automaticamente
quando `bin_path` não é configurado. Releases sem assets não são instaláveis
pelo backend GitHub.

Testes de instalação mise devem desabilitar discovery de configuração com
`MISE_NO_CONFIG=1`/`--no-config`, redirecionar HOME, `GH_CONFIG_DIR` e todos os
diretórios `MISE_*`, e desativar fallbacks de tokens do gh/Git. Redirecionar
somente os diretórios não é evidência suficiente de isolamento.


Exemplo de gate isolado após existir uma release real:

```sh
sandbox="<diretório-descartável-confirmado>"
mkdir -p "$sandbox"/{home,data,cache,state,config,tmp,gh}

mise_sandbox() {
  env -i \
    PATH="<diretório-do-mise>:/usr/bin:/bin" \
    HOME="$sandbox/home" USERPROFILE="$sandbox/home" \
    GH_CONFIG_DIR="$sandbox/gh" \
    MISE_NO_CONFIG=1 \
    MISE_DATA_DIR="$sandbox/data" \
    MISE_CACHE_DIR="$sandbox/cache" \
    MISE_STATE_DIR="$sandbox/state" \
    MISE_CONFIG_DIR="$sandbox/config" \
    MISE_GLOBAL_CONFIG_FILE="$sandbox/config/global.toml" \
    MISE_GLOBAL_CONFIG_ROOT="$sandbox/home" \
    MISE_SYSTEM_CONFIG_DIR="$sandbox/system-config" \
    MISE_TMP_DIR="$sandbox/tmp" \
    MISE_GITHUB_GH_CLI_TOKENS=false \
    MISE_GITHUB_USE_GIT_CREDENTIALS=false \
    "<caminho-absoluto-do-mise>" --no-config "$@"
}

mise_sandbox install github:matheusvcouto/cli-tools@<versão>
mise_sandbox exec github:matheusvcouto/cli-tools@<versão> -- \
  ai-profile --version
mise_sandbox exec github:matheusvcouto/cli-tools@<versão> -- \
  repo-zip --version
```

Listar e remover o sandbox regenerável somente depois da validação. Esse
procedimento não deve ler nem alterar a configuração mise real do usuário.

## Proveniência

Em repositório público, o workflow de release gera GitHub Artifact Attestation/SLSA provenance para todos os archives listados em `SHA256SUMS`. Em repositório privado comum essa etapa é ignorada para não depender de um recurso que pode exigir Enterprise Cloud. Checksums continuam obrigatórios em todos os casos.
