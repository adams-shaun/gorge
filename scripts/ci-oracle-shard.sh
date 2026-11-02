#!/usr/bin/env bash
# ci-oracle-shard.sh <shard> <shards> [go test flags...]: one part of ./dirac/oracle.
#
# The package plays its lockstep games serially, past 30 minutes on a 4-core runner, so CI splits
# it across a matrix. The test list comes from the package, so a new test needs no edit here. The
# Lockstep tests, the heavy ones, are dealt round-robin by sorted name and every other test
# continues the same deal: part i runs the tests at positions i, i+N, i+2N, ...
#
# A skip must not read green. A clone with no deck pool plays committed slices and synthetic decks,
# where exactly the tests in dirac/oracle/testdata/poolskips.txt skip; with the FDN and BLB pools
# (GORGE_POOLS, or .pools at the module root) none may. A part fails on any other skip, and on a
# listed test that ran.
set -euo pipefail
export LC_ALL=C # comm and sort must agree
i=${1:?usage: ci-oracle-shard.sh <shard> <shards> [go test flags...]}
n=${2:?usage: ci-oracle-shard.sh <shard> <shards> [go test flags...]}
shift 2
pkg=./dirac/oracle
list=$(go test -list '.*' "$pkg" | grep -E '^Test' || true)
[ -n "$list" ] || { echo "ci-oracle-shard: no tests listed in $pkg"; exit 1; }
mapfile -t all < <({ grep 'Lockstep' <<<"$list" | sort; grep -v 'Lockstep' <<<"$list" | sort; })
sel=()
for k in "${!all[@]}"; do
	if [ $((k % n)) -eq "$i" ]; then sel+=("${all[k]}"); fi
done
[ ${#sel[@]} -gt 0 ] || { echo "ci-oracle-shard: part $i of $n is empty"; exit 0; }

tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT
pools=${GORGE_POOLS:-.pools}
if [ -d "$pools/fdn/decks" ] && [ -d "$pools/blb/decks" ]; then
	echo "ci-oracle-shard: deck pools at $pools: no test may skip"
	: >"$tmp/allowed"
else
	echo "ci-oracle-shard: no deck pools: committed slices and synthetic decks only"
	printf '%s\n' "${sel[@]}" >"$tmp/selected"
	grep -v '^#' dirac/oracle/testdata/poolskips.txt | awk -F/ 'NR == FNR { s[$0]; next } $1 in s' "$tmp/selected" - | sort -u >"$tmp/allowed"
fi

re=$(IFS='|'; echo "${sel[*]}")
echo "ci-oracle-shard: part $i of $n, ${#sel[@]} of ${#all[@]} tests"
rc=0
go test "$@" -v -run "^($re)\$" "$pkg" >"$tmp/log" 2>&1 || rc=$?
if [ "$rc" -ne 0 ]; then cat "$tmp/log"; fi
sed -nE 's/^ *--- SKIP: ([^ ]+) .*/\1/p' "$tmp/log" | sort -u >"$tmp/skipped"
grep -E '^(ok|FAIL|panic:)' "$tmp/log" || true
echo "ci-oracle-shard: $(wc -l <"$tmp/skipped") skips, $(wc -l <"$tmp/allowed") expected"
unexpected=$(comm -23 "$tmp/skipped" "$tmp/allowed")
ran=$(comm -13 "$tmp/skipped" "$tmp/allowed")
if [ -n "$unexpected" ]; then echo "ci-oracle-shard: FAIL: these skipped, which a green run must not hide:"; echo "$unexpected"; rc=1; fi
if [ -n "$ran" ]; then echo "ci-oracle-shard: FAIL: these ran, so delete them from poolskips.txt:"; echo "$ran"; rc=1; fi
exit "$rc"
