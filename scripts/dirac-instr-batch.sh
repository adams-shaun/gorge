#!/usr/bin/env bash
# dirac-instr-batch.sh -- the merge-queue batch's instruction check of the four-seat Commander
# transition (BenchmarkTransitionEDH4), run by ci.yml's dirac-slices job, never by a pull request's
# own checks. The measure is dirac-gate.sh step 5's: per-op instructions:u under perf stat, GOGC=off,
# GOMAXPROCS=1 on core $INSTR_CORE, differenced (N100k-N20k)/80000. No wall time is read.
#
# The base is scripts/dirac-instr-base.tsv (bench<TAB>instructions<TAB>commit), meant to be the CI
# runner's value. The starting row (682003 at 57aaf5fb9e) is not: it is this script's output on a
# remote test host (AMD EPYC 9R14), to be re-based on the runner before 2026-12-18, because hosts
# can differ by more than the 2% band. Only the lane that regenerates pinned files moves it, in a
# change of its own that names the cause (dirac-gate.sh step 0 refuses an ordinary change to it or
# to this file). A lower count never fails; that lane lowers the base when a speed-up lands.
#
# INSTR_EDH4_ENFORCE=1 (the workflow sets it from the date; this script reads no clock):
#   exit 1 when head / base > 1.02, exit 2 when nothing was measured (no edh table, no perf, a
#   crashed bench, no base row). Otherwise the same line is printed and the exit is 0.
set -uo pipefail
export LC_ALL=C
INSTR_CORE=${INSTR_CORE:-20}
ENFORCE=0
[ "${INSTR_EDH4_ENFORCE:-}" = 1 ] && ENFORCE=1
MAX=1.02
BENCH=BenchmarkTransitionEDH4
N_LO=20000
N_HI=100000

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd) || exit 2
top=$(dirname "$here")
basefile=$here/dirac-instr-base.tsv

# infra: nothing was measured. A failure while enforcing (infrastructure, as the gate's exit 2),
# a report otherwise; never a silent 0 with no measurement.
infra() {
	echo "$BENCH: not measured: $* (enforce=$ENFORCE)"
	[ "$ENFORCE" = 1 ] && exit 2
	exit 0
}

S=$(mktemp -d) || exit 2
trap 'rm -rf "$S"' EXIT

base=$(awk -F'\t' -v b="$BENCH" '$1 == b && $2 ~ /^[0-9]+$/ && $2 > 0 {print $2; exit}' "$basefile" 2>/dev/null)
[ -n "$base" ] || infra "no $BENCH row in ${basefile#"$top"/}"

if ! command -v perf >/dev/null 2>&1 || [ -z "$(perf stat -x, -e instructions:u true 2>&1 | awk -F, '$1 ~ /^[0-9]+$/')" ]; then
	infra "perf stat unavailable (instructions:u not counted)"
fi

(cd "$top" && go test -c -o "$S/dirac.test" ./dirac/) >"$S/build.log" 2>&1 || {
	tail -20 "$S/build.log"
	infra "the dirac test binary does not build"
}

# has_result <N>: the run at N iterations printed the bench's result line.
has_result() {
	grep -qE "^$BENCH(-[0-9]+)?[[:space:]]+[0-9]+[[:space:]]" "$S/run-$1"
}

# instr <N>: total instructions:u of the bench at N iterations; empty when the run failed or did
# not print the bench's result line (a renamed bench matches nothing and exits 0). The bench's own
# output goes to $S/run-<N> so a skip (no edh table) is told from a crash. -test.v is needed for
# that: without it a skipped bench prints nothing, only PASS, as does a bench that matches nothing.
instr() {
	local rc
	(cd "$top/dirac" && GOGC=off GOMEMLIMIT=3GiB GOMAXPROCS=1 taskset -c "$INSTR_CORE" perf stat -x, -e instructions:u "$S/dirac.test" -test.v -test.run XXX -test.bench "^$BENCH\$" -test.benchtime "${1}x") >"$S/run-$1" 2>&1
	rc=$?
	echo "$rc" >"$S/rc-$1"
	[ "$rc" -eq 0 ] || return 0
	grep -qE '^(panic:|fatal error:|--- FAIL|FAIL|--- SKIP)' "$S/run-$1" && return 0
	has_result "$1" || return 0
	awk -F, '$3 ~ /^instructions/ && $1 ~ /^[0-9]+$/ {print $1}' "$S/run-$1"
}

why() {
	local f
	f=$(grep -m1 -E '^(panic:|fatal error:|--- FAIL|FAIL|taskset:)' "$S/run-$1")
	if grep -q 'no edh table' "$S/run-$1"; then
		echo "no edh table (make dirac-tables)"
	elif [ -n "$f" ] || [ "$(cat "$S/rc-$1")" != 0 ]; then
		echo "the bench crashed at ${1}x (exit $(cat "$S/rc-$1")): $f"
	elif has_result "$1"; then
		echo "perf gave no instructions:u count at ${1}x (exit 0)"
	else
		echo "no result line at ${1}x: bench renamed or dropped? (exit 0)"
	fi
}

lo=$(instr "$N_LO")
[ -n "$lo" ] || infra "$(why "$N_LO")"
hi=$(instr "$N_HI")
[ -n "$hi" ] || infra "$(why "$N_HI")"
head=$(((hi - lo) / (N_HI - N_LO)))

echo "$BENCH: head $head instr/op, base $base (max ${MAX}x), enforce=$ENFORCE"
if awk -v h="$head" -v b="$base" -v x="$MAX" 'BEGIN{exit !(h / b > x)}'; then
	msg="$BENCH: head $head is over ${MAX}x the base's $base instructions"
	if [ "$ENFORCE" = 1 ]; then
		echo "dirac instr batch: FAIL -- $msg"
		exit 1
	fi
	echo "dirac instr batch: report only (INSTR_EDH4_ENFORCE unset) -- $msg"
fi
exit 0
