# ai-profile

`ai-profile` is a shell-agnostic launcher for isolated Claude Code, Codex and Grok Build homes. It does not emulate the tools' configuration or context discovery; it selects an isolated home and then execs the real CLI/ACP adapter in the caller's current working directory.

## Profile isolation

### Claude Code

For a selected Claude profile, `ai-profile` sets:

```text
CLAUDE_CONFIG_DIR=<profile-dir>
ANTHROPIC_CONFIG_DIR=<profile-dir>/.anthropic
```

Claude Code uses this directory for the profile's user-scoped state and credentials. On macOS, Claude Code keys the Keychain entry by `CLAUDE_CONFIG_DIR`, so different profile directories resolve different login entries.

Claude Code also honors Anthropic CLI/SDK profiles, including the active or `default` profile under the Anthropic configuration directory. Pointing `ANTHROPIC_CONFIG_DIR` at a real, profile-local `.anthropic` directory prevents `~/.config/anthropic` from silently supplying a different profile or federation credential.

Profile-global files belong inside the selected profile directory, for example:

```text
<profile-dir>/
├── CLAUDE.md
├── settings.json
├── .anthropic/
├── rules/
├── skills/
└── agents/
```

Project context remains native to Claude Code because `ai-profile` preserves the working directory. Claude Code can therefore discover project `CLAUDE.md` / `.claude/CLAUDE.md`, `CLAUDE.local.md`, and `.claude/rules/**/*.md` normally.

Claude Code does **not** natively treat `AGENTS.md` as its memory file. To share project instructions with Codex, use a project `CLAUDE.md` containing `@AGENTS.md`, or a symlink when appropriate.

To prevent a selected profile from being silently replaced by parent-shell authentication/routing, `ai-profile` removes Claude-specific login/provider overrides before launch, including direct API/OAuth credentials and all fixed environment components of Workload Identity Federation. It intentionally preserves generic project credentials such as `AWS_PROFILE` and Google/Azure environment state: project commands may need them, and a profile that intentionally enables a cloud provider can still use the provider's normal credential chain.

As defense in depth, non-default Claude profiles add exclusions for the default `$HOME/.claude/CLAUDE.md`, `$HOME/.claude/CLAUDE.local.md`, and `$HOME/.claude/rules/**` while preserving existing `settings.json` keys. Managed organization policy and project settings still apply; profile isolation is not a bypass for organization/project policy.

### Codex

For a selected Codex profile, `ai-profile` sets:

```text
CODEX_HOME=<profile-dir>
```

Profile-global instructions belong in:

```text
<profile-dir>/AGENTS.override.md
```

or, when no override is needed:

```text
<profile-dir>/AGENTS.md
```

`ai-profile` does not copy or symlink guidance from the default `~/.codex`; each profile owns its own global instructions. Codex continues to discover applicable project `AGENTS.md` files from the project hierarchy because the wrapper preserves the caller's working directory.

Parent-shell authentication/state overrides (`OPENAI_API_KEY`, `OPENAI_BASE_URL`, `CODEX_API_KEY`, `CODEX_ACCESS_TOKEN`, `CODEX_SQLITE_HOME`, and the public `OPENAI_*` workload-identity variables) are removed before launch so the selected `CODEX_HOME` remains authoritative.

## run and ACP

`run` and `acp` use the same profile selection and environment isolation. ACP adds no wrapper output to stdout, because stdout belongs to the adapter protocol.

```sh
ai-profile claude run personal
ai-profile claude acp personal
ai-profile codex run work
ai-profile codex acp work
```

Arguments after the profile alias are passed literally to the target process.

## Scope of the guarantee

`ai-profile` isolates the selected tool's user account/configuration roots and removes known fixed tool-specific parent-shell overrides. It does not disable project configuration, managed organization policy, generic cloud credentials used by project tooling, or a provider credential named explicitly by the selected profile's own `env_key`. Those remain intentionally visible to the real Claude Code/Codex process according to each tool's own rules.

## Provider Grok Build

`grok` é provider ativo. `ai-profile grok new <nome>` cria um `GROK_HOME` próprio com `config.toml` regular e privado. O default desativa descoberta global de compatibilidade Claude/Cursor e descoberta Codex de skills/hooks/sessões; contexto do projeto continua obedecendo às regras e ao folder trust do próprio Grok.

