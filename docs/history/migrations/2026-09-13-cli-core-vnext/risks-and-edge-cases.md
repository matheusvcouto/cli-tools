# Risks and edge cases

This file exists to prevent the implementation from succeeding on the happy path
while recreating hard CLI bugs later.

## Library/process boundary

The public core library must not call `os.Exit`. A test/embedded caller must be
able to execute:

```text
Run(ctx, argv, IO) -> Result/error/exit class
```

and decide process termination itself.

Likewise, core parsing should accept argv explicitly rather than reading
`os.Args` deep inside implementation code. Thin `cmd/<tool>/main.go` may adapt
real process state to the library.

Signal registration should have an explicit process-level helper/composition
boundary so embedding the core does not unexpectedly install global handlers.

## Negative values versus flags

Define and test how tokens such as `-1`, `-0.5` and values beginning with `-`
behave. Do not accidentally turn a valid typed numeric positional/value into an
unknown short flag, and do not create ambiguous heuristics that vary between
strict parse and completion.

Use explicit grammar/context and `--` where ambiguity cannot be resolved safely.

## Short-option clusters

If supporting `-abc`, define what happens when one member requires a value.
Do not infer behavior from whichever parser implementation is easiest. This is a
public grammar rule and needs tests/completion parity.

## Bool flags and negation

Do not auto-create `--no-foo` or implicit bool values unless declared by policy.
A switch, a bool-valued option and a tri-state setting are different semantics.
Avoid boolean creep when an enum/state is more precise.

## Global/inherited flags

Define scope precisely:

- may a global flag appear before and/or after subcommands?
- can a child shadow a parent flag?
- how are duplicate occurrences resolved?
- does completion offer the flag in every valid position?

The compiler should reject ambiguous shadowing unless a deliberate rule exists.

## Completion is triggered by typing

Pressing Tab is not consent for mutation, login, network access or destructive
probing.

Dynamic completion providers must not:

- create/update config;
- prompt;
- log in;
- write cache/state by default;
- run arbitrary project hooks;
- perform network calls implicitly.

If an external executable is required for completion, invoke it with the same
argv/env safety discipline as normal subprocess code and classify the provider
as an explicit runtime requirement.

## Completion performance

Shell completion can execute repeatedly for every keystroke. Prevent accidental
latency multiplication:

- embed static command/flag/enum data;
- initialize only the requested dynamic provider;
- bound candidate counts;
- avoid broad filesystem scans when shell-native file completion can do better;
- do not add concurrency/caching complexity until benchmarks show a need.

If caching is later added, make scope/TTL/invalidation explicit and never cache
secrets.

## Static availability in completion

Commands known at compile/runtime startup to be unsupported on the current
platform should normally not be suggested, or should be clearly marked according
to adapter capability. Do not perform expensive requirement probes merely to
render every Tab completion.

## Quoting and shell injection

Generated scripts must treat command names, descriptions and candidate values as
data. Every adapter owns correct escaping for its shell.

Test at minimum values containing:

- spaces;
- tabs;
- single/double quotes;
- backslashes;
- `$`, backticks and shell metacharacters;
- parentheses/brackets;
- leading `-`;
- newlines where the shell/protocol can represent them;
- Unicode.

Never generate code by interpolating unescaped user/domain candidate text.

## Unicode and non-UTF-8 paths

Go paths on Unix may contain byte sequences that are not valid UTF-8, while JSON
is a Unicode text protocol. Prefer shell-native path completion for generic
filesystem values so arbitrary filenames do not need to cross the dynamic JSON
protocol.

If the project later requires dynamic byte-exact path candidates, design an
explicit encoding/transport rather than assuming JSON strings round-trip every
filesystem name.

## Windows command-line behavior

Go receives already-parsed argv according to the Windows process runtime. The
core should parse the resulting argv, not reimplement `cmd.exe` or PowerShell
quoting rules for normal execution.

PowerShell completion generation/transport is an adapter concern and must be
runtime-tested on Windows before declaring support.

## Shell version drift

Generated integrations must target documented shell capabilities and have
adapter tests against supported versions where practical. Keep shell-specific
workarounds private to adapters. Do not leak version checks into command/domain
Specs.

## Help and machine output

Human wrapping/color/terminal width must never alter JSON/raw protocol output.
Diagnostics go to stderr unless a machine diagnostic API explicitly requests
structured output.

## Broken pipes

Commands piped to consumers that exit early can receive broken-pipe errors.
Define a consistent process-level policy instead of treating every EPIPE as an
internal crash. Test where behavior matters, especially generated large output.

## Environment variable binding

Do not automatically derive environment variable names from every flag. Env
bindings should be explicit to avoid collisions, surprising configuration and
accidental secret exposure.

## Response files / `@file`

Do not add response-file expansion in the first milestone without a real use
case. If needed later, implement it as an explicit pre-parse token-expansion
extension with defined encoding, recursion limit, quoting and security rules.
Do not bury `@file` magic inside the core parser.

## Localization

Diagnostics have stable codes separate from human text. This keeps future
localization/custom renderers possible without changing parser/control-flow
logic. Do not make localized message text part of program logic.

## Generated artifact drift

Schema, contracts, docs and completions must be reproducible. CI should compare
generated content with committed locks/goldens rather than trusting agents to
remember every file.

## Public API growth

The easiest way to make future extension hard is to export internal machinery
too early. Keep public contracts capability-oriented and place fast-changing
compiler/parser structures under `cli/internal/`.
