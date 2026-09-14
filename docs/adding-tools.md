# Adicionando uma CLI

## 1. Criar o produto

Crie `cmd/<tool>/main.go` e `cmd/<tool>/tool.json`. O manifest usa schema v1 e SemVer sem prefixo `v`:

```json
{
  "schema_version": 1,
  "name": "my-tool",
  "version": "0.1.0",
  "stability": "beta"
}
```

O entrypoint deve embutir/validar o manifest, chamar `cli.SignalContext`, compor dependências lazy e passar stdio/terminal explicitamente ao core.

## 2. Manter domínio e CLI separados

Regra de negócio fica em `internal/<domain>/`. A composição da superfície fica em `internal/<domain>/cli/` e retorna um `*cli.CompiledApp`.

Declare uma única `cli.App` com stable IDs para comandos, argumentos e flags. Não escreva parser, help, version dispatch ou scripts de completion paralelos.

## 3. Modelar o contrato

Use:

- codecs tipados em vez de parse dentro do handler;
- `ArgOpaque` para argv que pertence a processo filho;
- constraints para relações entre opções;
- `OptionPolicy` explícita quando a ordem flags/argumentos fizer parte do contrato;
- providers ordenados para resolução `CLI > env/config > default`, preservando provenance;
- `Sensitive` para valores que nunca podem aparecer em completion/schema/defaults/diagnostics;
- `Capability` para disponibilidade conhecida sem I/O;
- `Requirement` para probes seguros de runtime;
- `Completer` somente para candidatos dinâmicos side-effect-free;
- `Interaction` para prompts; não leia stdin diretamente em handlers interativos.

Nomes visíveis não substituem stable IDs. Não reutilize um ID para outra semântica.

## 4. Gerar e travar o contrato

```sh
go run ./cmd/<tool> __cli contract > cmd/<tool>/cli.contract.json
```

Adicione teste que compare o lock byte a byte e prove que sua geração não inicializa dependências desnecessárias.

Schema/documentação podem ser inspecionados por:

```sh
go run ./cmd/<tool> __cli schema
go run ./cmd/<tool> --help
```

## 5. Completion

Todo tool com `Builtins.Completion` herda:

```text
<tool> completion list
<tool> completion generate <shell>
<tool> completion install [shell]
<tool> completion uninstall [shell]
<tool> completion status [shell]
<tool> completion doctor [shell]
```

Adapters suportados: Fish, Nushell, Bash, Zsh e PowerShell. Não crie scripts shell específicos dentro do domínio. O adapter Nushell requer Nushell 0.114 ou superior: usa `@complete` para delegar ao planner e `commandline complete` para mesclar diretivas nativas de path/diretório sem duplicar a árvore da CLI. `completion doctor nushell` detecta uma instalação local abaixo desse piso sem transformar Nushell em dependência do runtime. Omitir `shell` em `install`/`uninstall` faz preflight de todos os adapters antes de qualquer mutação.

## 6. Registrar impacto

Qualquer mudança relevante adiciona/ajusta `changes/*.json` para `module` e/ou o produto afetado. Em PR, CI valida cobertura dos caminhos alterados.

Não edite versão do produto a cada commit. O bump é preparado por `tools/release prepare` conforme `docs/release.md`.

## 7. Testar

No mínimo:

```sh
gofmt -w <arquivos>
go test ./internal/<domain>/... ./cmd/<tool>
go test ./cli
```

Antes de considerar o trabalho pronto, use os gates herméticos de `docs/testing.md`. Testes de completion install/uninstall sempre usam HOME/XDG sintéticos.

## Pacote público novo

`cli/` é a exceção pública arquitetural já aceita. Outro package fora de `internal/` exige consumidor real, API mínima e decisão no ADR porque passa a carregar custo de compatibilidade.
