# Completion engine and shell adapters

## Core rule

Completion is a first-class subsystem of the CLI model, not a script generator
bolted onto help text.

```text
partial parser
     ↓
completion planner
     ↓
static hints + optional dynamic providers
     ↓
neutral CompletionResult
     ↓
shell adapter renderer/transport
```

## Neutral candidate model

A candidate should be able to represent at least:

- insertion value;
- display label when different;
- description/help;
- semantic kind;
- optional group;
- stable ID/dedup key;
- no-space/keep-order/filter directives where meaningful;
- optional shell-neutral style hint (adapters may ignore it).

Value hints should include at least:

- any path;
- file;
- directory;
- executable;
- command name;
- hostname;
- username;
- URL;
- email;
- enum/custom.

Adapters expose capabilities so unsupported hints degrade safely rather than
forcing shell-specific logic into the Spec.

## Static versus dynamic

The compiler should identify static data that can be embedded in generated
shell integration:

- command names/aliases;
- flags;
- descriptions;
- finite enums;
- static availability when known.

Dynamic providers are used only for runtime values such as profiles,
repositories or remote names.

Dynamic completion requirements:

- side-effect-free;
- no interactive prompts;
- context/cancellation aware;
- bounded result count;
- no secrets;
- no writes to stdout outside the protocol encoder;
- lazy domain/service initialization only if that specific provider needs it.

## Versioned protocol

Generated shell code and the binary must negotiate a versioned machine
protocol. Reserve protocol integer `1` for the first implementation.

Conceptual request:

```json
{
  "protocol": 1,
  "argv": ["ai-profile", "claude", "run", "ma"],
  "cursor_arg": 3,
  "cursor_offset": 2,
  "shell": "fish"
}
```

Conceptual response:

```json
{
  "protocol": 1,
  "candidates": [
    {"value": "main", "description": "Claude profile", "kind": "value"}
  ],
  "directive": {"files": false, "keep_order": true}
}
```

The exact wire format can differ, but it must be structured, versioned and able
to preserve spaces, tabs, Unicode and descriptions without newline/word-split
ambiguity.

Do not use `strings.Fields` or one-candidate-per-line as the canonical protocol.

## Self-correcting/version mismatch behavior

A generated adapter records the protocol/schema version it expects. If the
binary is incompatible, it must fail completion safely rather than emit garbage.

Where a shell supports cheap regeneration/sourcing, adapters may choose a
self-correcting strategy. Installed static files still require an explicit
`completion status`/upgrade path.

## Supported adapters

First target set:

- Fish;
- Nushell;
- Bash;
- Zsh;
- PowerShell.

A new adapter must implement the shared adapter contract and pass the conformance
suite before being listed as supported.

## Shell adapters

All installed scripts are deliberately **spec-agnostic**. They contain only the
tool name, completion protocol version and shell transport glue. Commands,
aliases, flags, enum/static values, availability and dynamic values always come
from the same runtime planner. This prevents a binary update from leaving a
second static representation of the CLI stale on disk.

Fish consumes the NUL-safe protocol and merges planner candidates with its
native file/directory/executable completion. Nushell 0.114+ uses command-wide
`@complete`; `commandline complete` is used only to satisfy neutral native
path/directory directives while planner candidates remain authoritative.
Bash, Zsh and PowerShell use their native completion idioms over the same
protocol. Quoting, cursor handling and installation paths remain adapter
responsibilities.

## User-facing commands

Every CLI should gain generated behavior equivalent to:

```text
<tool> completion list
<tool> completion generate <shell>
<tool> completion install [shell]
<tool> completion uninstall [shell]
<tool> completion status [shell]
<tool> completion doctor [shell]
```

The exact naming can be refined, but generation and install lifecycle must be
consistent across tools.

## Shell detection

Never treat `$SHELL` as proof of the currently running shell.

For `generate`, the shell is explicit. For `install`, `uninstall`, `status` and
`doctor`, omitting `shell` means "all supported adapters" in deterministic
order. `$SHELL` is not consulted. Install/uninstall-all performs a complete
safety preflight before the first mutation and rolls back prior mutations if a
later filesystem operation fails.

## Installation safety

Prefer shell-native user completion directories. Do not silently edit
`.bashrc`, `.zshrc`, `config.nu`, PowerShell profiles or equivalent.

If a shell requires a profile/config edit, require explicit opt-in and make the
change idempotent, reversible and testable in a synthetic HOME.

## Adapter conformance cases

Every adapter must cover at least:

- root and nested subcommands;
- aliases;
- short/long flags;
- flag value completion;
- enum/static values;
- file/directory hints;
- dynamic values;
- descriptions;
- spaces and Unicode;
- empty token;
- partial flag/command;
- `--` boundary;
- passthrough command;
- deprecated/hidden entries;
- unsupported command/platform behavior;
- protocol mismatch.
