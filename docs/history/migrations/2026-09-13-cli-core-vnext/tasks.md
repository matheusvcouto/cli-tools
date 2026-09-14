# Implementation task map

This is an ordered work graph, not a suggestion to implement only one bullet per
turn. Complete as much as can be done safely, keeping commits coherent.

## A. Core model/compiler

- [x] Create public `cli/` API surface with package docs.
- [x] Define App, Command, Arg, Flag, Value/Codec and stable ID rules.
- [x] Define module/composition model without global registration.
- [x] Define deprecation/hidden/experimental metadata.
- [x] Define output/interaction/requirement extension contracts.
- [x] Implement compiler normalization and deterministic ordering.
- [x] Implement all compile-time invariant checks.
- [x] Make compiled graph immutable/private.
- [x] Add synthetic fixture CLIs and compiler tests.

## B. Parsing and diagnostics

- [x] Define formal grammar/policies before parser implementation.
- [x] Implement strict parse.
- [x] Implement partial parse using the same grammar/state machine.
- [x] Implement typed bind/codecs.
- [x] Implement constraints.
- [x] Implement structured diagnostics and exit classes.
- [x] Implement bounded typo suggestions.
- [x] Implement generated help/usage.
- [x] Fuzz strict/partial parsing.

## C. Runtime

- [x] Define Invocation/context/stdio boundary.
- [x] Add signal cancellation at composition root/runtime helper.
- [x] Implement lazy dependency/service hooks.
- [x] Implement availability/capability model.
- [x] Implement runtime requirements/probes.
- [x] Implement terminal capability policy.
- [x] Implement interaction/confirmation abstraction.
- [x] Implement deterministic middleware pipeline.
- [x] Implement doctor framework.

## D. Completion engine

- [x] Define neutral candidate/directive model.
- [x] Define static value hints.
- [x] Define dynamic completer contract.
- [x] Implement completion planner on partial parse result.
- [x] Define protocol v1 request/response with Unicode/space safety.
- [x] Implement reserved `__cli complete` endpoint.
- [x] Add protocol mismatch behavior.
- [x] Add dynamic completion safety tests.

## E. Shell adapters

- [x] Define adapter capability interface.
- [x] Build shared conformance suite.
- [x] Implement Fish adapter + native E2E.
- [x] Implement Nushell `extern`/custom completion adapter + native E2E.
- [x] Implement Bash adapter + E2E where available.
- [x] Implement Zsh adapter + E2E where available.
- [x] Implement PowerShell adapter + Windows/appropriate E2E.
- [x] Implement generate/install/uninstall/status/doctor UX.
- [x] Test installers only against synthetic HOME/config roots.

## F. Introspection and generated docs

- [x] Define CLI schema format v1.
- [x] Implement deterministic schema export.
- [x] Define contract-lock format v1.
- [x] Implement `cli.contract.json` generation.
- [x] Implement contract diff classifications.
- [x] Generate Markdown command reference.
- [x] Generate man pages.
- [x] Add golden tests for all generated artifacts.

## G. Versioning/release

- [x] Define and validate `tool.json` schema v1.
- [x] Add manifests for `ai-profile` and `repo-zip`.
- [x] Define `changes/*.json` format and validator.
- [x] Add change-impact coverage check for agent/CI changes.
- [x] Implement release prepare/consume flow.
- [x] Separate suite/module version from tool product versions.
- [x] Generate version metadata from manifests/build info.
- [x] Update release tooling and workflow atomically.
- [x] Update changelog/release-note generation.
- [x] Update smoke tests for individual tool versions.
- [x] Resolve optional artifact packaging: intentionally deferred; runtime `completion generate/install` and generated man/docs remain canonical for this release.

## H. `repo-zip` migration

- [x] Express complete CLI as Spec.
- [x] Preserve explicit interspersed-options semantics.
- [x] Remove `-v/--version TEXT` suffix alias.
- [x] Wire handler to existing service/domain.
- [x] Delete manual help/parser code no longer needed.
- [x] Generate and commit contract lock.
- [x] Add change record for breaking syntax.
- [x] Pass tool E2E and core contract tests.

## I. `ai-profile` migration

- [x] Express tools/actions as command modules/spec.
- [x] Model profile/template codecs/completers.
- [x] Model `run/acp` opaque trailing argv.
- [x] Preserve ACP stdio/exit semantics.
- [x] Replace manual confirmations with interaction policy where suitable.
- [x] Delete hand-written help.
- [x] Delete Bash/Fish/Zsh scripts and old `__complete` dispatch.
- [x] Remove `isStaticInvocation` workaround.
- [x] Generate and commit contract lock.
- [x] Pass E2E and shell completion tests.

## J. Cleanup/docs

- [x] Delete `internal/cliapp` after last consumer is gone.
- [x] Remove dead parser/completion helpers.
- [x] Update root AGENTS.md to the new mandatory CLI rules.
- [x] Update architecture/adding-tools/engineering/platform/testing/release docs.
- [x] Update CLI README/ADRs for intentional syntax changes.
- [x] Run repo-wide dead-code/reference search.

## K. Quality gates

- [x] `gofmt` clean.
- [x] hermetic unit/integration tests pass.
- [x] `go vet` pass.
- [x] shuffle tests pass.
- [x] race pass on supported runners.
- [x] fuzz smoke pass for new surfaces.
- [x] shell conformance pass.
- [x] native shell E2E pass where supported.
- [x] platform runtime evidence matches support claims.
- [x] benchmarks establish baseline for compile/parse/completion.
- [x] generated contracts/docs are deterministic.
- [x] no real user state/network/credentials touched.
