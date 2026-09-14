# Evidências de validação

Última revisão local: 2026-09-12 (revisada após auditoria atual de isolamento/contexto de `ai-profile`).

Este arquivo registra **o que realmente foi executado**. Cross-build não é tratado como teste de runtime e nenhum item externo é inferido a partir de teste local.

## Ambiente local

```text
GOOS/GOARCH: darwin/arm64
Go: go1.27.1 (selecionado pelo mise)
Dependências de módulo externas: nenhuma
```

A toolchain oficial do projeto/release, incluindo a implementação `os.Root` de
`internal/safefs`, foi compilada e exercitada localmente. Todos os comandos Go
receberam HOME/TMP/XDG/caches e configuração Git sintéticos, com downloads e
rede do Go desabilitados. No macOS, o PATH de teste apontou diretamente para o
Apple Git em `/Library/Developer/CommandLineTools/usr/bin`, evitando que o shim
`/usr/bin/git` misturasse diagnóstico do Xcode à saída de hashes dos testes.

## Gates locais executados

Todos passaram no estado empacotado:

```sh
test -z "$(gofmt -l .)"
go test ./...
go vet ./...
go test -shuffle=on -count=3 ./...
go test -race ./...
```

`go test ./...` compilou todos os packages e testes do módulo. `go vet ./...`,
que inclui análise de tipos, terminou sem diagnóstico. Nenhuma toolchain ou
dependência foi baixada: todos os comandos usaram Go 1.27.1 já instalado, com
`GOTOOLCHAIN=local`, `GOPROXY=off`, `GOVCS=*:off` e caches sintéticos.

O E2E incluído em `integration/e2e_unix_test.go` compila e executa os binários reais com HOME, profile root, Git repo e executáveis Codex/ACP exclusivamente sintéticos.

## Fuzz smoke executado

Sessões limitadas de 5 segundos por alvo passaram sem panic/falha nesta revisão:

- `FuzzValidateAlias`: ~135k execuções;
- `FuzzValidateSuffix`: ~470k execuções;
- `FuzzVerifyZipNeverPanics`: ~97k execuções;
- `FuzzParseLegacyNUONNeverPanics`: ~511k execuções.

Os números são apenas evidência daquela execução, não meta de cobertura.

## Cross-build executado

Compile-only passou para os dois comandos em:

```text
darwin/amd64
darwin/arm64
linux/amd64
linux/arm64
windows/amd64
windows/arm64
```

Total: 12 binários compilados.

Isso comprova fronteiras de build, **não** suporte de runtime Windows/macOS.

## Revisões adicionais

- `go list -m all` contém somente `github.com/matheusvcouto/cli-tools`;
- sem `TODO`, `FIXME`, shell intermediário ou secrets reais encontrados na varredura final;
- workflows YAML parseiam corretamente;
- `mise.toml` foi aceito pelo `mise`, que selecionou Go 1.27.1; nenhuma instalação de toolchain ou pacote foi feita nessa revisão histórica;
- `actions/checkout@v7`, `actions/setup-go@v7` e `actions/attest@v4` foram conferidos contra as versões oficiais atuais;
- API `os.Root` usada por `safefs` foi conferida contra a documentação oficial atual;
- release e CI descobrem `cmd/*` em vez de manter lista de CLIs distribuídas;
- `repo-zip .` foi testado com o próprio repositório como cwd, evitando regressão no output automático;
- `ai-profile --version`, `--help` e `completion generate <shell>` foram testados sem HOME/store;
- criação de diretórios sensíveis usa `safefs.EnsureDir`, com teste provando que parent symlink não cria conteúdo fora;
- `repo-zip --git` foi migrado de cópia bruta de `.git` para `git bundle`; linked worktree real passou em teste de pacote e E2E do binário compilado;
- o bundle gerado é verificado com `git bundle verify` e o teste de worktree também prova clone/restauração do HEAD;
- mudanças em refs não-HEAD durante o snapshot abortam a publicação;
- `.gitignore` continua sendo respeitado em linked worktree com `--git`;
- o E2E compila `repo-zip`, cria uma linked worktree sintética e valida o bundle gerado pelo binário real;
- alias de caminho para a mesma raiz, incluindo a classe `/var` versus `/private/var` do macOS, é coberto por regressão sintética com identidade comprovada por `os.SameFile`;
- `.repo-zip/` é reservado no archive para impedir colisão entre arquivos do usuário e metadata interna;
- repositório sem commit recebe erro explícito em `--git`, pois Git não produz bundle restaurável vazio;
- release publica apenas macOS/Linux; Windows continua compile-only e `repo-zip` falha antes de efeitos colaterais de output.
- `ai-profile` foi re-auditado contra a documentação atual de Claude Code e do Codex: Claude recebe `CLAUDE_CONFIG_DIR` e `ANTHROPIC_CONFIG_DIR`; Codex recebe `CODEX_HOME`; parent-shell auth/provider/workload-identity overrides conhecidos são removidos; Codex não herda `~/.codex/AGENTS*`; cwd do projeto é preservado; Claude profiles mantêm seu `CLAUDE.md` próprio e recebem defesa em profundidade contra contexto do `~/.claude` default;
- E2E do `ai-profile` prova `run` e `acp` para Codex/Claude com profile-global guidance própria, project guidance no cwd, argv/exit status, roots esperados e ausência dos redirects de autenticação testados;
- o teste unitário rejeita `.anthropic` como symlink antes do spawn e prova que o alvo externo sintético permanece inalterado.

