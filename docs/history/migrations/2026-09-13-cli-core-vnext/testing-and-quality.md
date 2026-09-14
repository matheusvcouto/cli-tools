# Testing and quality strategy

## Core principle

The CLI Core is infrastructure used by every executable. Treat parser,
completion protocol and contract generation as compatibility/security surfaces,
not merely convenience helpers.

## Test layers

### Compiler invariants

Table tests for every invalid Spec class:

- duplicate IDs/names/aliases;
- flag collisions;
- invalid arity/variadic position;
- invalid constraint graph;
- bad deprecation replacement;
- reserved namespace;
- unsupported adapter declaration;
- conflicting modules.

### Parser

Cover strict and partial parsing from the same grammar:

- nested commands;
- aliases;
- long/short flags;
- `--flag=value`;
- clustered short flags only when supported;
- interspersed options;
- `--` terminator;
- passthrough/trailing argv;
- repeated flags;
- missing/invalid values;
- Unicode and empty strings.

### Property/fuzz tests

Fuzz at least:

- strict parser never panics on arbitrary argv;
- partial parser never panics on arbitrary incomplete argv;
- completion request decoder/protocol;
- schema/contract decode/round-trip constraints;
- shell escaping/render helpers where pure enough.

Useful invariant: a successful strict parse should also be representable as a
valid terminal state of partial parse for the same argv.

### Golden tests

Use stable golden files for:

- help;
- schema;
- contract JSON;
- generated shell integration;
- man/Markdown reference docs.

Regeneration must be explicit so tests cannot silently bless changes.

### Shell adapter conformance

A shared behavioral suite runs against every adapter and exercises the cases
listed in `completion-and-shells.md`.

### Native shell E2E

Where a shell is available in CI, execute the generated integration in the real
shell rather than validating only strings. Cover Fish, Nushell, Bash, Zsh and
PowerShell across suitable runners.

Cross-build is not evidence that completion syntax works in that shell/OS.

### Platform/preflight tests

Prove an unavailable capability is rejected before domain side effects. Use
synthetic capability providers and filesystem/process fakes; do not depend on
real user state.

### Tool migration E2E

Existing `ai-profile` and `repo-zip` scenarios must be re-expressed through the
new core. Preserve domain safety tests. Add regressions for intentional breaking
changes such as removal of `repo-zip --version TEXT`.

## Benchmarks

Benchmark representative operations:

- compile small/medium/large Spec;
- strict parse;
- partial parse;
- help render;
- shell-neutral completion planning;
- dynamic completion dispatch without provider I/O;
- schema/contract generation.

Track allocations as a signal. Do not set arbitrary hard RAM limits without a
measured baseline.

Expected architectural performance properties:

- compile once per process;
- immutable graph/lookups reused;
- O(argv) or near-linear normal parsing;
- no domain/config I/O for help/version/completion script generation;
- no goroutine required for ordinary parse/help;
- dynamic completion only initializes what its provider needs.

## Race and determinism

`go test -race` remains a separate gate. Immutability should make concurrent
read-only use safe if the core is ever embedded in a long-lived process.

Run deterministic-output tests multiple times/shuffled to catch map-order leaks.

## Security/robustness cases

Explicitly test:

- shell escaping/injection boundaries;
- completion candidates containing spaces, tabs, quotes, Unicode and leading
  hyphens;
- secrets never present in schema/diagnostic/completion output;
- no prompt from dynamic completion;
- no command execution from typo suggestions;
- completion install/uninstall confined to synthetic HOME/config roots;
- protocol mismatch fails closed/quietly;
- machine stdout remains uncontaminated by diagnostics.

## Safe repository gates

Continue using the repository sandbox runner. Extend it rather than bypassing
its isolation model. If extra shells are installed in CI, tests must still use a
synthetic HOME and must not source the user's real shell configuration.

## Definition of done for core cutover

The migration is not complete until:

1. both existing CLIs use the shared core;
2. no duplicate manual command/help/completion trees remain;
3. `internal/cliapp` is removed;
4. contract locks are generated and checked;
5. all five target shell adapters pass conformance;
6. native shell E2E runs wherever CI provides that shell;
7. release/version tooling follows the new contract;
8. active docs/AGENTS describe the implemented state;
9. safe test, vet, shuffle and race gates pass;
10. benchmarks establish a baseline and reveal no obvious regression.
