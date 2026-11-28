#!/usr/bin/env bash
# fresh-clone-check.sh: what a fresh clone sees of dirac: no corpus, no tables. It must build, vet
# and test, with every corpus-dependent test skipping and none failing. Checks a commit ($REF,
# default HEAD), exported to a scratch directory rather than a worktree, so no checkout above it
# can lead a test to a corpus, and given a .git of its own as a clone has.
set -euo pipefail
top=$(git rev-parse --show-toplevel)
ref=${REF:-HEAD}
scratch=$(mktemp -d "${TMPDIR:-/tmp}/fresh-clone-check.XXXXXX")
trap 'rm -rf -- "$scratch"' EXIT

# cardset walks up from the cwd looking for .cards: one above the scratch directory is a corpus.
d=$scratch
while :; do
	[ ! -e "$d/.cards" ] || { echo "fresh-clone-check: $d/.cards is above the scratch directory: set TMPDIR" >&2; exit 1; }
	[ "$d" != / ] || break
	d=$(dirname "$d")
done
git -C "$top" archive "$ref" | tar -x -C "$scratch"
cd "$scratch"
git init -q .
unset GORGE_CARDS GORGE_CARDS_DIR DIRAC_TABLES GORGE_POOLS KGEN_DECKS
export GOWORK=off GOFLAGS=-trimpath # -trimpath: the random directory must not defeat the build cache

echo "fresh-clone-check: $ref ($(git -C "$top" rev-parse --short "$ref")): go build"
go build ./...
echo "fresh-clone-check: go vet"
go vet ./dirac/...

# A corpus that leaked in would make the next step a full oracle run reading green, so prove first
# that a test needing tables skips.
out=$(go test -timeout 2m -v -run '^TestCoverageRatchet$' ./dirac/cardset)
case $out in
*"--- SKIP: TestCoverageRatchet"*) ;;
*) printf '%s\n' "$out"; echo "fresh-clone-check: TestCoverageRatchet did not skip: this tree found a corpus" >&2; exit 1 ;;
esac

echo "fresh-clone-check: go test"
# -timeout 5m is the repo's default package timeout (scripts/test_targets.py DEFAULT_TIMEOUT), and the same
# fresh-clone packages pr-precheck.sh runs; 2m was the only place below it. The 1-minute per-test budget is
# unchanged (a test is split, never trimmed); a package binary on a shared runner is not a test. 11-28 merge
# batch 46140183894, with 6 of 9 runners offline, ran ./dirac at 120.3s (timed out) and ./dirac/oracle at 83.9s
# against 56.6s and 50.3s in the uncontended batch minutes earlier: one shared-runner slowdown, not a slow test.
log=$scratch/.test.log
rc=0
go test -timeout 5m ./dirac/... >"$log" 2>&1 || rc=$?
grep -v 'no test files' "$log" || true
[ "$rc" -eq 0 ] || { echo "fresh-clone-check: FAIL" >&2; exit 1; }
echo "fresh-clone-check: ok"
