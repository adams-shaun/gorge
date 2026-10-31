#!/usr/bin/env bash
# hostbench-pair.sh BASE CAND: compare what a hosted bot match costs at two git
# revisions, in user-space instructions per match, read with perf stat.
#
# Each revision's server/host test binary is built from a git archive, so no
# checkout is touched, and runs BenchmarkHostBotMatch at N=4 and N=20 with the
# collector off on one core. The difference between the two counts cancels
# corpus load and setup, and leaves 16 fixed seeds. Both binaries run from this
# checkout's server/host, so they read one corpus and one set of deck files.
#
# Prints instr/op and allocs/op per revision and CAND/BASE's ratio. A missing
# perf or bench is a failure; judging the ratio is left to the reader.
# BENCH_DIR, MEM_MAX and BENCH_GOMEMLIMIT are as in enginebench-lib.sh.
set -euo pipefail
shopt -s inherit_errexit
. "$(dirname "$0")/enginebench-lib.sh"
base_rev=${1:?usage: hostbench-pair.sh BASE CAND}
cand_rev=${2:?usage: hostbench-pair.sh BASE CAND}
if [ -z "$(perf stat -x, -e instructions:u true 2>&1 | awk -F, '$1 ~ /^[0-9]+$/')" ]; then
	echo "hostbench-pair: perf stat cannot count instructions:u" >&2
	exit 1
fi
work=$(mktemp -d "$BENCH_DIR/hostbench.XXXXXX")
trap 'rm -rf "$work"' EXIT

# build REV: print the path of REV's test binary.
build() {
	local sha src
	sha=$(git -C "$REPO" rev-parse --verify "$1^{commit}")
	src=$work/$sha
	if [ ! -d "$src" ]; then
		mkdir "$src"
		git -C "$REPO" archive "$sha" | tar -x -C "$src"
		(cd "$src" && GOWORK=off go test -c -trimpath -o host.test ./server/host)
	fi
	echo "$src/host.test"
}

# measure BIN N: print "INSTRUCTIONS ALLOCS_PER_OP" for N iterations, each the
# median of five runs. The corpus load dwarfs the 16 matches, and its count
# varies enough between single runs to move the ratio by about 0.3%.
measure() {
	: >"$work/instr"
	: >"$work/allocs"
	for _ in 1 2 3 4 5; do
		(cd "$REPO/server/host" && heavy env GOGC=off GOMAXPROCS=1 perf stat -x, -e instructions:u -o "$work/perf" \
			"$1" -test.run '^$' -test.bench '^BenchmarkHostBotMatch$' -test.benchtime "$2x" -test.benchmem) >"$work/out"
		awk -F, '/instructions/ {print $1}' "$work/perf" >>"$work/instr"
		awk '/^BenchmarkHostBotMatch/ {for (i = 2; i <= NF; i++) if ($i == "allocs/op") print $(i-1)}' "$work/out" >>"$work/allocs"
	done
	if [ "$(wc -l <"$work/instr")" -ne 5 ] || [ "$(wc -l <"$work/allocs")" -ne 5 ]; then
		echo "hostbench-pair: $1 reported no bench result:" >&2
		cat "$work/out" >&2
		exit 1
	fi
	echo "$(sort -n "$work/instr" | sed -n 3p) $(sort -n "$work/allocs" | sed -n 3p)"
}

# per_op BIN: print "INSTR_PER_OP ALLOCS_PER_OP".
per_op() {
	local low high
	low=$(measure "$1" 4)
	high=$(measure "$1" 20)
	echo "$(((${high% *} - ${low% *}) / 16)) ${high#* }"
}

base=$(build "$base_rev")
cand=$(build "$cand_rev")
base_cost=$(per_op "$base")
cand_cost=$(per_op "$cand")
echo "base $base_rev: ${base_cost% *} instr/op, ${base_cost#* } allocs/op"
echo "cand $cand_rev: ${cand_cost% *} instr/op, ${cand_cost#* } allocs/op"
awk -v c="${cand_cost% *}" -v b="${base_cost% *}" 'BEGIN { printf "ratio cand/base: %.4f\n", c / b }'
