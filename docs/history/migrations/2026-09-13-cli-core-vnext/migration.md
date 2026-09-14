# Migration strategy

Breaking changes are allowed. Prefer a clean target architecture over adapters
that permanently preserve accidental old CLI structure.

## Rule: build core independently first

Do not start by incrementally moving random helpers out of `ai-profile` and
`repo-zip`. That tends to encode their current quirks into the core.

First implement the core against synthetic fixture CLIs that exercise the model.
Then migrate real tools.

## Phase 0 — decision/docs baseline

Already started by this plan:

- former D014 removed/replaced by D019;
- D012 redefined around shell adapters;
- D019–D021 define target CLI Core/version/contracts.

Do not yet rewrite active runtime docs to claim implementation exists.

## Phase 1 — public model + compiler

Create `cli/` and private implementation packages.

Deliver:

- App/Command/Arg/Flag/Value concepts;
- stable IDs;
- generic codecs;
- modules/composition;
- compiler and invariant validation;
- immutable compiled graph;
- synthetic fixture specs/tests.

No real tool migration yet.

## Phase 2 — strict/partial parser + diagnostics

Deliver:

- one grammar engine;
- strict and partial modes;
- typed binding;
- constraints;
- structured diagnostics;
- exit class mapping;
- suggestions;
- help generation.

Fuzz before depending on the parser from real tools.

## Phase 3 — runtime lifecycle

Deliver:

- context/cancellation;
- stream/terminal contracts;
- lazy dependencies;
- availability/capability model;
- runtime requirements/probes;
- interaction abstraction;
- middleware boundary;
- generated `doctor` foundation.

## Phase 4 — completion protocol + adapters

Implement neutral completion engine and protocol first, then adapters:

1. Fish;
2. Nushell;
3. Bash;
4. Zsh;
5. PowerShell.

Fish/Nushell first because they are primary user requirements and exercise two
different shell transports. Installed integrations remain spec-agnostic; the
runtime planner is the only source of commands, flags, aliases and values.

Add conformance tests before claiming each adapter supported.

## Phase 5 — introspection/contracts/docs

Deliver:

- versioned CLI schema;
- deterministic `cli.contract.json` generator;
- contract diff tool;
- Markdown/man generation;
- internal `__cli schema/complete` protocol endpoints.

## Phase 6 — migrate `repo-zip`

Migrate the simpler command surface first.

Intentional breaking change:

- remove `-v/--version TEXT` as suffix alias;
- keep `--suffix TEXT`;
- reserve root `--version` for executable version.

Prove:

- existing archive/domain tests remain independent of CLI parser;
- interspersed options behavior is explicitly represented by the new grammar;
- help and completion come only from the Spec.

## Phase 7 — migrate `ai-profile`

Replace:

- manual routing switch;
- hard-coded usage/help;
- completion scripts;
- `__complete tools/actions/profiles/templates`;
- static invocation workaround.

Model profile/template completion as dynamic providers. Model `run/acp`
passthrough explicitly. Preserve ACP stdout contract.

## Phase 8 — remove legacy CLI layer

Delete `internal/cliapp` after both tools are migrated and all consumers are
gone. Remove obsolete completion endpoints/scripts and parser helpers.

## Phase 9 — versioning/release cutover

Implement atomically:

- `cmd/<tool>/tool.json`;
- `changes/` records;
- release preparation tooling;
- individual tool version embedding;
- suite/module version handling;
- contract-lock validation;
- release smoke-test updates;
- docs/release.md update;
- AGENTS.md update;
- CI workflow updates.

Do not leave release tooling expecting one version while binaries report another.

## Phase 10 — active docs cutover

Once implementation is real, update:

- `docs/architecture.md`;
- `docs/adding-tools.md`;
- `docs/engineering.md` dependency/framework wording;
- `docs/platforms.md` CLI capability model;
- `docs/testing.md` core/shell tests;
- `docs/release.md` version/change-record flow;
- root `AGENTS.md`;
- tool READMEs/ADRs where syntax changed.

Then move this plan to history according to D008 only after all acceptance
criteria are satisfied.

## Rollback discipline

Core phases should land in reviewable commits. During migration, a tool should
use either the old CLI path or the new one as the authoritative execution path;
do not indefinitely maintain two live parsers and compare results at runtime.

Temporary test-only parity harnesses are acceptable and should be removed after
cutover.
