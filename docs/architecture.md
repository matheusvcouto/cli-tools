# Arquitetura

## Forma geral

```text
cmd/<tool>/main.go
      ↓
internal/<tool>/cli/      # composição declarativa da superfície pública
      ↓
cli.Compile(App)          # grafo privado, validado e imutável
      ↓
parser/runtime/help/completion/schema/contracts/docs
      ↓
internal/<tool>/...       # casos de uso e invariantes de domínio
      ↓
stdlib + capabilities específicas quando a semântica muda por SO
```

`cli/` é o único pacote público deliberadamente compartilhado. O domínio não conhece shell, parser ou renderer; adapters de shell não conhecem regra de negócio.

## CLI Core

Cada executável declara uma única `cli.App` composta por `Command`, `Arg`, `Flag`, codecs tipados, constraints, capabilities e requirements. `cli.Compile` rejeita Specs ambíguas antes da execução e constrói uma vez os lookups usados por:

- strict parse e partial parse pela mesma máquina de estados;
- binding tipado, diagnostics e exit classes;
- help, versão e doctor como comandos gerados no próprio grafo;
- runtime, middleware, interaction e preflight;
- completion shell-neutral;
- Fish, Nushell, Bash, Zsh e PowerShell;
- schema v1, contract lock v1, Markdown e man page.

Comandos/flags/args possuem **stable IDs** distintos do texto visível. O namespace `__cli` é reservado a endpoints de máquina versionados.

## Estrutura

```text
cli/                         # API pública + implementação do CLI Core
cmd/<tool>/
├── main.go                  # composition root
├── tool.json                # versão individual do produto
└── cli.contract.json        # lock derivado do contrato público
internal/
├── aiprofile/               # domínio ai-profile
│   └── cli/                 # Spec/composição da CLI
├── repozip/                 # domínio repo-zip
│   └── cli/                 # Spec/composição da CLI
├── filelock/                # primitive de plataforma focada
├── fscommit/                # commit/replace confinado
├── safefs/                  # operações confinadas a uma raiz
└── version/                 # manifest/version metadata
changes/                     # change records pendentes
plans/                       # planos ativos
scripts/                     # gates herméticos
tools/                       # tooling interno, inclusive release
docs/                        # documentação do estado implementado
```

Não existe mais `internal/cliapp` nem árvore manual paralela de help/completion.

## Runtime e lazy initialization

O composition root passa `context.Context` e stdio explicitamente. `cli.SignalContext` é chamado explicitamente pelo entrypoint e encapsula os sinais suportados por plataforma (SIGINT e, em Unix, SIGTERM); o pacote `cli` não lê `os.Args` nem chama `os.Exit`.

Dependências de domínio podem usar `cli.Lazy[T]`. Help, versão, schema, contract e completion gerada não devem abrir HOME/store nem executar probes de domínio. Capability/requirement é verificada antes do handler.

## Completion

```text
argv + cursor
   ↓
partial parse
   ↓
completion planner neutro
   ↓
CompletionResult protocol v1
   ↓
adapter Fish/Nu/Bash/Zsh/PowerShell
```

O protocolo usa offsets em Unicode scalars, candidatos estruturados e limite de candidatos; coordenadas fora do argv/token são rejeitadas. Scripts instalados carregam somente o nome da ferramenta e o adapter do protocolo: comandos, aliases, flags, choices, disponibilidade e completers vêm sempre do planner em runtime, portanto mudar a Spec não deixa metadata shell desatualizada. Completers podem ler valores anteriores por stable ID com `CompletionValueAs`/`CompletionValuesAs`; valores `Sensitive` são omitidos. Cada adapter deriva o prefixo do mecanismo de cursor nativo do shell, inclusive completion no meio da linha. `completion install [shell]` grava somente no diretório de integração do shell; sem `shell`, faz preflight de todos os targets e aplica com rollback. Nunca edita silenciosamente `.bashrc`, `.zshrc`, `config.nu` ou profiles do PowerShell.

## Contrato e versionamento

Há três versões independentes:

1. tag `vX.Y.Z`: módulo/suíte e API pública de `cli/`;
2. `cmd/<tool>/tool.json`: produto individual;
3. inteiros de schema/contract/completion protocol.

`cmd/<tool>/cli.contract.json` é gerado do mesmo grafo e testado byte a byte. Mudanças recebem registros em `changes/*.json`; `tools/release prepare` calcula os bumps e materializa a release.

## Compartilhamento

Extraia código apenas quando existe a mesma semântica/invariante. Não criar `utils`, `helpers`, `common` ou `shared` genéricos. Plataforma fica em packages focados; não criar um `cli/platform` que absorva invariantes de filesystem/processo do domínio.
