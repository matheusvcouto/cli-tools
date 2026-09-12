# Evidências de validação

Última revisão local: 2026-09-12 (revisada após auditoria atual de isolamento/contexto de `ai-profile`).

Este arquivo registra **o que realmente foi executado**. Cross-build não é tratado como teste de runtime e nenhum item externo é inferido a partir de teste local.

## Ambiente local

```text
GOOS/GOARCH: linux/amd64
Go: go1.23.2
Dependências de módulo externas: nenhuma
```

A toolchain oficial do projeto/release é Go 1.27.1. O ambiente local não conseguiu baixar essa toolchain por restrição de rede. Portanto:

- os testes locais exercitam o fallback conservador de `internal/safefs` para Go < 1.25;
- a implementação `os.Root` selecionada em Go 1.27.1 foi revisada contra a API oficial, mas sua compilação/runtime é gate do CI oficial;
- isso **não** é representado como evidência local de Go 1.27.1.

## Gates locais executados

Todos passaram no estado empacotado:

```sh
test -z "$(gofmt -l .)"
go test ./...
go vet ./...
go test -shuffle=on -count=3 ./...
go test -race ./...
```

O E2E incluído em `integration/e2e_unix_test.go` compila e executa os binários reais com HOME, profile root, Git repo e executáveis Codex/ACP exclusivamente sintéticos.

## Fuzz smoke executado

Sessões limitadas de 5 segundos por alvo passaram sem panic/falha:

- `FuzzValidateAlias`: ~233k execuções na revisão mais recente;
- `FuzzValidateSuffix`: ~454k execuções na revisão mais recente;
- `FuzzVerifyZipNeverPanics`: ~191k execuções na revisão mais recente;
- `FuzzParseLegacyNUONNeverPanics`: ~456k execuções na revisão mais recente.

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
- `mise.toml` parseia como TOML válido; o executável `mise` não está instalado neste ambiente, então nenhuma instalação foi simulada localmente;
- `actions/checkout@v7`, `actions/setup-go@v7` e `actions/attest@v4` foram conferidos contra as versões oficiais atuais;
- API `os.Root` usada por `safefs` foi conferida contra a documentação oficial atual;
- release e CI descobrem `cmd/*` em vez de manter lista de CLIs distribuídas;
- `repo-zip .` foi testado com o próprio repositório como cwd, evitando regressão no output automático;
- `ai-profile --version`, `--help` e `completion <shell>` foram testados sem HOME/store;
- criação de diretórios sensíveis usa `safefs.EnsureDir`, com teste provando que parent symlink não cria conteúdo fora;
- `repo-zip --git` foi migrado de cópia bruta de `.git` para `git bundle`; linked worktree real passou em teste de pacote e E2E do binário compilado;
- o bundle gerado é verificado com `git bundle verify` e o teste de worktree também prova clone/restauração do HEAD;
- mudanças em refs não-HEAD durante o snapshot abortam a publicação;
- `.gitignore` continua sendo respeitado em linked worktree com `--git`;
- o E2E compila `repo-zip`, cria uma linked worktree sintética e valida o bundle gerado pelo binário real;
- `.repo-zip/` é reservado no archive para impedir colisão entre arquivos do usuário e metadata interna;
- repositório sem commit recebe erro explícito em `--git`, pois Git não produz bundle restaurável vazio;
- release publica apenas macOS/Linux; Windows continua compile-only e `repo-zip` falha antes de efeitos colaterais de output.
- `ai-profile` foi re-auditado contra a documentação atual de Claude Code e o código/documentação atual do Codex: `CLAUDE_CONFIG_DIR`/`CODEX_HOME` continuam sendo os roots corretos; parent-shell auth/provider overrides conhecidos são removidos; Codex não herda mais `~/.codex/AGENTS*`; cwd do projeto é preservado; Claude profiles mantêm seu `CLAUDE.md` próprio e recebem defesa em profundidade contra contexto do `~/.claude` default;
- E2E do `ai-profile` prova `run` e `acp` para Codex/Claude com profile-global guidance própria, project guidance no cwd, argv/exit status e ausência dos redirects de autenticação testados.

## Gates externos ainda abertos

Não marcar como concluídos sem evidência real:

1. CI GitHub em macOS com Go 1.27.1 verde;
2. workflow de release real executada por tag;
3. asset real instalado via `mise` com HOME/MISE_* temporários e ambos os binários validados;
4. cutover real autorizado pelo usuário;
5. só então mover esta migração para `docs/history/migrations/2026-09-nushell-to-go/`.

## Auditoria de segurança dos testes locais

Após a implementação do suporte a worktrees, os testes foram re-auditados para execução em máquina pessoal. Subprocessos de teste agora recebem ambiente mínimo sintético via `internal/testenv`; Git global/system config, templates e hooks ficam desativados; HOME/TMP/XDG/Go caches ficam temporários; `GOTOOLCHAIN=local`, `GOPROXY=off` e `GOVCS=*:off` impedem downloads/rede do Go. `mise run check` usa `scripts/check-safe.sh`, que aplica o mesmo isolamento ao próprio processo `go test`/`vet`/`race` e remove somente o sandbox criado por `mktemp`. Nenhum teste usa `~/.ai-profiles`, repositórios reais, Keychain, Claude/Codex reais ou credenciais do usuário.

### Evidência após hardening do ambiente de testes

- `./scripts/check-safe.sh test`: PASS;
- `./scripts/check-safe.sh vet`: PASS;
- `./scripts/check-safe.sh race`: PASS;
- `./scripts/check-safe.sh fuzz`: PASS, executando fuzzers numa cópia temporária do source;
- shuffle: três execuções sandboxed com `-shuffle=on -count=1`: PASS. O comando único `-count=3` excede o limite temporal desta sessão por repetir o E2E, não por falha dos testes;
- cross-build compile-only: PASS novamente para darwin/linux/windows × amd64/arm64 × 2 CLIs;
- após os testes, não permaneceram diretórios `cli-tools-test.*` nem corpus `testdata/fuzz` no source tree.
