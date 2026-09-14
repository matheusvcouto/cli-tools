# CLI Core vNext — implementation plan

> Status: **completed on 2026-09-13**. Acceptance gates passed; this plan is archived under D008 after final smoke verification.

## Goal

Replace the current per-tool CLI parsing/help/completion code with one reusable,
typed and declarative CLI platform. Existing CLIs must adapt to the core; the
core must not be constrained by their current shape.

This directory is the implementation handoff. An agent should be able to read
these files, inspect the repository, implement the migration in stages and know
what is architecture, what is current behavior and what evidence is required.

## Read first

1. [`../../../../AGENTS.md`](../../../../AGENTS.md)
2. [`../../../../ADR.md`](../../../../ADR.md)
3. [`../../../engineering.md`](../../../engineering.md)
4. [`../../../testing.md`](../../../testing.md)
5. [`../../../platforms.md`](../../../platforms.md)
6. [`current-state.md`](current-state.md)
7. [`architecture.md`](architecture.md)
8. [`api-and-model.md`](api-and-model.md)
9. [`completion-and-shells.md`](completion-and-shells.md)
10. [`runtime-and-platform.md`](runtime-and-platform.md)
11. [`versioning-and-release.md`](versioning-and-release.md)
12. [`testing-and-quality.md`](testing-and-quality.md)
13. [`risks-and-edge-cases.md`](risks-and-edge-cases.md)
14. [`migration.md`](migration.md)
15. [`tasks.md`](tasks.md)
16. [`references.md`](references.md)

## Target in one diagram

```text
Tool Spec + Modules + typed Values
              │
              ▼
         cli.Compile()
              │
              ▼
       immutable CompiledApp
   ┌──────────┼───────────────┐
   ▼          ▼               ▼
strict     partial          schema
parser     parser           /contract
   │          │
   ▼          ▼
execute    completion engine
   │          │
   ▼          ▼
handlers   neutral candidates
              │
              ▼
         shell adapters
  fish / nu / bash / zsh / pwsh
```

## Non-negotiable outcomes

- one command graph is the source of truth;
- no hand-written duplicate help/completion command lists;
- public `cli/` package with a deliberately small API;
- implementation details remain private under `cli/internal/`;
- strict and partial parsing share one grammar;
- typed arguments/flags with explicit codecs and completion hints;
- deterministic compilation into an immutable graph;
- structured diagnostics and machine output boundaries;
- capability/requirement checks before side effects;
- automatic shell integrations from the graph;
- Fish, Nushell, Bash, Zsh and PowerShell adapters;
- versioned dynamic-completion protocol;
- per-tool SemVer plus suite/module SemVer and independent schema/protocol versions;
- generated CLI contract locks and contract diff checks;
- docs/man/schema generation from the same graph;
- hermetic tests, fuzzing, shell conformance and benchmarks;
- removal of `internal/cliapp` after both tools migrate.

## Transition rule

`docs/architecture.md`, `docs/release.md` and tool READMEs describe the current
implemented product until their cutover phase lands. Do not rewrite them to
pretend the target already exists. When a phase becomes real, update active docs
in the same commit as the implementation.

## Implementation style

Prefer explicit data and pure functions over magic. Avoid global mutable
registries, hidden `init()` registration, reflection-driven core behavior and
framework-level dependency injection. Optimize architecture first: immutable
compiled state, lazy domain initialization and runtime-planned completion with thin shell adapters.
Measure allocations/latency before micro-optimizing.
