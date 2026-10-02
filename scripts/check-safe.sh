#!/bin/sh
set -eu

ROOT=$(CDPATH= cd "$(dirname "$0")/.." && pwd)
MODE=${1:-all}
if [ "$#" -gt 0 ]; then
  shift
fi
if [ "$#" -eq 0 ]; then
  set -- ./...
fi
# Tool caches survive checks; application state remains disposable.
TOOL_CACHE="$ROOT/dist/go-cache"
mkdir -p "$TOOL_CACHE/runs" "$TOOL_CACHE/build" "$TOOL_CACHE/mod" "$TOOL_CACHE/path"
BASE_TMP=$(CDPATH= cd "$TOOL_CACHE/runs" && pwd -P)
SANDBOX=$(mktemp -d "$BASE_TMP/cli-tools-test.XXXXXX")
MODULE_PROXY=off
MODULE_SUMDB=off
MODULE_VCS="*:off"
if [ "$MODE" = prepare ]; then
  MODULE_PROXY=${GOPROXY:-https://proxy.golang.org,direct}
  MODULE_SUMDB=${GOSUMDB:-sum.golang.org}
  MODULE_VCS="public:git|hg,private:all"
fi
SAFE_PATH=$PATH
if [ "$(uname -s)" = Darwin ] && [ -x /Library/Developer/CommandLineTools/usr/bin/git ]; then
  SAFE_PATH="/Library/Developer/CommandLineTools/usr/bin:$SAFE_PATH"
fi
cleanup() {
  case "${SANDBOX:-}" in
    "$BASE_TMP"/cli-tools-test.*)
      printf 'Removing disposable check state: %s\n' "$SANDBOX" >&2
      rm -rf "$SANDBOX"
      ;;
    *) printf '%s\n' "refusing to remove unexpected sandbox path: ${SANDBOX:-<empty>}" >&2 ;;
  esac
}
trap cleanup EXIT HUP INT TERM

mkdir -p \
  "$SANDBOX/home" \
  "$SANDBOX/tmp" \
  "$SANDBOX/xdg/config" \
  "$SANDBOX/xdg/cache" \
  "$SANDBOX/xdg/state" \
  "$SANDBOX/git/hooks" \
  "$SANDBOX/git/template"

# GOTELEMETRY is a read-only `go env` result, not an environment override.
# Seed the isolated mode before invoking Go, so no background telemetry process
# can race with cleanup. These are os.UserConfigDir locations on POSIX hosts.
case "$(uname -s)" in
  Darwin) TELEMETRY_CONFIG="$SANDBOX/home/Library/Application Support" ;;
  *) TELEMETRY_CONFIG="$SANDBOX/xdg/config" ;;
esac
mkdir -p "$TELEMETRY_CONFIG/go/telemetry"
printf 'off\n' > "$TELEMETRY_CONFIG/go/telemetry/mode"

run_clean() {
  env -i \
    PATH="$SAFE_PATH" \
    HOME="$SANDBOX/home" \
    USERPROFILE="$SANDBOX/home" \
    TMPDIR="$SANDBOX/tmp" \
    TMP="$SANDBOX/tmp" \
    TEMP="$SANDBOX/tmp" \
    XDG_CONFIG_HOME="$SANDBOX/xdg/config" \
    XDG_CACHE_HOME="$SANDBOX/xdg/cache" \
    XDG_STATE_HOME="$SANDBOX/xdg/state" \
    LANG=C LC_ALL=C TZ=UTC TERM=dumb \
    GOTOOLCHAIN=local \
    GOENV=off \
    GOWORK=off \
    GOFLAGS=-mod=readonly \
    GOPROXY="$MODULE_PROXY" \
    GOSUMDB="$MODULE_SUMDB" \
    GOVCS="$MODULE_VCS" \
    GOCACHE="$TOOL_CACHE/build" \
    GOMODCACHE="$TOOL_CACHE/mod" \
    GOPATH="$TOOL_CACHE/path" \
    GIT_CONFIG_NOSYSTEM=1 \
    GIT_CONFIG_GLOBAL="$SANDBOX/git/no-global-config" \
    GIT_CONFIG_COUNT=2 \
    GIT_CONFIG_KEY_0=core.hooksPath \
    GIT_CONFIG_VALUE_0="$SANDBOX/git/hooks" \
    GIT_CONFIG_KEY_1=init.templateDir \
    GIT_CONFIG_VALUE_1="$SANDBOX/git/template" \
    GIT_TERMINAL_PROMPT=0 \
    GIT_PAGER=cat \
    PAGER=cat \
    "$@"
}

cd "$ROOT"

fmt_check() {
  files=$(run_clean gofmt -l cli cmd internal integration tools)
  if [ -n "$files" ]; then
    printf '%s\n' "gofmt required:" "$files" >&2
    return 1
  fi
}

# Check test imports too. Dependency misses should explain the online preparation
# step, rather than implicitly downloading or changing manifests.
case "$MODE" in
  all|test|vet|shuffle|race|fuzz)
    if ! run_clean go list -deps -test ./... >/dev/null; then
      printf '%s\n' 'Offline preflight failed. If dependencies are missing, run ./scripts/check-safe.sh prepare with a compatible Go on PATH, then retry.' >&2
      exit 1
    fi
    ;;
esac

case "$MODE" in
  prepare)
    run_clean go mod download all
    run_clean go mod verify
    ;;
  fmt)
    fmt_check
    ;;
  test)
    run_clean go test "$@"
    ;;
  shuffle)
    run_clean go test -shuffle=on -count=3 "$@"
    ;;
  race)
    run_clean go test -race "$@"
    ;;
  vet)
    run_clean go vet "$@"
    ;;
  fuzz)
    # Run in the persistent worktree so testdata/fuzz crashers survive failures.
    run_clean go test ./cli -run='^$' -fuzz=FuzzStrictAndPartialNeverPanic -fuzztime=5s
    run_clean go test ./cli -run='^$' -fuzz=FuzzCompletionProtocolDecoderNeverPanics -fuzztime=5s
    run_clean go test ./cli -run='^$' -fuzz=FuzzSchemaAndContractJSONRoundTrip -fuzztime=5s
    run_clean go test ./cli -run='^$' -fuzz=FuzzShellEscapersNeverPanic -fuzztime=5s
    run_clean go test ./internal/aiprofile -run='^$' -fuzz=FuzzValidateAlias -fuzztime=5s
    run_clean go test ./internal/repozip -run='^$' -fuzz=FuzzValidateSuffix -fuzztime=5s
    run_clean go test ./internal/repozip -run='^$' -fuzz=FuzzVerifyZipNeverPanics -fuzztime=5s
    run_clean go test ./tools/migrate-ai-profile-index -run='^$' -fuzz=FuzzParseLegacyNUONNeverPanics -fuzztime=5s
    ;;
  all)
    fmt_check
    run_clean go test ./...
    run_clean go vet ./...
    run_clean go test -shuffle=on -count=3 ./...
    run_clean go test -race ./...
    run_clean go run ./tools/release api check
    run_clean go run ./tools/release contracts check
    run_clean go run ./tools/release changes validate
    ;;
  *)
    echo "usage: $0 [prepare|all|fmt|test|shuffle|race|vet|fuzz]" >&2
    exit 2
    ;;
esac
