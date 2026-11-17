#!/usr/bin/env bash
# pr-precheck.sh BASE: the checks that turned pull requests red in their fast stage (11-17: the function-shape
# ratchets, the generated support page, the PR's own new tests), run on the branch before agentctl open-pr pushes
# it. A few seconds to a few minutes; the PR's fast stage and its Mergify batch run the rest. The last line printed
# is the verdict: "pr-precheck: ok" or the first failure.
set -uo pipefail
base=${1:?usage: pr-precheck.sh BASE}
source scripts/pr-cards.sh
cap=(systemd-run --user --scope -q -p MemoryMax=4G -p CPUQuota=400% env GOMAXPROCS=4 GOMEMLIMIT=3GiB)
log=$(mktemp); trap 'rm -f "$log"' EXIT
fail() { echo "pr-precheck: $*"; exit 1; }
changed=$(git diff --name-only "$base"...HEAD)
if ! "${cap[@]}" go test -timeout 3m -run '^TestComplexFunctionsOnlyShrink$|^TestLongFunctionsOnlyShrink$|^TestFuncCeilingsLowerOnly$' ./internal/codeshape >"$log" 2>&1; then
  cat "$log"; fail "codeshape: $(grep -o -E '[^ ]+ \([^)]*\) grew to [a-z]+ [0-9]+, above its ceiling of [0-9]+' "$log" | head -1)"
fi
if grep -q '^dirac/' <<<"$changed"; then
  "${cap[@]}" go test -timeout 5m -run '^TestSupportDocIsCurrent$' ./dirac/kgen >"$log" 2>&1 || { cat "$log"; fail "dirac/docs/support.md is stale (scripts/pr-regen.sh dirac/docs/support.md)"; }
fi
for pkg in $(grep '_test\.go$' <<<"$changed" | xargs -r -n1 dirname | sort -u); do
  rx=$(grep "^$pkg/[^/]*_test\.go$" <<<"$changed" | while read -r f; do [ -f "$f" ] && sed -n 's/^func \(Test[A-Za-z0-9_]*\)(t \*testing\.T).*/\1/p' "$f"; done | sort -u | paste -sd'|')
  [ -n "$rx" ] || continue
  "${cap[@]}" go test -timeout 5m -run "^($rx)\$" "./$pkg" >"$log" 2>&1 || { cat "$log"; fail "tests in $pkg: $(grep -o -E -- '--- FAIL: [A-Za-z0-9_/]+' "$log" | head -2 | sed 's/--- FAIL: //' | paste -sd' ')"; }
done
echo "pr-precheck: ok"
