# Testes

## Regra central

Nenhum teste usa estado real do usuário.

- `t.TempDir()` para profiles/repos/outputs;
- HOME/USERPROFILE/XDG/TMP e caches Go redirecionados;
- secrets, SSH agent, editors, GOFLAGS e overrides Git do usuário não são copiados para subprocessos de teste;
- Git system/global config, templates e hooks do usuário são desabilitados;
- Claude/Codex/ACP reais nunca são executados;
- subprocess tests usam executáveis/processos sintéticos;
- Git real roda somente em repositório temporário criado pelo teste;
- Go usa `GOTOOLCHAIN=local`, `GOPROXY=off` e `GOVCS=*:off`: sem rede nem download automático.

## Camadas

### Unit/domain

Parsing, schema, naming, environment, store, archive e guards.

### Processos

O runner Unix substitui um processo de teste por outro processo de teste, provando `exec` e exit status sem chamar ferramentas do usuário.

### Git integration

Cobrir tracked/untracked/ignored, symlink, submodule/gitlink, skip-worktree, dirty state e mudança durante snapshot. Para `--git`, cobrir também linked worktree real, bundle verificável/restaurável, mudança de refs durante o snapshot, namespace `.repo-zip/` reservado e ausência de `.git` bruto no archive.

### Filesystem adversarial

Cobrir symlink em store/lock/backup/settings/profile/output, parent symlink durante criação de diretório, no-clobber, force replace, rollback e escapes.

Regressões de CLI incluem `repo-zip .` executado com cwd no próprio repo e comandos estáticos do `ai-profile` sem HOME/store.

### Fuzz

Fuzz tests focam superfícies pequenas expostas a entrada não confiável: alias, suffix/ZIP verification e o parser NUON transitório. Os seeds rodam em todo `go test`; sessões fuzz contínuas não fazem parte do gate padrão para manter CI determinística. Para smoke manual limitado e sandboxed (executado em cópia temporária do source, para que um crasher não grave `testdata/fuzz` no checkout):

```sh
mise run fuzz-smoke
# ou
./scripts/check-safe.sh fuzz
```

## E2E

`integration/e2e_unix_test.go` compila os dois binários e valida o caminho externo completo usando apenas HOME/profile root/repositório Git temporários e executáveis `codex`/`codex-acp` falsos. Nenhum teste chama contas, credentials ou CLIs reais.

## Gates locais/CI

No computador do usuário, use:

```sh
./scripts/check-safe.sh all
```

`mise run check` executa a mesma task e continua disponível como conveniência
interativa. Para uma auditoria de isolamento, agentes usam o script diretamente:
o mise pode fazer discovery/resolução de configuração global antes de iniciar a
task, mesmo quando o processo Go chamado depois está isolado.

O runner cria um diretório temporário exclusivo e executa format check, testes, vet, shuffle e race com ambiente mínimo. Ele só preserva `PATH` para localizar as toolchains instaladas. Ao terminar, remove apenas o diretório retornado por `mktemp`, com validação de prefixo antes do `rm -rf`.

`go test ./...` direto continua usando dados sintéticos dentro dos testes, mas o próprio comando Go pode usar os caches/configuração normais da sua conta. Por isso o runner sandboxed é a opção recomendada.

O projeto fixa Go 1.27.1 em `mise.toml`; o runner exige que a toolchain adequada
já esteja no PATH e impede download automático com `GOTOOLCHAIN=local`.

Cross-build é compile-only. O workflow de release repete testes nativos em Linux e macOS antes da publicação.
