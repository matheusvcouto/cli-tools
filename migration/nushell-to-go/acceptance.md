# Gates de aceitação

Evidência detalhada da última execução local: [`validation.md`](validation.md).

## Já comprovado localmente

- [x] um módulo Go;
- [x] Go 1.27.1 pinado em mise/CI/release;
- [x] `go test ./...`;
- [x] `go vet ./...`;
- [x] shuffle/repeat;
- [x] race detector;
- [x] fuzz seeds em `go test` + sessões fuzz limitadas sem falhas;
- [x] sem dependências externas;
- [x] fixtures e repos sintéticos;
- [x] cross-build Darwin/Linux/Windows amd64/arm64;
- [x] shell não faz parte da arquitetura;
- [x] runtime não chama Nushell;
- [x] E2E dos binários reais usa somente HOME/profile root/repo/fake executables sintéticos;

## `ai-profile`

- [x] capacidades funcionais do baseline cobertas;
- [x] aliases/diretórios/created_at preserváveis na migração;
- [x] rename não move diretório;
- [x] delete transacional com rollback;
- [x] env isolation atual para overrides de auth/provider/state de Claude e Codex;
- [x] run/ACP preservam argv e exit em Unix;
- [x] ACP sem output do wrapper;
- [x] context isolation atual: Codex profile-native `AGENTS*`, Claude profile-native `CLAUDE.md`/rules + cwd de projeto;
- [x] statusline merge;
- [x] Bash/Fish/Zsh completion;
- [x] nenhum teste usa CLI/credencial real;
- [ ] macOS CI real verde.

## `repo-zip`

- [x] Git continua fonte de verdade;
- [x] Go puro para create/verify do ZIP;
- [x] symlinks preservados;
- [x] guards de Git;
- [x] containment do destino;
- [x] `--git` gera/verifica bundle Git restaurável, sem copiar `.git`, inclusive em linked worktree real + E2E;
- [x] ZIP verificado antes da publicação;
- [x] mudança no conjunto de arquivos aborta;
- [x] no-clobber/force sem remove-then-rename em Unix;
- [ ] macOS CI real verde.

## Release/mise

- [x] tooling descobre `cmd/*`;
- [x] releases publicam apenas macOS/Linux inicialmente;
- [x] artifacts usam `bin/`;
- [x] checksums;
- [x] release workflow depende de testes Linux/macOS;
- [ ] tag/release real executada;
- [ ] `mise use -g github:matheusvcouto/cli-tools@<versão>` validado em sandbox de mise.

## Encerramento

A migração só vira histórico depois dos itens ainda abertos acima e do cutover autorizado pelo usuário.
