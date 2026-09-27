#!/usr/bin/env bash
# CI/Release gate. Fetch the published upstream actionlint binary, verify its
# official release SHA-256, and lint all workflows. Never use unpinned installers.
set -euo pipefail

ROOT="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$ROOT"

if [[ "$(uname -s)" != Linux || "$(uname -m)" != x86_64 ]]; then
  echo 'check-workflows.sh requires native linux/amd64; run on ubuntu-24.04' >&2
  exit 1
fi

version='1.7.12'
asset="actionlint_${version}_linux_amd64.tar.gz"
sha256='8aca8db96f1b94770f1b0d72b6dddcb1ebb8123cb3712530b08cc387b349a3d8'
tmp="$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/actionlint.XXXXXX")"
trap 'rm -rf -- "$tmp"' EXIT

curl --fail --location --retry 3 --proto '=https' --proto-redir '=https' --tlsv1.2 \
  --output "$tmp/$asset" \
  "https://github.com/rhysd/actionlint/releases/download/v${version}/${asset}"
printf '%s  %s\n' "$sha256" "$tmp/$asset" | sha256sum --check --strict
tar -xzf "$tmp/$asset" -C "$tmp" actionlint
"$tmp/actionlint" -version
# Pass the config explicitly. Auto-discovery would also read it from .github/,
# but a missing file must fail the gate instead of linting with the stale catalog.
"$tmp/actionlint" -color -config-file .github/actionlint.yaml \
  .github/workflows/ci.yml .github/workflows/release.yml
