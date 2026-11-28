#!/usr/bin/env bash
# ci-oracle-shard.sh <shard> <shards> [go test flags...]: one part of ./dirac/oracle.
#
# The package plays its lockstep games serially, past 30 minutes on a 4-core runner, so CI splits
# it across a matrix. The test list comes from the package, so a new test needs no edit here. The
# tests are dealt by measured cost, longest first, each to the part with the least so far
# (dirac/oracle/testdata/ci-shard-seconds.txt: "<test> <seconds>"; a test it does not list weighs
# DEFAULT_SECONDS, and a pool-only test weighs nothing on a clone with no pool, where it skips). The
# deal is a pure function of the test list and that file, so every part computes the same one; it is
# scripts/oracle_shard_deal.py's, shared with the precheck's targeted lanes (scripts/test_targets.py),
# never copied. Part indices are 0-based, as .github/workflows/*'s matrix is. A part ends by printing
# its slowest tests, the input of the next refresh of the file. By name, round-robin, the parts
# measured 574-1184 s on one promote PR (2026-11-12): a few heavy tests shared a part.
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
# TestRefusedEntriesStillDiverge (the dirac gate's entry) checks what its shards 0..7 check, and the shards
# are listed too: dealt apart, the parent would replay every entry again.
list=$(go test -list '.*' "$pkg" | grep -E '^Test' | grep -vx 'TestRefusedEntriesStillDiverge' || true)
[ -n "$list" ] || { echo "ci-oracle-shard: no tests listed in $pkg"; exit 1; }
pools=${GORGE_POOLS:-.pools}
havepools=0
if [ -d "$pools/fdn/decks" ] && [ -d "$pools/blb/decks" ]; then havepools=1; fi
weights=dirac/oracle/testdata/ci-shard-seconds.txt
[ -r "$weights" ] || weights=/dev/null
# "<part> <seconds> <test>" per test: the shared deal, heaviest first (ties by name), each test to the
# part carrying the least so far. A pool-only test weighs nothing on a clone with no pool, where it skips.
zero=()
[ "$havepools" = 1 ] || zero=(--zero dirac/oracle/testdata/poolskips.txt)
mapfile -t dealt < <(
	python3 -I scripts/oracle_shard_deal.py --parts "$n" --weights "$weights" \
		--default "${DEFAULT_SECONDS:-0.08}" ${zero[@]+"${zero[@]}"} <<<"$list"
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
# Every PASS and SKIP line, folded in the UI: a green run's -v timings, which refresh the landing gate's oracle fast tier
# (scripts/oracle_fasttier.py --logs on the parts' logs). Without them only a red part showed its log, and the tier
# went stale: a 0.02 s test added after it (TestMBCPodTables) reached a promote PR red (11-13 #328).
# A red part has printed its whole log above.
if [ "$rc" -eq 0 ]; then
	echo "::group::ci-oracle-shard: PASS and SKIP lines of part $i (scripts/oracle_fasttier.py --logs)"
	grep -E '^ *--- (PASS|SKIP): ' "$tmp/log" || true
	echo "::endgroup::"
fi
grep -E '^(ok|FAIL|panic:)' "$tmp/log" || true
echo "ci-oracle-shard: slowest tests of part $i, summed over their leaf subtests (refresh $weights from these):"
# a test that plays its seeds as parallel subtests reports its wall, queueing included: weigh its leaves
sed -nE 's/^ *--- (PASS|FAIL): (Test[^ ]*) \(([0-9.]+)s\)$/\2 \3/p' "$tmp/log" |
	awk '{ d[$1] = $2; n = split($1, p, "/"); s = p[1]; for (k = 2; k <= n; k++) { par[s] = 1; s = s "/" p[k] } }
		END { for (t in d) if (!(t in par)) { split(t, p, "/"); w[p[1]] += d[t] }; for (t in w) printf "%s %.1f\n", t, w[t] }' |
	sort -k2,2gr | sed -n 1,15p | sed 's/^/  /'
echo "ci-oracle-shard: $(wc -l <"$tmp/skipped") skips, $(wc -l <"$tmp/allowed") expected"
unexpected=$(comm -23 "$tmp/skipped" "$tmp/allowed")
ran=$(comm -13 "$tmp/skipped" "$tmp/allowed")
if [ -n "$unexpected" ]; then echo "ci-oracle-shard: FAIL: these skipped, which a green run must not hide:"; echo "$unexpected"; rc=1; fi
if [ -n "$ran" ]; then echo "ci-oracle-shard: FAIL: these ran, so delete them from poolskips.txt:"; echo "$ran"; rc=1; fi
exit "$rc"
