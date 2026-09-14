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
BASE_TMP=$(CDPATH= cd "${TMPDIR:-/tmp}" && pwd -P)
SANDBOX=$(mktemp -d "$BASE_TMP/cli-tools-test.XXXXXX")
SAFE_PATH=$PATH
if [ "$(uname -s)" = Darwin ] && [ -x /Library/Developer/CommandLineTools/usr/bin/git ]; then
  SAFE_PATH="/Library/Developer/CommandLineTools/usr/bin:$SAFE_PATH"
fi
cleanup() {
  case "${SANDBOX:-}" in
    "$BASE_TMP"/cli-tools-test.*) rm -rf "$SANDBOX" ;;
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
  "$SANDBOX/go/cache" \
  "$SANDBOX/go/modcache" \
  "$SANDBOX/go/path" \
  "$SANDBOX/git/hooks" \
  "$SANDBOX/git/template"

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
    GOFLAGS= \
    GOPROXY=off \
    GOSUMDB=off \
    GOVCS='*:off' \
    GOTELEMETRY=off \
    GOCACHE="$SANDBOX/go/cache" \
    GOMODCACHE="$SANDBOX/go/modcache" \
    GOPATH="$SANDBOX/go/path" \
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
  files=$(run_clean gofmt -l .)
  if [ -n "$files" ]; then
    printf '%s\n' "gofmt required:" "$files" >&2
    return 1
  fi
}

case "$MODE" in
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
    FUZZ_ROOT="$SANDBOX/source"
    mkdir -p "$FUZZ_ROOT"
    for item in "$ROOT"/* "$ROOT"/.[!.]* "$ROOT"/..?*; do
      [ -e "$item" ] || continue
      case "$(basename "$item")" in
        .git|.tmp|dist) continue ;;
      esac
      cp -R "$item" "$FUZZ_ROOT/"
    done
    cd "$FUZZ_ROOT"
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
    echo "usage: $0 [all|fmt|test|shuffle|race|vet|fuzz]" >&2
    exit 2
    ;;
esac
