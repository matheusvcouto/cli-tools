#!/usr/bin/env bash
# Verify that the remote release tag still resolves to the exact triggering
# commit, including annotated tags. This is NOT a signature verification:
# branch/tag protection and signed-tag policy belong in GitHub rulesets.
set -euo pipefail

test -n "${GH_TOKEN:-}" || { echo 'GH_TOKEN is required' >&2; exit 1; }
test -n "${GITHUB_REPOSITORY:-}" || { echo 'GITHUB_REPOSITORY is required' >&2; exit 1; }
test -n "${GITHUB_REF_NAME:-}" || { echo 'GITHUB_REF_NAME is required' >&2; exit 1; }
[[ "$GITHUB_REF_NAME" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]] || {
  echo 'release tag must be an explicit semver-style vX.Y.Z tag' >&2
  exit 1
}
[[ "${GITHUB_SHA:-}" =~ ^[0-9a-fA-F]{40}$ ]] || {
  echo 'GITHUB_SHA must be the exact triggering 40-digit commit SHA' >&2
  exit 1
}

parse_git_object() {
  python3 -c '
import json, re, sys
obj = json.load(sys.stdin)
value = obj.get("object")
if not isinstance(value, dict) or value.get("type") not in ("commit", "tag"):
    raise SystemExit("unexpected GitHub git object type")
sha = value.get("sha")
if not isinstance(sha, str) or not re.fullmatch(r"[a-fA-F0-9]{40}", sha):
    raise SystemExit("unexpected GitHub git object SHA")
print(value["type"], sha.lower())
'
}

response="$(gh api "repos/$GITHUB_REPOSITORY/git/ref/tags/$GITHUB_REF_NAME")"
remote_ref="$(printf '%s' "$response" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("ref", ""))')"
[[ "$remote_ref" == "refs/tags/$GITHUB_REF_NAME" ]] || {
  echo "remote tag ref mismatch: $remote_ref" >&2
  exit 1
}
object_data="$(printf '%s' "$response" | parse_git_object)"
read -r object_type object_sha <<<"$object_data"

# Git supports annotated tags of tags; peel a bounded chain and refuse any
# unfamiliar object type rather than accepting a tag-object SHA as a commit.
for _ in 1 2 3 4; do
  if [[ "$object_type" == commit ]]; then
    [[ "$object_sha" == "${GITHUB_SHA,,}" ]] || {
      echo "remote tag $GITHUB_REF_NAME moved: $object_sha != $GITHUB_SHA" >&2
      exit 1
    }
    echo "remote release tag $GITHUB_REF_NAME resolves to $object_sha"
    exit 0
  fi
  response="$(gh api "repos/$GITHUB_REPOSITORY/git/tags/$object_sha")"
  object_data="$(printf '%s' "$response" | parse_git_object)"
read -r object_type object_sha <<<"$object_data"
done

echo 'remote tag nesting limit exceeded' >&2
exit 1
