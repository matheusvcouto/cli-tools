# Parsing and runtime contract

## One grammar, two parse modes

Implement one grammar engine with two entry modes:

### Strict

Used for execution. Missing/incomplete tokens are errors.

### Partial

Used for completion. The cursor can be in an incomplete state:

- partial command;
- partial flag name;
- flag awaiting a value;
- incomplete positional;
- empty current token;
- cursor after `--`;
- quoted/space-containing logical argument as provided by the shell adapter.

Do not create a second completion parser with subtly different rules.

## Parse result should preserve context

The internal parse result should identify enough state for diagnostics and
completion without reparsing text:

- matched command path;
- consumed args/flags;
- expected next entities;
- current arg/flag value slot;
- whether option parsing ended via `--`;
- passthrough/trailing boundary;
- token index/cursor context;
- parse ambiguity/incompleteness reason.

## OS argv versus shell syntax

Execution receives `[]string`; do not attempt to reconstruct shell quoting.
Completion adapters may need cursor/token metadata specific to the shell. Keep
that in the adapter request boundary, not in domain/parser semantics.

## Wrapper/passthrough commands

Support wrappers like `ai-profile ... run <profile> [args...]` explicitly.
Trailing child argv is opaque after the declared boundary. The core must not
reinterpret child flags as parent flags.

This should be a modeled argument mode, not a handler workaround.

## Help/version precedence

Define reserved built-ins consistently. Suggested policy:

- `-h/--help` is recognized for the active command unless after opaque
  passthrough boundary;
- `--version` at root is reserved for executable product version;
- `version` may be a generated root command;
- tool-specific meanings cannot reuse root `--version` with a value.

This intentionally breaks `repo-zip --version TEXT`; use `--suffix` instead.

## Suggestions

Unknown command/flag diagnostics may provide bounded typo suggestions using a
small deterministic edit-distance/prefix heuristic. Never auto-execute a
suggested command and never make suggestion generation expensive on large
dynamic domains.

## Context and signals

The composition root creates a cancelable `context.Context` tied to supported
termination signals. Long-running handlers, probes and dynamic completers accept
context.

Signal handling should be platform-aware and tested where behavior differs.
Do not create background goroutines for ordinary parsing/completion.

## Streams

Contract:

```text
stdout -> requested payload/protocol
stderr -> diagnostics, warnings, progress
stdin  -> command input only when declared
```

Machine output must remain parseable. ACP/raw protocol commands can reserve
stdout entirely for the child/protocol.

## Terminal capabilities

A narrow terminal abstraction can expose:

- whether stdin/stdout/stderr are TTYs;
- width when known;
- color capability/policy;
- interactive availability.

Respect plain/non-interactive environments and `NO_COLOR`. Rendering must not
be required for core parsing/execution correctness.

## Middleware

Middleware is for true cross-cutting concerns such as:

- debug timing;
- trace/audit hooks;
- standardized logging/redaction;
- future retry policy where semantically safe.

Order is explicit and deterministic. Middleware cannot mutate the compiled
command graph.

## Built-ins

The core can provide opt-in generated commands/features:

- help;
- version;
- completion;
- doctor;
- schema/introspection if exposed to humans.

Machine-only endpoints remain under the reserved namespace.
