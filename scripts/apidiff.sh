#!/usr/bin/env bash
# apidiff.sh — compare each stable package's exported API against the newest
# release tag and fail on an incompatible change.
#
# Usage: scripts/apidiff.sh [BASE_TAG]
#
#   BASE_TAG     compare against this tag instead of the newest v* semver tag.
#   BREAKING_OK  set to 1 to report incompatible changes without failing
#                (CI sets it when the PR carries the `breaking-change` label,
#                and on pushes to main, where the PR gate already ran).
#   APIDIFF_VERSION  golang.org/x/exp version to build apidiff from.
#
# The stable package list is .github/stable-packages.txt (import paths
# relative to the module root; `#` comments and blank lines are ignored).
# See docs/releasing.md for the policy this enforces.
#
# apidiff is installed into a temporary GOBIN from outside the module, so
# gorge's go.mod and go.sum are never touched (archtest
# TestRootModuleHasNoRequires keeps the root go.mod free of requires).
# The base tag is checked out into a temporary `git worktree` (detached) that
# is removed on exit; the current checkout is never switched.
set -euo pipefail

APIDIFF_VERSION=${APIDIFF_VERSION:-v0.0.0-20261014143641-f0251eb4aa3e}
BREAKING_OK=${BREAKING_OK:-0}

root=$(git rev-parse --show-toplevel)
cd "$root"
module=$(go list -m)
list="$root/.github/stable-packages.txt"

semver_re='^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$'
base=${1:-}
if [ -z "$base" ]; then
  base=$(git tag --list 'v*' --sort=-version:refname | grep -E "$semver_re" | head -n 1 || true)
fi
if [ -z "$base" ]; then
  echo "::notice::apidiff: no v* release tag yet; nothing to compare against"
  exit 0
fi
git rev-parse -q --verify "refs/tags/$base^{commit}" >/dev/null || {
  echo "apidiff: tag $base does not exist (fetch tags: git fetch --tags)" >&2
  exit 2
}

tmp=$(mktemp -d)
old="$tmp/old"
cleanup() {
  git -C "$root" worktree remove --force "$old" >/dev/null 2>&1 || true
  rm -rf "$tmp"
}
trap cleanup EXIT

echo "apidiff: building golang.org/x/exp/cmd/apidiff@$APIDIFF_VERSION"
(cd "$tmp" && GOFLAGS= GOBIN="$tmp/bin" go install "golang.org/x/exp/cmd/apidiff@$APIDIFF_VERSION")
apidiff="$tmp/bin/apidiff"

git worktree add --quiet --detach "$old" "$base"
echo "apidiff: base $base ($(git rev-parse --short "$base^{commit}")) vs working tree $(git rev-parse --short HEAD)"

broken=0
n=0
while IFS= read -r line || [ -n "$line" ]; do
  pkg=${line%%#*}
  pkg=$(echo "$pkg" | tr -d '[:space:]')
  [ -n "$pkg" ] || continue
  n=$((n + 1))
  ip="$module/$pkg"
  echo
  echo "== $ip"
  if [ ! -d "$old/$pkg" ]; then
    echo "   new since $base: no baseline, skipped"
    continue
  fi
  (cd "$old" && "$apidiff" -w "$tmp/old-$n.export" "$ip")
  (cd "$root" && "$apidiff" -w "$tmp/new-$n.export" "$ip")
  all=$("$apidiff" "$tmp/old-$n.export" "$tmp/new-$n.export")
  bad=$("$apidiff" -incompatible "$tmp/old-$n.export" "$tmp/new-$n.export")
  if [ -z "$all" ]; then
    echo "   no API change"
  else
    echo "$all" | sed 's/^/   /'
  fi
  if [ -n "$bad" ]; then
    broken=$((broken + 1))
    echo "::warning::apidiff: incompatible change in $ip since $base"
  fi
done <"$list"

echo
if [ "$broken" -eq 0 ]; then
  echo "apidiff: OK, no incompatible change in $n stable package(s) since $base"
  exit 0
fi
if [ "$BREAKING_OK" = 1 ]; then
  echo "apidiff: $broken package(s) changed incompatibly since $base; allowed (BREAKING_OK=1)."
  echo "Record the change under ### Breaking in CHANGELOG.md (docs/releasing.md)."
  exit 0
fi
echo "::error::apidiff: $broken stable package(s) changed incompatibly since $base."
echo "If the break is intended, label the PR 'breaking-change' and record it under"
echo "### Breaking in CHANGELOG.md; otherwise restore the old API (docs/releasing.md)."
exit 1
