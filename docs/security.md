# Segurança

## `ai-profile`

- store JSON com schema versionado;
- corrupção nunca vira estado vazio;
- criação/rename/delete e demais read-modify-write persistentes usam lock cross-process;
- `index.json`, backup, lock e `settings.json` recusam symlink/special file;
- temp + fsync + replace dentro da mesma safety root;
- backup do estado anterior;
- diretório físico do profile permanece opaco e não muda em rename;
- `new` cria diretório e persiste índice sob a mesma root+lock, com rollback se o commit falhar;
- delete usa quarentena + rollback se o commit falhar;
- secrets configurados para limpeza são removidos do ambiente filho;
- Claude recebe `CLAUDE_CONFIG_DIR` e um `ANTHROPIC_CONFIG_DIR` confinado ao mesmo profile, evitando herança do profile Anthropic ativo/default do usuário;
- componentes fixos de workload identity do Codex e do Claude são removidos do ambiente herdado;
- ACP não imprime bytes do wrapper em stdout;
- `AI_PROFILE_ROOT` permite isolamento explícito em testes/uso avançado.

## `repo-zip`

- Git decide tracked/untracked/ignored;
- ignored não entra;
- saída tracked e saída dentro de `.git` são recusadas;
- symlink é armazenado como symlink e nunca dereferenciado;
- regular files são revalidados durante a leitura;
- conjunto de arquivos elegíveis é comparado novamente antes da publicação;
- `--git` não percorre nem copia `.git`; usa `git bundle create --all` e verifica o bundle com o próprio Git antes de inseri-lo no ZIP;
- linked worktrees são suportadas sem resolver manualmente `$GIT_DIR`/`$GIT_COMMON_DIR`;
- token de consistência de `--git` inclui HEAD, refs/reachable refs e status antes/depois do snapshot;
- overrides Git sensíveis vindos do ambiente, incluindo `GIT_ALTERNATE_OBJECT_DIRECTORIES`, são removidos antes de executar Git;
- `.repo-zip/` é namespace reservado quando `--git` está ativo, evitando colisão com metadata interna;
- ZIP é reaberto pelo mesmo descriptor temporário e cada entry é lida;
- verificação aceita apenas entry regular ou symlink;
- temp fica no mesmo parent do destino;
- `force` e no-clobber usam commits diferentes;
- `.tmp/repo-zip` recusa componentes symlink.

## Root confinado

Build oficial usa `os.Root` através de `internal/safefs`. `safefs.EnsureDir` ancora criação no ancestral real existente e cria o restante pela safety root, evitando `MkdirAll` direto em paths sensíveis. `safefs.Open` fixa/revalida a identidade do diretório; depois disso, operações sensíveis continuam relativas à raiz aberta em vez de voltar a resolver paths absolutos.

## Dependências

As CLIs atualmente usam somente a standard library. A política é stdlib-first, não stdlib-only.

## Supply chain

- release com `CGO_ENABLED=0`;
- `-trimpath`;
- `SHA256SUMS`;
- release só depois de gates Linux + macOS;
- artifacts contêm apenas `bin/*`.
