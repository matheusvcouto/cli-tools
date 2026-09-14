# Current state and migration pressure

This file records the repository state observed before the CLI Core migration.
It is reference material, not the target design.

## Existing entrypoints and CLI code

Relevant files:

- `cmd/ai-profile/main.go`
- `internal/aiprofile/app.go`
- `cmd/repo-zip/main.go`
- `internal/repozip/app.go`
- `internal/repozip/options.go`
- `internal/cliapp/cliapp.go`
- `internal/version/version.go`
- `integration/e2e_unix_test.go`

## `internal/cliapp`

It currently owns only:

- rendering `error: ...`;
- `ExitCoder` lookup;
- conversion from error to process exit code.

The former D014 intentionally prevented it from becoming a command framework.
That decision was removed from the active ADR and replaced by D019.

## `ai-profile`

The current implementation duplicates command knowledge across multiple places:

- routing/switches;
- usage strings and `printHelp`;
- shell completion scripts;
- internal `__complete` endpoint;
- hard-coded action lists.

The current completion implementation supports Bash, Fish and Zsh and emits
newline-oriented candidates. Dynamic profile/template completion touches the
service layer directly. Static invocations require special handling so that
help/version/completion can work without a valid HOME/store.

Migration consequence: command definition, help and completion must collapse
into one Spec. The old `__complete tools/actions/profiles/templates` protocol
should be deleted after the new completion protocol is proven.

## `repo-zip`

`repo-zip` has its own argument parser and help. It accepts interspersed options
and currently overloads `--version`:

- `repo-zip --version` means executable version;
- `-v/--version TEXT` is also an alias of `--suffix` in archive options.

This ambiguity should be removed during the breaking migration. Reserve
`--version` for product version and keep `--suffix` for archive naming.

## Platform model

The repository already has a good principle: domain logic is portable, while
semantics that truly differ by platform live behind narrow capabilities/build
specific implementations. Existing examples include:

- `internal/filelock/`;
- `internal/fscommit/`;
- `internal/aiprofile/platform/`;
- `internal/repozip/publish_*.go`.

The CLI Core should generalize *declaration and preflight* of platform/runtime
requirements; it should not move these domain/platform implementations into the
CLI package.

## Versioning/release today

Current state:

- one Git tag/version for the entire suite;
- `internal/version.Version` is injected into every binary;
- `CHANGELOG.md` is the release-notes source;
- release tooling discovers `cmd/*` dynamically;
- smoke tests expect all binaries to print the same version.

Target state is defined in `versioning-and-release.md`; migration must update the
release tooling and smoke tests atomically.

## Important constraints to preserve

- no user state in tests;
- no real Claude/Codex/credential access in tests;
- stdout is reserved for protocol/payload where required (especially ACP);
- subprocess argv is never constructed through a shell;
- runtime support claims require native evidence;
- security-sensitive filesystem invariants stay in their existing focused
  packages rather than being swallowed by the CLI Core.
