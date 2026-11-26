#!/usr/bin/env bash
# ci-sentinels.sh <shard> <shards> [procs]: run one shard of the oracle sentinels (ci.yml, oracle-sentinels).
# ci-sentinels.sh --plan <shards> [procs]: print "<shard> <proc> <seconds> <test>" for every sentinel, run nothing.
#
# The sentinels are the tests named in dirac/oracle/testdata/sentinels.txt. Most of them flip package-level mutation
# switches (dirac.Debug*), so they cannot call t.Parallel and one go test runs them one after another: about 560 s of
# serial tests in 860 s of measured test time (11-26). Separate processes share no globals, so each shard runs
# <procs> go test processes at once. The sentinels are dealt to the shards*procs processes by measured cost, heaviest
# first, each to the process with the least so far (dirac/oracle/testdata/sentinel-seconds.txt: "<test> <seconds>"; a
# sentinel it does not list weighs DEFAULT_SECONDS). The deal is a pure function of the two files, so every shard
# computes the same one. Before 11-26 the shards took every third name: one go test each, 225-265 s of test time.
#
# A skip or a name that matches nothing must not read green: every sentinel must print its own "--- PASS: <name> (".
set -euo pipefail
export LC_ALL=C
usage="usage: ci-sentinels.sh <shard> <shards> [procs] | --plan <shards> [procs]"
plan=0
if [ "${1:-}" = --plan ]; then plan=1; shift; set -- -1 "$@"; fi
shard=${1:?$usage}
shards=${2:?$usage}
procs=${3:-4}
list=dirac/oracle/testdata/sentinels.txt
weights=dirac/oracle/testdata/sentinel-seconds.txt
[ -r "$weights" ] || weights=/dev/null

all=$(grep -vE '^[[:space:]]*(#|$)' "$list" | sed 's/[[:space:]]*#.*//; s/[[:space:]]*$//' | sort -u)
[ -n "$all" ] || { echo "ci-sentinels: $list names no tests"; exit 1; }
# "<bin> <seconds> <test>", bin = shard*procs + proc: heaviest first (ties by name), each to the lightest bin so far.
dealt=$(awk -v def="${DEFAULT_SECONDS:-10}" '
		FILENAME == ARGV[1] { if ($0 !~ /^#/ && NF == 2) w[$1] = $2; next }
		NF { printf "%.1f %s\n", (($1 in w) ? w[$1] : def), $1 }
	' "$weights" - <<<"$all" | sort -k1,1gr -k2,2 |
	awk -v n=$((shards * procs)) '{ b = 0; for (k = 1; k < n; k++) if (l[k] < l[b]) b = k; l[b] += $1; print b, $1, $2 }')
if [ "$plan" = 1 ]; then
	awk -v p="$procs" '{ print int($1 / p), $1 % p, $2, $3 }' <<<"$dealt" | sort -k1,1n -k2,2n -k4,4
	exit 0
fi

tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT
want=0
for ((p = 0; p < procs; p++)); do
	awk -v b=$((shard * procs + p)) '$1 == b { print $3 }' <<<"$dealt" >"$tmp/names.$p"
	want=$((want + $(wc -l <"$tmp/names.$p")))
done
[ "$want" -gt 0 ] || { echo "ci-sentinels: shard $shard of $shards: no sentinels"; exit 0; }
echo "ci-sentinels: $want of $(wc -l <<<"$all") sentinels (shard $shard of $shards, $procs processes)"

# Compile once, so the processes below only link.
go test -c -o "$tmp/oracle.test" ./dirac/oracle
rm -f "$tmp/oracle.test"

pids=()
for ((p = 0; p < procs; p++)); do
	[ -s "$tmp/names.$p" ] || continue
	rx=$(paste -sd'|' "$tmp/names.$p")
	est=$(awk -v b=$((shard * procs + p)) '$1 == b { s += $2 } END { printf "%.0f", s }' <<<"$dealt")
	echo "ci-sentinels: process $p: $(wc -l <"$tmp/names.$p") sentinels, ${est} s measured"
	( start=$SECONDS; rc=0
	  go test -v -timeout 15m -run "^($rx)\$" ./dirac/oracle >"$tmp/log.$p" 2>&1 || rc=$?
	  echo "$rc $((SECONDS - start))" >"$tmp/rc.$p" ) &
	pids+=($!)
done
wait "${pids[@]}" || true

rc=0
for ((p = 0; p < procs; p++)); do
	[ -s "$tmp/names.$p" ] || continue
	read -r prc secs <"$tmp/rc.$p" || { prc=1; secs=?; }
	if [ "$prc" -eq 0 ]; then
		echo "::group::ci-sentinels: process $p: PASS in ${secs} s"
		cat "$tmp/log.$p"
		echo "::endgroup::"
	else
		echo "ci-sentinels: process $p: FAIL (exit $prc) in ${secs} s"
		cat "$tmp/log.$p"
		rc=$prc
	fi
	# Every name must print its own top-level --- PASS line: a skip, or a name that matches nothing, is a failure.
	missing=$(while read -r n; do grep -q "^--- PASS: $n (" "$tmp/log.$p" || echo "$n"; done <"$tmp/names.$p")
	if [ -n "$missing" ]; then
		echo "::error::oracle-sentinels: process $p: these sentinels did not pass (missing corpus or tables, a skip, a failure, or a name that matches nothing): $(paste -sd' ' <<<"$missing")"
		grep -E '^--- SKIP: ' "$tmp/log.$p" || true
		[ "$rc" -ne 0 ] || rc=1
	fi
done
[ "$rc" -eq 0 ] || exit "$rc"
echo "ci-sentinels: all $want sentinels of shard $shard passed"
