# ai-profile changelog

## [1.0.0] - 2026-09-14

### Adicionado

- **ai-profile:** Migrate to CLI Core with generated shell completion and introspection

### Alterado

- **ai-profile:** Expose ACP only when the tool registry declares it and retire apply-statusline from the active ai-profile implementation while preserving the historical Nushell reference

## [1.0.1] - 2026-09-14

### Corrigido

- **ai-profile:** Fix generated Nushell completions

## [1.1.0] - 2026-09-27

### Adicionado

- **ai-profile:** Add and harden Grok Build profiles: isolated GROK_HOME, documented Codex skills/hooks defaults, inherited auth/compat override filtering that preserves administrative security restrictions, safe config identity checks, bounded profile metadata reads and writes, rejection of duplicate profile directories and orphaned profile state, safe feedback/trace routing, native ACP, and bounded fail-closed Windows npm resolution; fix invalid Claude settings causing panic
- **ai-profile:** Implement Windows locking, safe profile commits, and Job Object-contained native process execution

### Corrigido

- **ai-profile:** Prevent interactive profile deletion from removing an alias reassigned during confirmation by checking the originally displayed profile under the store lock; reject canceled launches before exec on Unix and process startup on Windows
- **ai-profile:** Reject all residual profile-root entries when index.json is missing, add validated explicit import of legacy NUON directories (including original index.nuon in the target root), bound and revalidate migration source, and preserve Windows PATH order when resolving official npm shims