- `ai-profile grok run <perfil> [args...]` executa TUI ou headless sem shell intermediário.
- `ai-profile grok acp <perfil> [agent-options...]` executa `grok agent <agent-options...> stdio`, porque opções de agent devem preceder o transporte.
- O wrapper nunca injeta `--always-approve`/`--yolo`; deny rules, hooks, folder trust e sandbox pertencem ao Grok.
- `XAI_API_KEY`, overlays de auth/endpoint/config, compatibilidade por env e paths externos herdados são removidos. **Não** são removidos `GROK_DISABLE_API_KEY_AUTH`, `GROK_FORCE_LOGIN_TEAM_ID`, `GROK_SANDBOX` e os requisitos administrados: são restrições de segurança, não credenciais. `config.toml` pertence ao profile; alterações explícitas nele (inclusive opt-in de compatibilidade) prevalecem sobre o shell herdado. Config é validada como arquivo regular estável via `os.Root`, sem reescrita em cada execução.
- Auto-update é desativado nas execuções lançadas por `ai-profile`, evitando mutação da instalação durante TUI/headless/ACP.
- No Windows, `.cmd` não é executado por `cmd.exe`; o resolver valida o manifesto instalado de `@xai-official/grok` e chama `bin/grok` via `node.exe`. A instalação selecionada segue a ordem real do `PATH`/`PATHEXT`: um `.cmd` de um diretório anterior não é trocado silenciosamente por um `grok.exe` de outro diretório.
- A distribuição oficial observada documenta Windows x64, Linux x64/arm64 e macOS arm64. O fato de `ai-profile` compilar em Windows ARM64 não implica que a xAI distribua Grok Build ARM64 para Windows.

Limites de evidência, problemas corrigidos e gates pendentes estão em [`plans/grok-build/CODE_REVIEW_003.md`](../plans/grok-build/CODE_REVIEW_003.md). O isolamento de `GROK_HOME` **não** é isolamento completo dos arquivos de projeto, das permissões explícitas de arquivos legados ou de credenciais genéricas usadas por comandos do agente.

## Futuro: View Limits (somente planejamento)

O projeto pretende adicionar visualização *read-only* de limites por provider e perfil, sem acessar credenciais durante `list`/`run`/`acp` nem confundir saldo de assinatura com limites de API ou contagem local de sessões. **Não existe comando `limits` nesta versão.** Arquitetura, contratos possíveis e gates de segurança estão em [`plans/view-limits/README.md`](../plans/view-limits/README.md). A revisão de segurança vigente é [`plans/grok-build/CODE_REVIEW_004.md`](../plans/grok-build/CODE_REVIEW_004.md).

## Recuperação conservadora quando o índice desaparece

`index.json` é a única fonte de associação entre aliases e diretórios dos
providers. Se esse arquivo desaparecer, **a existência de `index.json.bak` ou
qualquer entrada residual no root (exceto o lock regular `.index.lock`)
bloqueia a criação e a listagem**. Isso inclui diretórios legados `claude-id`,
`index.nuon` e quarentenas `.deleted-*`: a versão anterior só reconhecia
nomes físicos gerados pela versão nova. Não recrie um índice vazio por cima
dos diretórios existentes: isso pode perder referências a sessões/credenciais,
especialmente em perfis que foram renomeados. Após preservar uma cópia externa do diretório `.ai-profiles`,
revise o backup e recupere explicitamente um índice íntegro e consistente;
`index.json.bak` registra o estado *anterior* ao último commit e pode estar
incompleto em relação ao estado atual. A ferramenta não recupera aliases
automaticamente a partir do nome físico de diretórios.

A preparação de Claude rejeita `settings.json` com JSON `null` em vez de
entrar em panic ou sobrescrever o arquivo inválido. Corrija manualmente
seus settings de perfil para um objeto JSON válido antes de iniciar Claude.

No Windows, resolução npm de Claude, Codex, Grok e adapters ACP exige um
`package.json` regular, manifesto e entrypoint previstos, limite de leitura
de 2 MiB e Node nativo. A validação do manifesto não equivale a assinatura
ou verificação criptográfica do pacote instalado, nem substitui testes
nativos do CLI oficial.


## Migração explícita do índice NUON legado

`migrate-ai-profile-index --from /path/index.nuon --to-root /path/.ai-profiles`
é uma operação **explícita**: exige `index.json` e `index.json.bak` ausentes,
confere que cada diretório informado existe como diretório real imediatamente
abaixo do root, rejeita diretórios/arquivos não referenciados e revalida os
bytes do NUON de origem antes de gravar o JSON. Quando a origem ainda é
`<root>/index.nuon` (layout Nushell original), esse arquivo é permitido como
única entrada residual adicional ao lock e aos diretórios declarados, e **não
é removido**. A leitura do NUON é limitada a 8 MiB e rejeita symlinks/arquivos
especiais. Em caso de inconsistência, preserve uma cópia externa de todo o
root e reconcilie a origem/diretórios antes de repetir a migração; o programa
não infere aliases a partir de nomes físicos nem faz recuperação automática.
