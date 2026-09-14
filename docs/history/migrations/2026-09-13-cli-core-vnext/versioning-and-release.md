# Versioning, contracts and release

## Three independent version domains

### 1. Suite/module version

Git tag `vX.Y.Z` versions:

- the Go module `github.com/matheusvcouto/cli-tools`;
- public package `cli/` for external Go consumers;
- the GitHub Release/archive set.

This version must follow Go module/SemVer compatibility rules. If a future stable
major version of the public Go API becomes `v2+`, the module path implications
must be handled correctly.

### 2. Tool product version

Each CLI has its own manifest:

```text
cmd/<tool>/tool.json
```

Recommended minimal shape:

```json
{
  "schema_version": 1,
  "name": "ai-profile",
  "version": "0.1.0",
  "stability": "beta"
}
```

This is the version users see for that tool.

### 3. Protocol/schema format versions

Use independent integer versions for machine formats, for example:

- completion protocol;
- CLI schema format;
- contract-lock format;
- machine output schemas when public.

Do not bump a product SemVer merely because an internal schema integer changes
compatibly, and do not force these numbers to match.

## Change records instead of manual bump-per-edit

Do not require an agent to edit `0.1.3 -> 0.1.4` on every implementation commit.
That creates artificial versions during multi-commit work.

Add a root directory:

```text
changes/
```

Each meaningful change adds a small structured record (JSON is preferred for
simple stdlib parsing; YAML/TOML would add parser concerns without need).

Example:

```json
{
  "schema_version": 1,
  "changes": [
    {
      "component": "ai-profile",
      "impact": "minor",
      "breaking": false,
      "summary": "Generate Nushell completions from the shared CLI spec"
    }
  ]
}
```

Allowed impact:

- `none`;
- `patch`;
- `minor`;
- `major`.

`none` still requires justification when code under a product/core contract
changed.

## Agent rule

Changes under these scopes trigger impact evaluation:

```text
cli/                     -> module/core + every affected CLI
cmd/<tool>/              -> that tool
internal/<tool>/         -> that tool when behavior changes
cli contract/schema      -> affected tool(s) + protocol/schema impact
release tooling          -> suite/module release process
```

CI should reject relevant changes without a change record once the mechanism is
fully implemented.

## Pre-1.0 policy

While a product/tool or module is `<1.0`, breaking changes are allowed when
intentional and explicitly marked. The release tooling may map a breaking
pre-1.0 change to a minor bump according to the project's chosen policy, but the
record must preserve `breaking: true`.

After `1.0`, normal SemVer applies: breaking -> major, compatible feature ->
minor, compatible fix -> patch.

## CLI contract lock

Generate per tool:

```text
cmd/<tool>/cli.contract.json
```

It is derived from the compiled Spec but committed as a reviewable lock.

Include public CLI facts such as:

- command/arg/flag stable IDs and names;
- aliases;
- arity/type/value hints;
- constraints;
- defaults that are not sensitive;
- deprecation/hidden/stability metadata;
- output formats;
- availability/requirements declarations;
- relevant schema/protocol versions.

Do not serialize handler implementation details or resolved secrets/config.

## Contract diff

A repository tool should compare generated/current contract and classify at
least:

- added command/optional flag -> additive;
- removed command/flag -> breaking;
- required arg added -> breaking;
- type/arity changed -> usually breaking;
- alias removed -> breaking unless documented otherwise;
- deprecation added -> compatible warning;
- default changed -> semantic change requiring explicit impact review;
- requirement/platform support changed -> explicit review;
- output machine schema changed -> schema-specific compatibility review.

Automation can classify syntax-level changes, but semantic compatibility still
requires the agent to declare impact.

## Version command

Generated behavior:

```text
<tool> --version
<tool> version
<tool> version --json
```

Human output should center the individual product version, for example:

```text
ai-profile 0.3.0
```

Structured output may include:

- tool name/version;
- suite/module version;
- commit/revision and VCS dirty flag when available;
- Go version;
- OS/arch;
- CLI schema version;
- completion protocol version.

Use `runtime/debug.ReadBuildInfo` where appropriate rather than inventing
non-reproducible build timestamps.

## Release preparation

Target release flow:

```text
change records
    ↓
release prepare
    ├── compute tool bumps
    ├── compute/validate suite module bump
    ├── update tool.json
    ├── update per-tool changelog/history
    ├── update suite release notes
    ├── regenerate contract locks
    └── consume/archive change records
```

The exact suite version bump may be explicit when the public `cli/` Go API is
involved; tooling must not incorrectly infer Go API compatibility from CLI
contracts alone.

## Artifacts

Release archives may evolve to include generated integrations:

```text
cli-tools_<suite-version>_<os>_<arch>/
├── bin/
│   ├── ai-profile
│   └── repo-zip
└── share/
    ├── completions/
    │   ├── fish/
    │   ├── nushell/
    │   ├── bash/
    │   ├── zsh/
    │   └── powershell/
    └── man/man1/
```

The runtime `completion generate/install` path still exists; shipping generated
files is an additional distribution convenience.

## Migration rule

Do not enforce change records or individual version smoke tests until the
supporting tooling lands. Update `.github/workflows/release.yml`, `tools/release`,
`docs/release.md`, `AGENTS.md` and smoke tests in one coherent release phase.
