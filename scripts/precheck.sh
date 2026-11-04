#!/usr/bin/env bash
# precheck.sh [base]: the checks a landing gate most often fails on, run on a committed change before it
# is handed over, so a finding costs a minute here instead of a gate run and a round trip. base is the
# branch the change will land on (default origin/main). `make precheck BASE=<ref>` runs it.
#
#   1. gofmt on the Go files changed since the merge base, and go vet on their packages.
#   2. The census and coverage tests (a struct's every field compared or encoded, stall classes, flag
#      blocks, cache-line padding, card censuses, kgen's acceptance pins, TestHeads and TestSetHeads, the
#      file-name rule, the committed repro tables every set replays in lockstep) in every package that
#      holds one, plus ./internal/archtest. About 40 s on 8 cores,
#      answered from the test cache on a rerun.
#   3. base's own scripts/dirac-gate.sh, as a landing runs it: State size, allocs, instructions against
#      the merge base, lockstep, refused entries, synthetic pools. It needs the deck pools (GORGE_POOLS)
#      for the refused entries.
#
# The first failing step ends the run. It changes nothing and checks a commit, so the tree must be
# clean. Exit 0 means ready.
set -uo pipefail
base=${1:-origin/main}
top=$(git rev-parse --show-toplevel) || exit 2
cd "$top" || exit 2
[ -d .cards/cardsfolder ] || [ -n "${GORGE_CARDS:-}" ] || { echo "precheck: no .cards corpus (make fetch-cards, or link the main checkout's): every corpus test would skip"; exit 2; }
[ -z "$(git status --porcelain --untracked-files=no)" ] || { echo "precheck: commit first; it checks a commit, not a dirty tree"; exit 2; }
mb=$(git merge-base "$base" HEAD) || exit 2
[ "$mb" = "$(git rev-parse "$base")" ] || echo "precheck: note: not rebased onto $base, so a conflict with it is not covered"
# The steps run cheapest first and the first failure ends the run: the rest cannot change the answer.
bad() { echo "precheck: FAIL -- $*"; echo "precheck: FAIL $(git rev-parse --short HEAD)"; exit 1; }

echo "== 1 gofmt, vet"
files=$(git diff --name-only --diff-filter=d "$mb" HEAD -- '*.go')
if [ -n "$files" ]; then
	unfmt=$(gofmt -l $files)
	[ -z "$unfmt" ] || bad "gofmt: $unfmt"
	pkgs=$(for f in $files; do echo "./$(dirname "$f")"; done | sort -u)
	out=$(go vet $pkgs 2>&1) || bad "go vet: $(tail -15 <<<"$out")"
fi

echo "== 2 census and coverage tests"
C='Census|Covers|IsCurrent|Ratchet|Padded|HasAClass|FileNames|OnlyShrinks|IotaBlock|NoVerbatim|PublicBoundary|Acceptance|ReprosLockstep|^TestHeads$|^TestSetHeads$'
cpkgs=$(git grep -lE "^func Test[A-Za-z0-9_]*($C)" -- '*_test.go' | xargs -n1 dirname | sort -u | sed 's|^|./|')
out=$(go test -timeout 10m -run "$C" $cpkgs ./internal/archtest 2>&1) || bad "census tests: $(grep -E -A8 -- '^--- FAIL|^FAIL|panic' <<<"$out" | head -40)"

echo "== 3 dirac gate ($base's copy)"
gate=$(mktemp "${TMPDIR:-/tmp}/precheck-dirac-gate.XXXXXX") || exit 2
trap 'rm -f -- "$gate"' EXIT
git show "$base:scripts/dirac-gate.sh" >"$gate" || exit 2
out=$(bash "$gate" "$base" 2>&1)
rc=$?
grep -E '^(==|dirac gate|State|Benchmark|fdn:|blb:|std-|  )' <<<"$out" | head -60
[ "$rc" -eq 0 ] || bad "dirac gate rc=$rc"

echo "precheck: ok $(git rev-parse --short HEAD)"
