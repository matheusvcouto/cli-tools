# Windows support plan

Status: Windows runtime implemented in source; planned public Go module migration to `/v2` (Go 1.27.1). Native CI observation remains the support-promotion gate; legacy explicit ACLs remain a documented migration hardening risk in `CODE_REVIEW.md`. Cross-compilation is never runtime evidence.

## Goal

Make the existing `ai-profile` and `repo-zip` production paths available on Windows without moving platform rules into the domain, weakening filesystem guarantees, introducing shell command execution, or adapting production code to the development sandbox.

## Baseline

Already portable:

- CLI parsing/help/schema/contracts/version/completion generation;
- JSON/profile domain logic;
- Git selection and ZIP creation/verification;
- Windows amd64/arm64 cross-compilation;
- PowerShell completion adapter.

Initial Windows runtime gaps addressed by this implementation:

1. exclusive profile-store file locking;
2. safe same-root replace used by atomic profile writes;
3. `ai-profile run/acp` process execution and child exit propagation;
4. `repo-zip` final force/no-clobber publication;
5. native Windows CI evidence;
6. Windows release artifacts and native release smoke tests;
7. Windows completion update/replace semantics;
8. Windows case-insensitive environment-key isolation;
9. secure support for official npm `.cmd` shims without invoking `cmd.exe`.

## Implementation decisions

### 1. File locking

Implement `internal/filelock/lock_windows.go` with Win32 `LockFileEx`/`UnlockFileEx` on the already-open lock file handle.

Requirements:

- exclusive, blocking lock;
- cover a byte range extending beyond EOF so a zero-byte lock file is valid;
- validate that the opened object is a regular file before locking, matching Unix behavior;
- unlock before closing; always close the owned file even when unlock fails;
- no lock-file deletion as an unlock mechanism.

The implementation remains Windows-only behind its existing build tag. No domain `GOOS` branch is added.

Official basis:

- https://learn.microsoft.com/windows/win32/api/fileapi/nf-fileapi-lockfileex
- https://learn.microsoft.com/windows/win32/fileio/locking-and-unlocking-byte-ranges-in-files

### 2. Safe profile-store replace

Implement `internal/fscommit/commit_windows.go` through the already-open `safefs.Root` and its `Rename` operation. Official release builds use Go 1.27.1, where Windows `os.Root.Rename` is resolved relative to open directory handles and Go's Windows backend performs handle-relative native rename with replace semantics.

Requirements:

- source and destination remain relative to the same safety root;
- no destination pre-delete;
- no lexical containment fallback in the production release path;
- same-filesystem temporary file remains mandatory;
- failure leaves an explicit error rather than attempting an unsafe fallback.

Reference implementation/documents:

- https://pkg.go.dev/os#Root.Rename
- https://go.dev/src/internal/syscall/windows/at_windows.go
- https://learn.microsoft.com/windows/win32/api/winbase/ns-winbase-file_rename_info
- https://learn.microsoft.com/windows/win32/api/fileapi/nf-fileapi-setfileinformationbyhandle

### 3. `ai-profile run/acp`

Windows has no Unix `exec(2)` equivalent. Implement the closest faithful Windows runtime contract without a shell:

- resolve the executable with `os/exec` host rules;
- start the child directly with argv elements, never `cmd.exe`/PowerShell;
- pass the isolated environment exactly;
- connect stdin/stdout/stderr directly to the child;
- wait for the child;
- success returns normally;
- non-zero child exit is represented by a dedicated adapter error carrying the exact exit code;
- the `ai-profile` composition root recognizes only that dedicated child-exit error and exits with the same code without rendering a wrapper diagnostic;
- start/configuration errors remain ordinary diagnostics;
- the child is created with `CREATE_SUSPENDED`, assigned to a private Windows Job Object, then resumed, so it cannot create an uncontained descendant before assignment;
- context cancellation terminates the direct child through `exec.CommandContext`, while `TerminateJobObject` cleans the associated descendant tree;
- `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` provides final cleanup if the wrapper itself exits unexpectedly after assignment.

This preserves ACP stdout: the wrapper emits nothing on a normal child execution path.

Official basis:

- https://pkg.go.dev/os/exec#CommandContext
- https://pkg.go.dev/os/exec#LookPath
- https://learn.microsoft.com/windows/win32/procthread/process-creation-flags
- https://learn.microsoft.com/windows/win32/api/jobapi2/nf-jobapi2-assignprocesstojobobject
- https://learn.microsoft.com/windows/win32/procthread/job-objects
- https://learn.microsoft.com/windows/win32/api/jobapi2/nf-jobapi2-terminatejobobject
- https://learn.microsoft.com/windows/win32/api/winnt/ns-winnt-jobobject_extended_limit_information

Important Windows argv note: Go quotes argv for native Windows processes according to Windows conventions. No manual command-line concatenation is allowed.

### 4. `repo-zip` publication

Implement the existing two distinct publication semantics on Windows through the confined `safefs.Root`:

- `--force`: `Root.Rename(temp, final)`; never delete `final` first;
- default/no-clobber: `Root.Link(temp, final)` followed by removal of the temporary name only after link success.

