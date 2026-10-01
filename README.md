# CLI Tools

Coleção pessoal de ferramentas de linha de comando, distribuídas como
binários standalone e criadas para uso pessoal.

## Ferramentas

- [`ai-profile`](cmd/ai-profile/README.md) — gerencia perfis isolados para Claude, Codex e Grok Build.
- [`repo-zip`](cmd/repo-zip/README.md) — cria snapshots ZIP de repositórios Git.
- [`media-get`](cmd/media-get/README.md) — baixa vídeo, áudio e legendas com yt-dlp/FFmpeg do sistema (experimental).

## Instalação

As releases podem ser instaladas globalmente com o [mise](https://mise.jdx.dev/):

```sh
mise use -g github:matheusvcouto/cli-tools@latest
```

Para instalar uma versão específica:

```sh
mise use -g github:matheusvcouto/cli-tools@0.1.1
```

Isso grava no arquivo global do mise uma entrada equivalente a:

```toml
[tools]
"github:matheusvcouto/cli-tools" = "latest"
```

Para atualizar uma instalação configurada como `latest`:

```sh
mise upgrade github:matheusvcouto/cli-tools
```

A instalação disponibiliza os comandos:

```text
ai-profile
repo-zip
media-get
```

`media-get` entra na distribuição a partir da suíte `v1.3.0`.

## Plataformas

- macOS: suportado;
- Linux: suportado;
- Windows x64/ARM64: implementação de runtime e geração de artifacts configuradas; promoção/publicação é condicionada aos gates nativos Windows do CI.

Cross-build comprova compilação; a evidência de runtime continua sendo o CI nativo.

## Desenvolvimento

As CLIs usam o `cli/` Core declarativo comum: parsing tipado, help, completion, schema, contracts e documentação são derivados do mesmo grafo compilado.

```sh
./scripts/check-safe.sh all
```

`mise run check` continua disponível como atalho interativo.

Documentação técnica:

- [Arquitetura e CLI Core](docs/architecture.md)
- [Adicionando ferramentas](docs/adding-tools.md)
- [Testes](docs/testing.md)
- [Plataformas](docs/platforms.md)
- [Portabilidade de paths e testes](docs/portability.md)
- [Release, changelog e mise](docs/release.md)
- [Histórico de mudanças](CHANGELOG.md)
- [Engenharia](docs/engineering.md)
- [API Go pública e lock de compatibilidade](docs/cli-api.md)
- [Decisões arquiteturais da suíte](ADR.md)
- [`ai-profile` ADR](cmd/ai-profile/ADR.md)
- [`repo-zip` ADR](cmd/repo-zip/ADR.md)

## Sobre o projeto

Este é um projeto pessoal, criado para automatizar fluxos específicos do
autor. Não é uma distribuição oficial do Claude, Codex ou Git.

**Roadmap (não implementado):** View Limits por provider/perfil, com fontes oficiais e separação entre assinatura, limites de API e uso local. Ver [`plans/view-limits/README.md`](plans/view-limits/README.md).
