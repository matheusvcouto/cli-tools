# Release e mise

## Suite

Uma tag versiona todos os binários em `cmd/*`.

`tools/release`:

1. descobre diretórios imediatos de `cmd/`;
2. exige `package main`;
3. rejeita colisão por case-folding;
4. exige Go >= 1.27.1;
5. builda todos os binários para cada target publicado;
6. cria archive com `bin/`;
7. gera `SHA256SUMS`.

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

A própria workflow de tag executa testes nativos em Ubuntu e macOS antes de `publish`. Depois de gerar os artifacts, extrai o Linux amd64, percorre dinamicamente todos os executáveis em `bin/*`, exige que cada um responda `--version` com a versão da tag e valida `SHA256SUMS`.

## mise

Instalação:

```sh
mise use -g github:matheusvcouto/cli-tools@latest
```

Os assets usam `macos` no nome (embora o GOOS de build seja `darwin`) para tornar a autodetecção do backend GitHub do mise explícita. Cada archive contém `bin/`, que o mise procura automaticamente quando `bin_path` não é configurado.

Testes futuros de instalação mise devem redirecionar HOME e todos os diretórios `MISE_*` para temp; nunca tocar configuração global real.


Exemplo de gate isolado após existir uma release real:

```sh
tmp="$(mktemp -d)"
HOME="$tmp/home" \
MISE_DATA_DIR="$tmp/data" \
MISE_CACHE_DIR="$tmp/cache" \
MISE_STATE_DIR="$tmp/state" \
MISE_CONFIG_DIR="$tmp/config" \
MISE_GLOBAL_CONFIG_FILE="$tmp/config/global.toml" \
MISE_TMP_DIR="$tmp/tmp" \
  mise use -g github:matheusvcouto/cli-tools@<versão>

HOME="$tmp/home" MISE_DATA_DIR="$tmp/data" MISE_CACHE_DIR="$tmp/cache" \
MISE_STATE_DIR="$tmp/state" MISE_CONFIG_DIR="$tmp/config" \
MISE_GLOBAL_CONFIG_FILE="$tmp/config/global.toml" MISE_TMP_DIR="$tmp/tmp" \
  mise exec -- ai-profile --version

HOME="$tmp/home" MISE_DATA_DIR="$tmp/data" MISE_CACHE_DIR="$tmp/cache" \
MISE_STATE_DIR="$tmp/state" MISE_CONFIG_DIR="$tmp/config" \
MISE_GLOBAL_CONFIG_FILE="$tmp/config/global.toml" MISE_TMP_DIR="$tmp/tmp" \
  mise exec -- repo-zip --version
```

O diretório temporário pode ser removido ao final. Esse procedimento não deve ler nem alterar a configuração mise real do usuário.

## Proveniência

Em repositório público, o workflow de release gera GitHub Artifact Attestation/SLSA provenance para todos os archives listados em `SHA256SUMS`. Em repositório privado comum essa etapa é ignorada para não depender de um recurso que pode exigir Enterprise Cloud. Checksums continuam obrigatórios em todos os casos.
