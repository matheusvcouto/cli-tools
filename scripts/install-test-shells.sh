#!/usr/bin/env bash
# CI and release use the same native shell prerequisites on Ubuntu amd64.
# The downloaded Nushell executable is installed only after upstream SHA-256
# verification; temporary extraction never uses a shared fixed /tmp filename.
set -euo pipefail

if [[ "$(uname -s)" != Linux || "$(uname -m)" != x86_64 ]]; then
  echo 'install-test-shells.sh requires native linux/amd64 on ubuntu-24.04' >&2
  exit 1
fi

sudo apt-get update
sudo apt-get install -y --no-install-recommends fish zsh

version='0.114.1'
sha256='ce3a1c5a07c784098b5675224a165e73a45f0f720a8392791d7c3a5b4720255e'
asset="nu-${version}-x86_64-unknown-linux-musl.tar.gz"
tmp="$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/nushell.XXXXXX")"
trap 'rm -rf -- "$tmp"' EXIT

curl --fail --location --retry 3 --proto '=https' --proto-redir '=https' --tlsv1.2 \
  --output "$tmp/$asset" \
  "https://github.com/nushell/nushell/releases/download/${version}/${asset}"
printf '%s  %s\n' "$sha256" "$tmp/$asset" | sha256sum --check --strict
tar -xzf "$tmp/$asset" -C "$tmp" "nu-${version}-x86_64-unknown-linux-musl/nu"
sudo install -m 0755 "$tmp/nu-${version}-x86_64-unknown-linux-musl/nu" /usr/local/bin/nu
nu --version
