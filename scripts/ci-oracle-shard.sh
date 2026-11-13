#!/usr/bin/env bash
# ci-oracle-shard.sh <shard> <shards> [go test flags...]: one part of ./dirac/oracle.
#
# The package plays its lockstep games serially, past 30 minutes on a 4-core runner, so CI splits
# it across a matrix. The test list comes from the package, so a new test needs no edit here. The
# tests are dealt by measured cost, longest first, each to the part with the least so far
# (dirac/oracle/testdata/ci-shard-seconds.txt: "<test> <seconds>"; a test it does not list weighs
# DEFAULT_SECONDS, and a pool-only test weighs nothing on a clone with no pool, where it skips). The
# deal is a pure function of the test list and that file, so every part computes the same one. A part
# ends by printing its slowest tests, the input of the next refresh of the file. By name, round-robin,
# the parts measured 574-1184 s on one promote PR (2026-11-12): a few heavy tests shared a part.
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
pools=${GORGE_POOLS:-.pools}
havepools=0
if [ -d "$pools/fdn/decks" ] && [ -d "$pools/blb/decks" ]; then havepools=1; fi
weights=dirac/oracle/testdata/ci-shard-seconds.txt
[ -r "$weights" ] || weights=/dev/null
# "<seconds> <test>" per test, heaviest first (ties by name), then a greedy deal: part k gets the next
# test while it carries the least. awk keeps the arithmetic exact enough and the order fixed (LC_ALL=C).
mapfile -t dealt < <(
	awk -v nopool=$((1 - havepools)) -v def="${DEFAULT_SECONDS:-0.08}" '
		FILENAME == ARGV[1] { if ($0 !~ /^#/ && NF == 2) w[$1] = $2; next }
		FILENAME == ARGV[2] { if ($0 !~ /^#/ && NF) { split($0, a, "/"); skip[a[1]] = 1 }; next }
		NF { s = ($1 in w) ? w[$1] : def; if (nopool && ($1 in skip)) s = 0; printf "%.3f %s\n", s, $1 }
	' "$weights" dirac/oracle/testdata/poolskips.txt - <<<"$list" | sort -k1,1gr -k2,2 |
	awk -v n="$n" '{ b = 0; for (k = 1; k < n; k++) if (l[k] < l[b]) b = k; l[b] += $1; print b, $1, $2 }'
)
sel=()
est=0
for row in "${dealt[@]}"; do
	read -r part secs name <<<"$row"
	if [ "$part" -eq "$i" ]; then sel+=("$name"); est=$(awk -v a="$est" -v b="$secs" 'BEGIN { printf "%.0f", a + b }'); fi
done
[ ${#sel[@]} -gt 0 ] || { echo "ci-oracle-shard: part $i of $n is empty"; exit 0; }

tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT
if [ "$havepools" = 1 ]; then
	echo "ci-oracle-shard: deck pools at $pools: no test may skip"
	: >"$tmp/allowed"
else
	echo "ci-oracle-shard: no deck pools: committed slices and synthetic decks only"
	printf '%s\n' "${sel[@]}" >"$tmp/selected"
	grep -v '^#' dirac/oracle/testdata/poolskips.txt | awk -F/ 'NR == FNR { s[$0]; next } $1 in s' "$tmp/selected" - | sort -u >"$tmp/allowed"
fi

re=$(IFS='|'; echo "${sel[*]}")
echo "ci-oracle-shard: part $i of $n, ${#sel[@]} of ${#dealt[@]} tests, ${est} s of measured test time"
rc=0
go test "$@" -v -run "^($re)\$" "$pkg" >"$tmp/log" 2>&1 || rc=$?
if [ "$rc" -ne 0 ]; then cat "$tmp/log"; fi
sed -nE 's/^ *--- SKIP: ([^ ]+) .*/\1/p' "$tmp/log" | sort -u >"$tmp/skipped"
grep -E '^(ok|FAIL|panic:)' "$tmp/log" || true
echo "ci-oracle-shard: slowest tests of part $i (refresh $weights from these):"
sed -nE 's/^--- (PASS|FAIL): (Test[^ ]*) \(([0-9.]+)s\)$/\2 \3/p' "$tmp/log" | sort -k2,2gr | head -15 | sed 's/^/  /'
echo "ci-oracle-shard: $(wc -l <"$tmp/skipped") skips, $(wc -l <"$tmp/allowed") expected"
unexpected=$(comm -23 "$tmp/skipped" "$tmp/allowed")
ran=$(comm -13 "$tmp/skipped" "$tmp/allowed")
if [ -n "$unexpected" ]; then echo "ci-oracle-shard: FAIL: these skipped, which a green run must not hide:"; echo "$unexpected"; rc=1; fi
if [ -n "$ran" ]; then echo "ci-oracle-shard: FAIL: these ran, so delete them from poolskips.txt:"; echo "$ran"; rc=1; fi
exit "$rc"
