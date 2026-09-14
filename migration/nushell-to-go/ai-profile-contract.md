# `ai-profile` — contrato Go

Este contrato descreve a implementação Go atual. O legado serve apenas como fonte histórica de capacidades e dados; aparência, mensagens e integração específica com Nushell não são contrato.

## Interface

```text
ai-profile <claude|codex> [list] [--json]
ai-profile <claude|codex> new <alias>
ai-profile <claude|codex> rename <alias> <novo-alias>
ai-profile <claude|codex> delete <alias>
ai-profile <claude|codex> run <alias> [...args]
ai-profile <claude|codex> acp <alias> [...args]
ai-profile claude apply-statusline <alias> [template]
ai-profile completion list
ai-profile completion generate <fish|nushell|bash|zsh|powershell>
ai-profile completion install [fish|nushell|bash|zsh|powershell]
```

`list` é a ação default. `--json` é a interface estável para automação; a saída humana pode evoluir.

## Store

- root default: `~/.ai-profiles`;
- `AI_PROFILE_ROOT` permite root explícito e torna testes isoláveis;
- `index.json` usa schema versionado;
- `index.json.bak` e `.index.lock` ficam no mesmo root;
- corrupção, schema desconhecido, duplicata ou entrada inválida são erro — nunca “store vazio”;
- metadata contém `tool`, `alias`, `dir`, `created_at`, nunca secrets;
- perfis migrados preservam `dir` e `created_at` exatamente;
- perfis novos usam diretório opaco e timestamp RFC3339 UTC.

Mutações seguem `lock → reler/validar → calcular → temp no mesmo root → fsync → backup → replace seguro`. Arquivos internos inesperadamente symlinkados são recusados.

## CRUD

- alias: `^[A-Za-z0-9_-]+$`;
- duplicata é proibida por tool, mas o mesmo alias pode existir em tools diferentes;
- `rename` altera somente o alias, nunca move o diretório físico;
- `new` remove somente o diretório criado pela própria operação se o commit falhar;
- `delete` move primeiro para quarentena no mesmo root, persiste o índice e faz rollback se o commit falhar; cleanup posterior que falhar é reportado com o nome da quarentena.

## Execução

Nenhum shell intermediário. Tudo após o alias em `run`/`acp` é argv literal. Em Unix, process replacement preserva stdio e exit status; Windows continua explicitamente sem backend equivalente até ser implementado e validado.

### Claude

Seta `CLAUDE_CONFIG_DIR=<profile-dir>` e `ANTHROPIC_CONFIG_DIR=<profile-dir>/.anthropic`. O segundo root isola profiles/federation do Anthropic CLI/SDK que Claude Code também consulta; `.anthropic` precisa ser diretório real e confinado. O profile é a fonte de estado/credencial user-scope; contexto global pertence ao próprio profile (`CLAUDE.md`, `rules/`, `settings.json`, `skills/`, `agents/`). O cwd é preservado para o discovery nativo de `CLAUDE.md`, `CLAUDE.local.md` e `.claude/rules/` do projeto.

Antes de executar, remove overrides Claude-specific herdados do shell que poderiam substituir autenticação, provider, endpoint ou roots de estado do profile: selectors `CLAUDE_CODE_USE_*`, auth-skip de providers, tokens OAuth, `ANTHROPIC_API_KEY`/`ANTHROPIC_AUTH_TOKEN`, profile IDs e todos os componentes fixos de federation (`RULE_ID`, `ORGANIZATION_ID`, `SERVICE_ACCOUNT_ID`, `WORKSPACE_ID`, identity token literal/arquivo), endpoints/credentials Anthropic para AWS/Bedrock/Foundry/Vertex, `AWS_BEARER_TOKEN_BEDROCK`, `ANTHROPIC_CUSTOM_HEADERS`, `CLAUDE_SECURESTORAGE_CONFIG_DIR` e `CLAUDE_CODE_PLUGIN_CACHE_DIR`. Credenciais genéricas de projeto (ex.: `AWS_PROFILE`, credenciais GCP/Azure) não são apagadas.

Para profiles diferentes do default, adiciona `claudeMdExcludes` para `$HOME/.claude/CLAUDE.md`, `$HOME/.claude/CLAUDE.local.md` e `$HOME/.claude/rules/**`, preservando todas as outras chaves de `settings.json`. Isso é defesa em profundidade; managed policy e settings do projeto continuam válidos. ACP usa `claude-agent-acp`.

### Codex

Seta `CODEX_HOME=<profile-dir>` e remove `OPENAI_API_KEY`, `OPENAI_BASE_URL`, `CODEX_API_KEY`, `CODEX_ACCESS_TOKEN`, `CODEX_SQLITE_HOME`, `OPENAI_FEDERATION_RULE_ID`, `OPENAI_IDENTITY_TOKEN_FILE` e `OPENAI_WORKLOAD_IDENTITY_CONTEXT`. ACP usa `codex-acp`.

Guidance global é **profile-native**: `AGENTS.override.md` ou `AGENTS.md` deve estar dentro do `CODEX_HOME` selecionado. O wrapper nunca copia/symlinka guidance de `~/.codex`. O cwd é preservado para que Codex descubra a cadeia `AGENTS.md` do projeto normalmente.

## ACP

- mesma política de ambiente de `run`;
- wrapper não escreve diagnóstico em stdout;
- stdin/stdout/stderr são herdados pelo processo final;
- argv e exit status são preservados.

## Statusline

`apply-statusline` é Claude-only e altera somente `statusLine` em `settings.json`, preservando as demais chaves. O template `default` é embutido no binário; templates externos podem vir de `AI_PROFILE_TEMPLATES_DIR`. Escrita ocorre dentro da mesma safety root do profile.

## Shells e automação

O runtime não depende de Nushell. O binário funciona igualmente quando chamado por Bash, Fish, Zsh, Nu, IDE, launcher ou outro processo. Completions opcionais são geradas para Fish, Nushell, Bash, Zsh e PowerShell; nenhuma delas lê o store diretamente. O adapter de completion Nushell exige Nu 0.114+ porque usa `commandline complete`; isso é requisito apenas da integração de completion, não do runtime de `ai-profile`.

## Segurança e testes

- nenhuma fixture usa HOME, conta, credential store ou CLI real;
- testes usam roots e executáveis sintéticos;
- profiles, índice, lock, backup e settings symlinkados de forma insegura são recusados;
- nenhuma validação automática acessa Keychain/Credential Manager/Secret Service.

## Suporte

Linux e macOS compartilham o backend Unix quando a semântica é a mesma e passam por runtime tests em CI. Windows é compile-only enquanto capabilities críticas não tiverem implementação e testes próprios. Compilar não significa suportar.
