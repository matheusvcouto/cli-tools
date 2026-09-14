# Agent implementation guide

## Mission

Implement the target architecture in this directory, not a minimal patch around
the existing CLIs. Existing `ai-profile` and `repo-zip` are migration consumers,
not constraints on the public core design.

## Required reasoning before a structural change

For every new abstraction, answer from code/tests:

1. What semantic responsibility does it own?
2. Is it part of public `cli/` API or private implementation?
3. Can another shell/platform/tool implement the same contract without editing
   unrelated core logic?
4. Does it create a second source of truth?
5. Can it remain deterministic and hermetically testable?
6. Does it accidentally initialize domain/user state during help/completion?
7. Is a generic boolean being used where an enum/capability/state type is safer?

## Public API discipline

The `cli/` package is intentionally public. Keep it small.

- Prefer constructors/options/builders that preserve invariants.
- Do not export the compiled parser's internal node structs.
- Do not expose mutable maps/slices that let callers corrupt compiled state.
- Document contracts and concurrency/lifecycle expectations.
- Avoid names tied to `ai-profile`/`repo-zip`.
- Before adding an exported symbol, verify it represents a reusable concept.

## Do not overfit tests

Implement semantic behavior, then build tool migrations. Do not add parser
special cases merely to preserve accidental old syntax unless the migration plan
explicitly says to preserve it.

## Reference-project policy

External projects in `references.md` are learning sources, not code to copy
blindly. Use them to inspect:

- how command graphs are modeled;
- where completion bugs/quoting edge cases arise;
- how shell-specific capabilities differ;
- how dynamic completion separates candidate discovery from rendering;
- how version/protocol mismatch is handled.

Preserve this repository's own safety/platform contracts even when references do
something different.

## Commit/review rhythm

Prefer coherent milestones:

1. model/compiler;
2. parser/diagnostics;
3. runtime/capabilities;
4. completion protocol;
5. shell adapters;
6. schema/contracts;
7. tool migrations;
8. release/versioning;
9. cleanup/docs.

At each milestone run relevant safe gates and inspect the diff for accidental API
surface growth. Do not wait until the end to fuzz parser/protocol code.

## Quality red flags

Stop and redesign if you see:

- command/action names repeated in multiple slices/switches;
- shell names branching inside domain handlers;
- `runtime.GOOS` in business logic;
- `strings.Fields` parsing machine completion output;
- errors compared by string;
- global command registries mutated by `init()`;
- reflection tags becoming the only source of CLI semantics;
- help/version opening user config/store;
- dynamic completion mutating state;
- generated files depending on map iteration order;
- a public core type importing tool-specific packages;
- a new `utils/common/shared` dumping ground.

## When to update active docs

Only update an active doc from “current” to “new core” when that behavior exists
in code and tests. ADR records the accepted destination; this plan bridges the
transition.
