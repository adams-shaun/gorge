#!/usr/bin/env bash
# Deprecated: `make precheck` is now an alias for `make test-smart` (scripts/test-targets.sh), which runs
# the steps below plus the affected-package gate, the dirac gate's fail handling and the wire goldens.
# This script stays as a direct fallback: `bash scripts/precheck.sh <base>` (base defaults to origin/main).
#
# precheck.sh [base]: the checks a landing gate most often fails on, run on a committed change before it
# is handed over, so a finding costs a minute here instead of a gate run and a round trip.
#
#   1. gofmt on the Go files changed since the merge base, and go vet on their packages.
#   2. The census and coverage tests (a struct's every field compared or encoded, stall classes, flag
#      blocks, cache-line padding, card censuses, kgen's acceptance pins, TestHeads and TestSetHeads, the
#      file-name rule) in every package that holds one, plus ./internal/archtest. The committed repro
#      tables (TestStd*ReprosLockstep in ./dirac/oracle) are skipped here: the dirac gate runs them.
#      About 40 s on 8 cores, answered from the test cache on a rerun.
#   2b. A card unrefused since the merge base needs a row in dirac/support/unrefused.tsv (TestUnrefusalNeedsProof).
#   3. base's own scripts/dirac-gate.sh, as a landing runs it: State size, allocs, instructions against
#      the merge base, lockstep, refused entries, synthetic pools, the repro tables. It needs the deck
#      pools (GORGE_POOLS) for the refused entries.
#   3b. Only when the gate skipped (no file in kfuzz's dependency closure changed): the repro tables,
#      so every one runs exactly once per precheck.
#
# It caps itself: it re-execs inside a user systemd scope (MemoryMax=8G, CPUQuota=400%, GOMAXPROCS=4,
# GOMEMLIMIT=6GiB), so how the caller typed the command does not decide its share of the box. With no
# user systemd (CI, a container) it notes so and runs uncapped. The first failing step ends the run. It
# changes nothing and checks a commit, so the tree must be clean. Exit 0 means ready.
set -uo pipefail
base=${1:-origin/main}
top=$(git rev-parse --show-toplevel) || exit 2
cd "$top" || exit 2
if [ -z "${PRECHECK_SCOPED:-}" ]; then
	if systemd-run --user --scope -q true >/dev/null 2>&1; then
		echo "precheck: capped (MemoryMax=8G CPUQuota=400%)"
		# env keeps the rest of the environment (GORGE_POOLS, GORGE_CARDS) for the child.
		exec systemd-run --user --scope -q -p MemoryMax=8G -p CPUQuota=400% env PRECHECK_SCOPED=1 GOMAXPROCS=4 GOMEMLIMIT=6GiB bash "$top/scripts/precheck.sh" "$base"
	fi
	echo "precheck: note: no user systemd; running uncapped"
fi
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
C='Census|Covers|IsCurrent|Ratchet|Padded|HasAClass|FileNames|OnlyShrinks|IotaBlock|NoVerbatim|PublicBoundary|Acceptance|ReprosLockstep|EnumConst|^TestHeads$|^TestSetHeads$'
cpkgs=$(git grep -lE "^func Test[A-Za-z0-9_]*($C)" -- '*_test.go' | xargs -n1 dirname | sort -u | sed 's|^|./|')
R='^TestStd[A-Za-z]*ReprosLockstep'
out=$(go test -timeout 10m -run "$C" -skip "$R" $cpkgs ./internal/archtest 2>&1) || bad "census tests: $(grep -E -A8 -- '^--- FAIL|^FAIL|panic' <<<"$out" | head -40)"

echo "== 2b unrefusals carry proof"
out=$(KERNEL_UNREFUSE_BASE="$base" go test -timeout 5m -run '^TestUnrefusalNeedsProof$' ./dirac/oracle 2>&1) || bad "unrefusal proof: $(grep -E -A8 -- '^--- FAIL|^FAIL|panic|^    ' <<<"$out" | head -30)"

echo "== 3 dirac gate ($base's copy)"
gate=$(mktemp "${TMPDIR:-/tmp}/precheck-dirac-gate.XXXXXX") || exit 2
trap 'rm -f -- "$gate"' EXIT
git show "$base:scripts/dirac-gate.sh" >"$gate" || exit 2
# PRECHECK_DIRAC_GATE names a program that runs the same gate elsewhere (another machine) and prints its
# output and exits with its code; it is called with the base and must itself fall back to a local run.
if [ -n "${PRECHECK_DIRAC_GATE:-}" ] && [ -x "$PRECHECK_DIRAC_GATE" ]; then
	out=$("$PRECHECK_DIRAC_GATE" "$base" 2>&1)
else
	out=$(bash "$gate" "$base" 2>&1)
fi
rc=$?
grep -E '^(==|dirac gate|State|Benchmark|fdn:|blb:|std-|  )' <<<"$out" | head -60
[ "$rc" -eq 0 ] || bad "dirac gate rc=$rc"

if grep -q 'dirac gate: skipped' <<<"$out"; then
	echo "== 3b repro tables (the dirac gate skipped)"
	out=$(go test -timeout 10m -run "$R" ./dirac/oracle 2>&1) || bad "repro tables: $(grep -E -A8 -- '^--- FAIL|^FAIL|panic' <<<"$out" | head -40)"
fi

echo "precheck: ok $(git rev-parse --short HEAD)"
