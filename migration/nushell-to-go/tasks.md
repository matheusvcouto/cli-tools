# Tasks — estado de execução

Marcar apenas com evidência.

## Suite/arquitetura
- [x] módulo Go único;
- [x] `cmd/*` extensível;
- [x] stdlib-first e zero dependências externas hoje;
- [x] mini core sem framework próprio;
- [x] domínio sem `runtime.GOOS`;
- [x] diferenças reais de SO isoladas em build tags/capabilities;
- [x] Windows compile-only com stubs explícitos onde falta semântica segura;
- [x] docs ativas não dependem do legado para trabalho normal.

## `ai-profile`
- [x] Claude e Codex registry;
- [x] `agy` deliberadamente fora;
- [x] list humano + `--json`;
- [x] store JSON versionado;
- [x] lock, backup, fsync, replace confinado;
- [x] corrupção recusada;
- [x] create com rollback;
- [x] rename alias-only;
- [x] delete com confirmação, quarentena e rollback;
- [x] env isolation, incluindo roots Anthropic/Claude e workload identity;
- [x] argv passthrough;
- [x] process replacement/exit status em Unix;
- [x] ACP stdout limpo;
- [x] guidance Codex;
- [x] statusline embutida + merge seguro;
- [x] completions Bash/Fish/Zsh;
- [x] conversor transitório NUON→JSON sem executar Nushell;
- [x] E2E do binário cobre `new → list → run → acp` com executáveis falsos e exit codes reais;
- [x] runtime macOS confirmado pelo CI do repositório real.

## `repo-zip`
- [x] parser/naming;
- [x] Git como fonte de verdade;
- [x] `archive/zip` sem `zip`/`unzip` externos;
- [x] ignored/tracked/untracked;
- [x] symlink preservado sem dereference;
- [x] guards submodule/sparse/skip-worktree;
- [x] guards `.git`/tracked output;
- [x] `--git` via `git bundle`, com linked worktree, verificação e refs/status rechecados;
- [x] output confinado e temp no mesmo parent;
- [x] ZIP verificado pelo mesmo descriptor temporário;
- [x] rechecagem do conjunto elegível antes de publicar;
- [x] no-clobber e force separados em Unix;
- [x] mudança durante snapshot aborta;
- [x] runtime macOS confirmado pelo CI do repositório real.

## Qualidade/release
- [x] `go test ./...` local;
- [x] `go vet ./...` local;
- [x] shuffle/repeat local;
- [x] race local;
- [x] fuzz seeds + sessões fuzz limitadas para parsers/validações críticas;
- [x] cross-build Darwin/Linux/Windows amd64/arm64;
- [x] CI descobre `cmd/*` para cross-build;
- [x] release descobre `cmd/*`;
- [x] release exige Go 1.27.1;
- [x] release gera `SHA256SUMS`;
- [x] workflow de release exige Linux + macOS antes de publish;
- [x] smoke test do archive Linux previsto na workflow;
- [ ] workflow real executada em GitHub;
- [ ] instalação de release real via mise testada com HOME/MISE_* temporários.

## Cutover/histórico
- [x] instruções de cutover documentadas;
- [ ] migração real de dados executada pelo usuário, se necessária;
- [ ] comandos/imports antigos removidos do ambiente real pelo usuário;
- [ ] rollback real validado ou janela de rollback encerrada;
- [ ] `git mv migration/nushell-to-go docs/history/migrations/2026-09-nushell-to-go`;
- [ ] commit/push somente quando explicitamente solicitado.
