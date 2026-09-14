# Target architecture

## 1. Source of truth

Each executable constructs one typed declarative `cli.App` specification. The
Spec is the only authoritative command tree.

The same compiled graph drives:

- parsing and binding;
- validation/constraints;
- command dispatch;
- help/usage;
- suggestions;
- static and dynamic completion;
- shell integration generation;
- availability/preflight;
- introspection/schema;
- contract lock generation;
- reference docs/man pages.

Any design that requires a second list of commands, flags, argument names or
aliases is rejected.

## 2. Package boundaries

Target layout:

```text
cli/                            # public API only
├── app.go
├── command.go
├── argument.go
├── flag.go
├── value.go
├── codec.go
├── constraint.go
├── module.go
├── invocation.go
├── diagnostic.go
├── completion.go
├── requirement.go
├── interaction.go
└── ...small stable contracts...

cli/internal/                   # implementation, freely evolvable
├── model/
├── compile/
├── parse/
├── runtime/
├── complete/
├── render/
├── schema/
└── shell/
    ├── fish/
    ├── nushell/
    ├── bash/
    ├── zsh/
    └── powershell/

internal/<tool>/cli/            # tool-specific Spec/modules/handlers
internal/<tool>/...             # domain/application/platform code
cmd/<tool>/main.go              # thin composition root
```

The exported `cli/` package must expose concepts, not internal data structures.
Do not leak parser nodes or renderer internals into the public API.

The library accepts `context`, argv and IO explicitly and returns a result/error.
It must not call `os.Exit`, implicitly read `os.Args`, or install process-global
signal handlers from deep library code. Thin entrypoints/process helpers own
those adaptations so the core remains embeddable and hermetically testable.

## 3. Compile, then run

Specs are mutable only during construction. `cli.Compile(spec)` validates and
normalizes them into an immutable `CompiledApp`.

Compilation should detect programming errors before an invocation exists:

- duplicate command/alias/flag/ID;
- short-option conflicts;
- illegal variadic position;
- unreachable/conflicting constraints;
- invalid inherited/global flag combinations;
- unknown capability/requirement reference;
- reserved internal namespace use;
- shell-incompatible declarations that cannot degrade safely;
- invalid deprecation/replacement reference.

Precompute child/flag/ID lookups and deterministic ordering here. Execution and
completion should not repeatedly normalize the graph.

## 4. Stable IDs

Visible names are UX. Internal IDs are contract identity.

Example:

```text
ID: profiles.delete
visible path: ai-profile claude delete
```

Rules:

- IDs are explicit and unique;
- renaming a visible command need not change its ID;
- IDs are emitted into schema/contract files;
- contract diff operates primarily on IDs, then names;
- IDs are not silently generated from Go source locations.

## 5. Composition and modules

Reusable features are values, not global registrations.

A module may contribute:

- commands/subcommands;
- shared flags;
- codecs/value types;
- completers;
- requirements/capabilities;
- documentation metadata.

Composition is explicit at the root. Compiler detects conflicts.

No `init()` command registration and no package-global mutable registry.

## 6. Extension points

First-class extension interfaces/descriptors:

- `Codec[T]` / typed value;
- constraint;
- command module;
- completion provider;
- requirement/probe;
- platform capability provider;
- middleware;
- output/diagnostic renderer;
- shell adapter;
- config provider;
- interaction provider.

New behavior should normally enter through one of these boundaries rather than
adding special cases to the central parser.

## 7. Lifecycle

Conceptual execution lifecycle:

```text
argv
  ↓
strict parse
  ↓
typed bind
  ↓
syntactic + cross-field validation
  ↓
value resolution (flag/env/config/default)
  ↓
static availability
  ↓
runtime requirements/preflight
  ↓
middleware
  ↓
handler
  ↓
result / diagnostic renderer
```

No domain service is instantiated merely to print help, version, schema or
generate a completion adapter script.

## 8. Reserved internal surface

Reserve a namespace such as `__cli` for machine-only endpoints. Human commands
cannot claim it.

Potential internal endpoints:

```text
__cli schema
__cli complete
__cli contract
```

They are protocol surfaces, not undocumented ad-hoc debug commands. Their
formats are versioned independently.

## 9. Determinism

The following outputs must be deterministic for the same compiled Spec:

- help ordering;
- generated completion scripts/modules;
- schema JSON;
- contract JSON;
- docs/man pages.

Never rely on Go map iteration order. Stable output makes reviews, caching,
contract diff and agent maintenance reliable.

## 10. No forced framework dependency

The target semantics are broader than simply wrapping Cobra/Kong/pflag. Use
those projects as behavioral references. If implementation later shows that a
small mature library eliminates a risky parser primitive without fighting the
model, evaluate it under D003 rather than forbidding it in advance.
