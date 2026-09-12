# CLI Tools

Coleção pessoal de ferramentas de linha de comando, distribuídas como
binários standalone e criadas para uso pessoal.

## Ferramentas

- [`ai-profile`](cmd/ai-profile/README.md) — gerencia perfis isolados para Claude e Codex.
- [`repo-zip`](cmd/repo-zip/README.md) — cria snapshots ZIP de repositórios Git.

## Instalação

As releases podem ser instaladas globalmente com o [mise](https://mise.jdx.dev/):

```sh
mise use -g github:matheusvcouto/cli-tools
```

Para instalar uma versão específica:

```sh
mise use -g github:matheusvcouto/cli-tools@1.0.0
```

A instalação disponibiliza os comandos:

```text
ai-profile
repo-zip
```

## Plataformas

- macOS: suportado;
- Linux: suportado;
- Windows: compilação disponível, mas suporte de runtime ainda está em desenvolvimento.

Cross-build comprova compilação, não suporte de runtime.

## Desenvolvimento

```sh
mise run check
```

Documentação técnica:

- [Arquitetura](docs/architecture.md)
- [Testes](docs/testing.md)
- [Plataformas](docs/platforms.md)
- [Engenharia](docs/engineering.md)
- [Decisões arquiteturais da suíte](ADR.md)
- [`ai-profile` ADR](cmd/ai-profile/ADR.md)
- [`repo-zip` ADR](cmd/repo-zip/ADR.md)

## Sobre o projeto

Este é um projeto pessoal, criado para automatizar fluxos específicos do
autor. Não é uma distribuição oficial do Claude, Codex ou Git.
