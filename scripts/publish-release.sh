#!/usr/bin/env bash
# Publication is recoverable after interrupted draft uploads, but never
# clobbers existing assets or publishes unverified bytes.
set -euo pipefail

ROOT="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$ROOT"

test -n "${GH_TOKEN:-}" || { echo 'GH_TOKEN is required' >&2; exit 1; }
tag="${GITHUB_REF_NAME:?GITHUB_REF_NAME required}"
[[ "$tag" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]] || {
  echo "unsupported release tag: $tag" >&2
  exit 1
}
test -s release-notes.md
test -s dist/SHA256SUMS

version="${tag#v}"
expected=(
  SHA256SUMS
  "cli-tools_${version}_linux_amd64.tar.gz"
  "cli-tools_${version}_linux_arm64.tar.gz"
  "cli-tools_${version}_macos_amd64.tar.gz"
  "cli-tools_${version}_macos_arm64.tar.gz"
  "cli-tools_${version}_windows_amd64.zip"
  "cli-tools_${version}_windows_arm64.zip"
)
expected_names="$(printf '%s\n' "${expected[@]}" | LC_ALL=C sort)"
actual_names="$(find dist -mindepth 1 -maxdepth 1 -printf '%f\n' | LC_ALL=C sort)"
[[ "$actual_names" == "$expected_names" ]] || {
  echo 'local release bundle has missing or unexpected files' >&2
  exit 1
}
for name in "${expected[@]}"; do
  [[ -f "dist/$name" && ! -L "dist/$name" ]] || {
    echo "missing or symlinked local release file: $name" >&2
    exit 1
  }
done
(cd dist && sha256sum --check --strict SHA256SUMS)
"$ROOT/scripts/verify-remote-release-tag.sh"

# A failed create may mean that an earlier attempt already left a draft, or
# that the API failed. In either case, proceed ONLY after verifying its exact
# metadata and all existing assets. Never use gh release upload --clobber.
if ! gh release create "$tag" --draft \
    --title "CLI Tools $tag" --notes-file release-notes.md --verify-tag; then
  echo 'draft creation failed; checking whether a matching draft can be resumed' >&2
fi

tmp="$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/cli-tools-publish.XXXXXX")"
trap 'rm -rf -- "$tmp"' EXIT

inspect_draft() {
  local metadata_path="$tmp/release.json"
  gh release view "$tag" --json tagName,name,body,isDraft,assets > "$metadata_path"
  python3 - "$metadata_path" release-notes.md "$tag" "${expected[@]}" <<'PY'
import json, pathlib, sys
meta = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
notes = pathlib.Path(sys.argv[2]).read_text(encoding="utf-8")
tag = sys.argv[3]
allowed = set(sys.argv[4:])
if meta.get("tagName") != tag or meta.get("name") != "CLI Tools " + tag:
    raise SystemExit("refusing to resume release with unexpected tag/title")
if not meta.get("isDraft"):
    raise SystemExit("refusing to alter an already published release")
if not isinstance(meta.get("body"), str) or meta["body"].replace("\r\n", "\n").rstrip("\n") != notes.replace("\r\n", "\n").rstrip("\n"):
    raise SystemExit("refusing to resume release with different notes")
assets = meta.get("assets")
if not isinstance(assets, list):
    raise SystemExit("release assets metadata is missing")
names = [asset.get("name") for asset in assets if isinstance(asset, dict)]
if len(names) != len(assets) or len(names) != len(set(names)) or not set(names).issubset(allowed):
    raise SystemExit("draft has unexpected or duplicate assets")
if names:
    print("\n".join(names))
PY
}

verify_downloaded() {
  local remote_dir="$tmp/remote"
  # A second check must fetch fresh bytes, not reuse the first download.
  rm -rf -- "$remote_dir"
  mkdir -p "$remote_dir"
  # Explicit tag works with authenticated draft releases. Download every
  # existing asset and verify against our local expected bytes.
  gh release download "$tag" --dir "$remote_dir" >/dev/null
  local uploaded_names
  uploaded_names="$(find "$remote_dir" -mindepth 1 -maxdepth 1 -printf '%f\n' | LC_ALL=C sort)"
  local metadata_names
  metadata_names="$(inspect_draft | LC_ALL=C sort)"
  [[ "$uploaded_names" == "$metadata_names" ]] || {
    echo 'downloaded release assets differ from API asset listing' >&2
    exit 1
  }
  local asset hash remote_hash
  for asset in "$remote_dir"/*; do
    [[ -e "$asset" ]] || continue
    [[ -f "$asset" && ! -L "$asset" ]] || {
      echo 'downloaded release asset is not a regular file' >&2
      exit 1
    }
    hash="$(sha256sum "dist/$(basename "$asset")" | cut -d' ' -f1)"
    remote_hash="$(sha256sum "$asset" | cut -d' ' -f1)"
    [[ "$hash" == "$remote_hash" ]] || {
      echo "remote asset differs from local release bundle: $(basename "$asset")" >&2
      exit 1
    }
  done
  printf '%s\n' "$metadata_names"
}

# First inspect independently of any transient error returned by create.
inspect_draft > "$tmp/existing.txt"
# gh release download may fail on an empty draft. There is nothing to verify
# when the API reports zero assets.
if [[ -s "$tmp/existing.txt" ]]; then
  verify_downloaded > "$tmp/verified.txt"
else
  : > "$tmp/verified.txt"
fi

for name in "${expected[@]}"; do
  if grep -Fqx -- "$name" "$tmp/verified.txt"; then
    continue
  fi
  gh release upload "$tag" "dist/$name"
done

# Re-read metadata and bytes from GitHub, not from our local upload directory.
verify_downloaded > "$tmp/final-remote.txt"
[[ "$(LC_ALL=C sort "$tmp/final-remote.txt")" == "$expected_names" ]] || {
  echo 'remote release is missing required verified assets' >&2
  exit 1
}
# Protect against a remote tag rewrite between draft creation and publish.
"$ROOT/scripts/verify-remote-release-tag.sh"
gh release edit "$tag" --draft=false
# Postcondition: do not announce success if GitHub still considers it a draft.
status="$(gh release view "$tag" --json tagName,isDraft --jq '.tagName + ":" + (.isDraft|tostring)')"
[[ "$status" == "$tag:false" ]] || {
  echo "release publication postcondition failed: $status" >&2
  exit 1
}
echo "published $tag with all remote assets SHA-256 verified"
