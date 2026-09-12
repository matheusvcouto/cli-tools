# ai-profile

`ai-profile` is a shell-agnostic launcher for isolated Claude Code and Codex homes. It does not emulate either tool's configuration or context discovery; it selects an isolated home and then execs the real CLI/ACP adapter in the caller's current working directory.

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

## Status line

`apply-statusline` is Claude-only. It merges only the `statusLine` property into that profile's `settings.json` and preserves all other settings.

## Scope of the guarantee

`ai-profile` isolates the selected tool's user account/configuration roots and removes known fixed tool-specific parent-shell overrides. It does not disable project configuration, managed organization policy, generic cloud credentials used by project tooling, or a provider credential named explicitly by the selected profile's own `env_key`. Those remain intentionally visible to the real Claude Code/Codex process according to each tool's own rules.
