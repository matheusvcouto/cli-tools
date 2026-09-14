# References

Use primary/official documentation first. These references justify design
patterns; they are not dependencies automatically approved for the project.

## Repository-local references

Read these before implementation:

- `ADR.md` — accepted shared decisions and transition rules.
- `AGENTS.md` — safety, testing, platform and release constraints.
- `docs/architecture.md` — current architecture (pre-cutover).
- `docs/engineering.md` — engineering/error/dependency conventions.
- `docs/testing.md` — hermetic test model.
- `docs/platforms.md` — native capability boundaries.
- `docs/portability.md` — filesystem/path lessons from native CI.
- `docs/security.md` — domain security invariants that CLI migration must not
  weaken.
- `docs/release.md` — current release process until versioning cutover.
- `docs/adding-tools.md` — current new-tool process; must be replaced at cutover.
- `internal/aiprofile/app.go` — duplicated routing/help/completion pressure.
- `internal/repozip/app.go` and `internal/repozip/options.go` — independent parser
  and current `--version` ambiguity.
- `internal/cliapp/cliapp.go` — legacy mini-core to remove.
- `internal/version/version.go` — current suite-only version source.
- `tools/release/` and `.github/workflows/release.yml` — release discovery and
  smoke behavior that must change atomically.

## CLI architecture/completion references

### Cobra

- Shell completion:
  https://cobra.dev/docs/how-to-guides/shell-completion/
- Documentation index / command model references:
  https://cobra.dev/docs/

Lessons to inspect:

- command-tree-driven completion;
- shell completion directives and custom/dynamic candidates;
- generated documentation from the same command model.

Do not assume Cobra's API is the target API.

### clap / clap_complete

- `ValueHint`:
  https://docs.rs/clap_complete/latest/clap_complete/enum.ValueHint.html
- shell-agnostic `CompletionCandidate`:
  https://docs.rs/clap_complete/latest/clap_complete/struct.CompletionCandidate.html
- dynamic completer traits:
  https://docs.rs/clap_complete/latest/clap_complete/engine/trait.ValueCompleter.html
- completion crate overview:
  https://docs.rs/clap_complete/latest/clap_complete/

Lessons to inspect:

- semantic value hints separate from raw language type;
- neutral candidate representation before shell rendering;
- static and dynamic completion are useful upstream patterns, but this core deliberately keeps installed scripts spec-agnostic and resolves CLI metadata at runtime;
- explicit handling of wrapper/trailing command arguments.

### Nushell

- Extern signatures:
  https://www.nushell.sh/book/externs.html
- Custom completions:
  https://www.nushell.sh/book/custom_completions.html
- External completers:
  https://www.nushell.sh/cookbook/external_completers.html

Lessons to inspect:

- typed external signatures can provide parse-time type checking and highlighting, but embedding the CLI tree would create a stale second source of truth here;
- completers attach to specific argument/flag types;
- completion records can carry descriptions/styles/options;
- shell adapters should exploit shell capabilities rather than emit only strings.

### Fish

- `complete` command documentation:
  https://fishshell.com/docs/current/cmds/complete.html
- Current Fish documentation:
  https://fishshell.com/docs/current/

Lessons to inspect:

- native declarations can avoid spawning a binary, but duplicating command metadata is intentionally rejected here in favor of runtime planner parity;
- shell quoting/tokenization belongs in the adapter.

### Carapace

Repository:

- https://github.com/carapace-sh/carapace

Lessons to inspect in source/docs before implementing adapters:

- separate completion resolution from shell-specific formatting;
- multi-shell behavior and escaping;
- avoid copying global mutable registry patterns that hurt deterministic tests.

## Versioning/release references

### Semantic Versioning

- https://semver.org/spec/v2.0.0.html

Use for suite/module and stable tool public contracts. Remember that the Go
module tag controls external consumers of the public `cli/` package.

### Go build metadata

- `runtime/debug.ReadBuildInfo`:
  https://pkg.go.dev/runtime/debug#ReadBuildInfo

Use embedded module/VCS/build metadata where possible instead of adding
non-reproducible timestamps.

### Go plugins

- https://pkg.go.dev/plugin

The official warnings include limited OS support and race/build/deployment
constraints. This supports the decision to use separate-process protocols for
future portable runtime extensions rather than Go `plugin`.

## CLI UX guidance

- Command Line Interface Guidelines:
  https://clig.dev/

Review particularly:

- stdout versus stderr;
- human-readable errors and actionable hints;
- configuration precedence;
- non-interactive behavior;
- terminal/color behavior.

These are UX guidance, not a substitute for this repository's ADR/security
requirements.