## Gates externos ainda abertos

Não marcar como concluídos sem evidência real:

1. cutover real autorizado pelo usuário;
2. só então mover esta migração para `docs/history/migrations/2026-09-nushell-to-go/`.

## Evidência externa concluída

- GitHub Actions run `34701668236`: PASS em macOS e Ubuntu com Go 1.27.1,
  incluindo format, `go test`, `go vet`, shuffle ×3 e race; cross-build também
  passou. A regressão `/var` versus `/private/var` foi executada no runner
  macOS que originalmente revelou a comparação textual incorreta.
- GitHub Actions run `34719833762`, disparado pela tag `v0.1.1`: PASS em macOS
  e Ubuntu para format, test, vet, shuffle ×3 e race; cross-build também passou.
- Release workflow `34719833750`: PASS. Publicou a GitHub Release estável
  `CLI Tools v0.1.1` com notas extraídas da seção versionada do changelog,
  quatro archives macOS/Linux para amd64/arm64, `SHA256SUMS` e attestations.
- `mise` 2026.4.7, em macOS arm64, resolveu `latest` como `0.1.1`, baixou
  `cli-tools_0.1.1_macos_arm64.tar.gz`, validou checksum e attestation e
  instalou o asset. `ai-profile --version` e `repo-zip --version` executados
  por `mise --no-config exec` retornaram `v0.1.1`.
- A prova de instalação usou HOME, TMP, `GH_CONFIG_DIR` e todos os diretórios
  `MISE_*` dentro de um sandbox regenerável no repositório, com discovery de
  config desabilitado e fallbacks de tokens do gh/Git desativados; nenhum path
  de configuração real apareceu na execução.
- Após um reinício local, `/usr/bin/git` acionou o shim do Xcode sob o ambiente
  mínimo e seus diagnósticos foram misturados a hashes por um helper de teste
  que usava `CombinedOutput`. O runner passou a priorizar o Apple Git real no
  macOS e o helper passou a separar stdout/stderr; a regra foi promovida para
  `AGENTS.md` e `docs/testing.md` para evitar repetição em outros projetos.

## Primeira tentativa de release

- A tag `v0.1.0` executou os jobs `verify` com PASS em macOS e Ubuntu no run
  `34719383662`.
- O build dos quatro assets e a extração das notas passaram, mas o job publish
  parou antes de criar a GitHub Release: `sha256sum -c dist/SHA256SUMS` resolveu
  os basenames contra a raiz do checkout.
- A correção executa o checker dentro de `dist` e possui regressão que valida a
  invocação exata do workflow. A tag `v0.1.0` não será movida nem reutilizada;
  a próxima tentativa usa a patch version `v0.1.1`.

## Auditoria de segurança dos testes locais

Após a implementação do suporte a worktrees, os testes foram re-auditados para execução em máquina pessoal. Subprocessos de teste agora recebem ambiente mínimo sintético via `internal/testenv`; Git global/system config, templates e hooks ficam desativados; HOME/TMP/XDG/Go caches ficam temporários; `GOTOOLCHAIN=local`, `GOPROXY=off` e `GOVCS=*:off` impedem downloads/rede do Go. `scripts/check-safe.sh` aplica o mesmo isolamento ao próprio processo `go test`/`vet`/`race` e remove somente o sandbox criado por `mktemp`; agentes o executam diretamente para evitar discovery prévio do mise. Nenhum teste usa `~/.ai-profiles`, repositórios reais, Keychain, Claude/Codex reais ou credenciais do usuário.

### Evidência após hardening do ambiente de testes

- `go test ./...`: PASS;
- `go vet ./...`: PASS;
- `go test -shuffle=on -count=3 ./...`: PASS;
- `go test -race ./...`: PASS;
- quatro fuzzers com `-fuzztime=5s`: PASS;
- cross-build compile-only: PASS para darwin/linux/windows × amd64/arm64 × 2 CLIs;
- criação concorrente do mesmo alias: PASS em 100 repetições com create-exclusive do lock;
- dry-run do release `v0.1.0`: quatro assets macOS/Linux gerados, checksums válidos e ambos os binários macOS arm64 responderam `v0.1.0`;
- a seção `v0.1.0` do `CHANGELOG.md` foi extraída como release notes com versão,
  data e listas categorizadas; versões/seções inválidas são recusadas por teste;
- uma segunda geração contra output não vazio foi recusada e o SHA-256 do
  asset existente permaneceu idêntico; parent/final symlink também são recusados
  por teste, sem criar arquivo no alvo externo sintético;
- nenhum corpus `testdata/fuzz` foi criado no source tree.

### Isolamento do mise

Uma verificação desta revisão mostrou que redirecionar apenas HOME e os
diretórios `MISE_*` não impediu o mise 2026.4.7 de descobrir a configuração
pessoal. O comando era somente leitura e as resoluções de rede falharam; nenhuma
configuração, instalação ou credential store foi alterada. A receita ativa foi
corrigida para exigir `MISE_NO_CONFIG=1`/`--no-config`, `GH_CONFIG_DIR`
sintético e fallbacks de credenciais desativados. A instalação real da release
deve usar essa receita mais restrita.
