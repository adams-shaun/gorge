#!/usr/bin/env bash
# instr-measure.sh <test-binary> <package-dir> <benchmark>
#
# Instructions per op of one dirac transition bench, measured by exactly the
# method scripts/dirac-gate.sh step 5 uses: the bench binary runs pinned to one
# core (INSTR_CORE, default 20) under GOGC=off GOMAXPROCS=1 and perf stat -x, -e
# instructions:u, at 100000x and 500000x, and the counts are differenced over
# 400000 ops. scripts/instr_measure_test.py holds the two copies together, so a
# change to the gate's method that this one does not get is red.
#
# Prints the integer instructions/op on stdout. A missing perf or taskset, a
# perf_event_paranoid above 1, a bench that crashes and a count perf did not
# report are errors (exit 2), never skips: the measurement is the check.
set -uo pipefail
export LC_ALL=C # perf's CSV must parse the same way everywhere
bin=${1:?usage: instr-measure.sh <test-binary> <package-dir> <benchmark>}
dir=${2:?usage: instr-measure.sh <test-binary> <package-dir> <benchmark>}
bench=${3:?usage: instr-measure.sh <test-binary> <package-dir> <benchmark>}
INSTR_CORE=${INSTR_CORE:-20}
die() {
	echo "instr-measure: $*" >&2
	exit 2
}

case $INSTR_CORE in '' | *[!0-9]*) die "INSTR_CORE=$INSTR_CORE is not a core number" ;; esac
[ -x "$bin" ] || die "no executable test binary at $bin"
[ -d "$dir" ] || die "no package directory at $dir"
command -v perf >/dev/null 2>&1 || die "perf is not installed: self-hosted runners need it for instructions:u"
if [ -r /proc/sys/kernel/perf_event_paranoid ]; then
	paranoid=$(cat /proc/sys/kernel/perf_event_paranoid) || die "cannot read /proc/sys/kernel/perf_event_paranoid"
	case $paranoid in '' | *[!0-9-]*) die "perf_event_paranoid=$paranoid is not a number" ;; esac
	[ "$paranoid" -le 1 ] || die "perf_event_paranoid is $paranoid (need <= 1): instructions:u cannot be counted"
fi
command -v taskset >/dev/null 2>&1 || die "taskset is not installed: the bench must run pinned to one core"
taskset -c "$INSTR_CORE" true 2>/dev/null || die "cannot pin to core $INSTR_CORE: set INSTR_CORE to a core this runner has"
[ -n "$(perf stat -x, -e instructions:u true 2>&1 | awk -F, '$1 ~ /^[0-9]+$/')" ] ||
	die "perf stat unavailable (instructions:u not counted): this step cannot pass without it"

instr() {
	local out rc
	out=$(cd "$2" && GOGC=off GOMEMLIMIT=3GiB GOMAXPROCS=1 taskset -c "$INSTR_CORE" perf stat -x, -e instructions:u "$1" -test.run XXX -test.bench "^$3\$" -test.benchtime "${4}x" 2>&1 >/dev/null)
	rc=$?
	[ "$rc" -eq 0 ] || return 0
	grep -qE '^(panic:|fatal error:|--- FAIL|FAIL)' <<<"$out" && return 0
	awk -F, '$3 ~ /^instructions/ && $1 ~ /^[0-9]+$/ {print $1}' <<<"$out"
}
per_op() {
	local a b
	a=$(instr "$1" "$2" "$3" 100000)
	b=$(instr "$1" "$2" "$3" 500000)
	[ -n "$a" ] && [ -n "$b" ] && echo $(((b - a) / 400000))
}
v=$(per_op "$bin" "$dir" "$bench")
[ -n "$v" ] || die "$bench: perf stat reported no instructions:u count at 100000x and 500000x"
printf '%s\n' "$v"