The no-clobber path intentionally fails closed when the underlying filesystem cannot provide the required hard-link operation. It must never degrade to "check then rename", because that would introduce a TOCTOU overwrite race.

Go 1.27's Windows `os.Root.Link` uses handle-relative native link creation with replace disabled. Native Windows tests must verify the expected behavior on the CI filesystem.

Reference:

- https://go.dev/src/internal/syscall/windows/at_windows.go
- https://pkg.go.dev/os#Root.Link

### 5. Native Windows tests

Add real Windows-only tests for the OS-specific contracts. They are intended for the pinned Windows CI runners (`windows-2025` and `windows-11-vs2026-arm`); they are not claimed as executed in non-Windows environments.

Required evidence:

- file lock: a second process cannot acquire the same exclusive range until release;
- file commit: destination is replaced, source disappears, and destination is never pre-deleted by our code;
- process runner: literal argv, env and stdio forwarding; child success; exact non-zero exit propagation; cancellation; descendant-tree termination through a real Job Object;
- repo publication: no-force refuses existing destination; force replaces; successful no-force publishes valid archive.

Existing end-to-end profile and repo tests then run natively too.

### 6. CI

Run the complete published target matrix natively with Go 1.27.1: Linux, macOS and Windows on amd64 and arm64. Keep cross-build as separate compile-only evidence.

Native Windows CI must run at least:

- `go test ./...`;
- `go vet ./...`;
- shuffled repeated tests;
- Windows-specific runtime tests;
- PowerShell native completion coverage where available.

Race remains a separate capability and is run only where the GitHub runner/toolchain supports it reliably; lack of race support must not be disguised as runtime support.

### 7. Release

Add:

- `windows/amd64`;
- `windows/arm64`;

to release targets. Windows artifacts are `.zip` files containing `.exe` binaries.

The release artifacts are built exactly once. Native smoke jobs for all six published OS/architecture targets download that immutable bundle, verify the archive checksum and exact expected CLI set, and execute the binaries from the exact archive that will later be published. Windows x64 and ARM64 therefore verify:

- `<tool>.exe --version`;
- `<tool>.exe version --json` and suite version.

The publish job downloads the same bundle, verifies the exact six-archive set plus `SHA256SUMS`, attests those checksums, and uploads the same files to GitHub Release without rebuilding.

### 8. Additional Windows hardening

During implementation review, three non-stub Windows gaps were found and included in scope:

- completion updates use the platform replace adapter instead of raw `os.Rename`;
- profile credential isolation and `repo-zip` Git override sanitization normalize environment keys according to Windows case-insensitive semantics;
- known npm/pnpm command shims are validated against installed package metadata and executed through `node.exe` directly, preserving argv without a shell. Unknown `.cmd`/`.bat`/PowerShell wrappers fail closed;
- profile roots use a protected inheritable Windows DACL instead of treating `chmod 0700/0600` as access control;
- `repo-zip --git` applies a protected DACL directly to the open temporary bundle handle on Windows before repository data is written.

Completion path selection was also moved out of common lifecycle logic into macOS/Linux/Windows path adapters.

### 9. Documentation and support state

Update active ADR/platform/release/testing docs to describe actual Windows semantics.

Do **not** label Windows `supported` merely because the implementation exists or cross-build succeeds. Under D011, `supported` requires native runtime evidence. Until a native CI run is observed, describe the code as implemented with the native gate installed, and record the native execution as pending/not executed in this environment.

## Errors to avoid

- treating `GOOS=windows go build` as runtime proof;
- deleting an existing destination before rename;
- implementing no-clobber as `stat -> rename`;
- replacing confined root operations with raw absolute-path mutations;
- introducing `cmd.exe /c`, PowerShell, or a shell string for child execution;
- manually concatenating Windows command lines;
- printing wrapper diagnostics into ACP stdout;
- swallowing the child exit code;
- starting the Windows child normally and assigning it to a Job Object afterward, leaving a process-spawn race before containment;
- killing only the direct Windows process and leaving Node/ACP descendants running;
- changing production behavior solely to satisfy the current sandbox;
- marking unexecuted Windows tests as passed;
- adding mocks as evidence for native Win32 semantics.

## Completion gates

- [x] G1 Windows locking implemented and statically reviewed against Win32 docs.
- [x] G2 Windows safe replace implemented and reviewed against Go 1.27 Windows source + Win32 rename semantics.
- [x] G3 Windows process runner implemented with exact child-exit handling, shell-free argv, suspended-before-assignment Job Object containment and descendant cleanup.
- [x] G4 Windows repo publication implemented with distinct force/no-clobber semantics.
- [x] G5 Native Windows tests added for all four primitive groups plus portable E2E.
- [x] G6 Native Windows x64/ARM64 CI configured.
- [x] G7 Windows amd64/arm64 release artifacts and native release smoke configured for both architectures.
- [x] G8 Completion paths/replacement, Windows environment semantics and npm shim handling hardened.
- [x] G9 Active platform/release/testing documentation synchronized.
- [x] G10 Static review completed and historical cross-build evidence/current native execution boundaries recorded honestly in `VALIDATION.md`.
