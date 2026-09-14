# Public API and data model

This is the intended shape, not a frozen code signature. Preserve the concepts
while allowing the implementing agent to choose idiomatic Go names.

## App

An app declares at least:

- stable app ID/name;
- summary/description;
- root command;
- product metadata/version provider;
- enabled built-ins/features;
- modules;
- config/interaction/output policy where applicable.

The root command is a normal command node, not a parser special case.

## Command

A command can declare:

- stable ID;
- visible name;
- aliases;
- summary/long description;
- examples;
- positional args;
- local/global flags;
- child commands;
- constraints;
- availability/requirements;
- output formats;
- handler;
- hidden/experimental/deprecated metadata;
- replacement/removal version.

Commands are composable values.

## Typed values

Do not make reflection/struct tags the primary model. Prefer explicit generic
value descriptors/codecs.

Baseline value families:

- string;
- bool;
- signed/unsigned integer;
- float;
- duration;
- enum;
- path/any path;
- file path;
- directory path;
- executable path;
- URL;
- hostname;
- username;
- email;
- opaque custom value.

A custom `Codec[T]` should be able to provide:

- parse from CLI text;
- display/format;
- validation;
- semantic type name;
- shell value hint;
- static possible values if finite;
- optional dynamic completer;
- schema representation;
- sensitive/redaction metadata if appropriate.

The Go type alone is not enough to infer shell semantics. `string` may represent
an arbitrary string, profile name, hostname or path. Require semantic metadata.

## Args and flags

Support deliberately:

- required/optional positional values;
- bounded arity;
- variadic/trailing args;
- literal passthrough after `--`;
- command-wrapper trailing argv;
- long flags;
- short flags;
- `--name=value`;
- repeat/append/count actions;
- bool switches;
- inherited/global flags;
- interspersed flags when enabled by grammar policy.

Avoid ambiguous abbreviations. Do not accept prefixes of long flags unless a
future explicit policy is added.

## Constraints

First-class constraints should cover common grammar without handler boilerplate:

- conflicts with;
- requires;
- exactly one of;
- at least one of;
- all or none;
- dependent value validation;
- min/max occurrences.

Compiler validates the constraint graph where possible. Runtime validates values.

## Invocation

After successful parse/bind/resolution, the handler receives a typed Invocation
rather than raw argv parsing responsibilities.

Invocation should expose:

- command path/IDs;
- typed arg/flag values;
- source/provenance for resolved config values;
- context/cancellation;
- stdin/stdout/stderr/terminal abstractions;
- explicitly requested output format;
- services/capabilities supplied by the composition root.

Do not turn Invocation into a global service locator. Domain dependencies should
remain explicit and narrow.

## Config provenance

The core should be able to resolve a declared setting through ordered providers:

```text
explicit CLI > environment > project config > user config > default
```

The exact providers are configurable per tool. Preserve source metadata:

```text
value = 30s
source = env
source_name = TOOL_TIMEOUT
```

This enables future `config explain` behavior without rebuilding precedence
logic in each CLI.

## Sensitive values

Metadata `Sensitive`/equivalent must ensure values are:

- redacted from diagnostics/debug views;
- excluded from completion candidates;
- not embedded as resolved values in schema/contract output;
- not accidentally logged by generic middleware.

The core should discourage secrets directly on command lines because process
lists/history may expose them.

## Interaction

Provide a narrow interaction abstraction for:

- confirm;
- text prompt;
- secret prompt.

Rules:

- prompts require interactive terminal unless explicitly supported otherwise;
- non-interactive use must fail clearly rather than hang;
- destructive confirmation policy is consistent;
- commands may opt into standardized `--yes` semantics;
- prompts/diagnostics do not pollute machine stdout.

## Output contracts

Commands declare supported output forms only when needed:

- human;
- JSON;
- NDJSON;
- raw/protocol passthrough.

Do not force every command to have JSON. When a machine format is a public API,
its schema/version is explicit.

## Diagnostics

Diagnostics are structured data with at least:

- stable code;
- category/kind;
- human message;
- optional hint;
- command path;
- related arg/flag ID;
- exit class/status;
- wrapped cause;
- redaction metadata.

Baseline codes should cover:

- unknown command;
- unknown flag;
- missing value/argument;
- invalid typed value;
- conflict/constraint failure;
- unavailable capability/platform;
- missing runtime requirement;
- non-interactive requirement;
- internal error.

Human and JSON renderers consume the same Diagnostic.

## Exit status policy

Keep process statuses small and stable rather than encoding every error kind:

- `0`: success;
- `1`: domain/execution failure;
- `2`: usage/parse/validation failure;
- `3`: unavailable capability/requirement/preflight failure.

A transparent wrapper command may preserve a child process exit status when its
public contract requires it.
