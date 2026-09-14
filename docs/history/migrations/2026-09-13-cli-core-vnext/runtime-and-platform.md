# Runtime, availability, capabilities and requirements

## Why capability-first

Do not encode behavior as scattered `runtime.GOOS == ...` checks or a pile of
booleans such as `SupportsWindows`.

Commands declare the semantic capability they require. Platform/runtime
providers report which capabilities are available.

Examples:

```text
ProcessReplace
AtomicReplace
NoClobberPublish
FileLock
InteractiveTTY
NetworkAccess
GitExecutable
```

Use enums/IDs/typed capability descriptors rather than boolean creep.

## Availability versus requirement

Keep two concepts separate.

### Availability

Known without external I/O, e.g.:

- compiled OS/arch;
- backend compiled in;
- feature included in build;
- command intentionally unavailable on a target.

### Runtime requirement/probe

May require safe inspection, e.g.:

- executable exists in PATH;
- writable directory is available;
- terminal is interactive;
- Git satisfies a minimum behavior/version;
- required local service/socket exists.

## Preflight order

A valid command should be rejected before side effects if requirements cannot be
satisfied:

```text
parse -> bind -> validate -> resolve -> availability -> requirements -> handler
```

A Windows build of a command requiring an unimplemented primitive must return a
structured unsupported-capability diagnostic before touching stores/files.

## Doctor

The core provides the health-check framework; tools register relevant checks.

Expected behavior:

- human table/list;
- optional structured JSON;
- no destructive probes;
- no credential contents;
- distinguish unsupported, missing, warning and healthy;
- deterministic exit semantics.

## Platform implementations remain outside the core

The CLI package only models requirements. Existing implementations such as
`filelock`, `fscommit`, process replacement and repo publication stay in focused
packages or tool platform layers.

Do not create a giant `cli/platform` that absorbs unrelated filesystem/process
business invariants.

## Config and environment

Core may standardize declaration/provenance, but tool-specific config schema and
auth logic remain with the tool/domain.

Configuration providers must be lazy. Help/version/completion script generation must not
open HOME, stores or project config.

## External extensions/plugins

Do not use Go `plugin` as the portable extension mechanism. It is not supported
on Windows and has deployment/race-detector/toolchain coupling drawbacks.

If runtime plugins become a real requirement, use:

```text
separate process + explicit discovery policy + versioned protocol + handshake
```

Prepare extension points now, but do not implement a plugin system in the first
CLI Core milestone without a concrete consumer.

## Future remote/network requirements

The suite currently avoids network runtime dependencies. The core may model a
`NetworkAccess` requirement but must not add telemetry, update checks or network
calls implicitly.
