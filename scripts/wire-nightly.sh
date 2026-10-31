#!/usr/bin/env bash
# wire-nightly.sh [-chunks N] [-games N] [-parity-games N]: the wire program's
# nightly run, on a seed that moves each day. It plays chunks of games on every
# wire, then fuzzed decks through ManaBrew, then SpellBench v2 parity, printing
# one line per row and exiting non-zero if a row fails. v2 parity is reported and
# does not fail the run: its bar starts red (docs/site/architecture/wires.md).
#
# A failed game writes wire-found-<date>.tsv in the repo root: wire, set, seed
# and pair, the columns of testkit/conformance/testdata/found.tsv. A fuzz failure
# writes cardfuzz's records to wire-found-<date>-fuzz.jsonl beside it, which
# cardfuzz -repro reads.
#
#   scripts/wire-nightly.sh                    # 20 chunks, 2000 fuzz games
#   CONFORMANCE_SEED=5 scripts/wire-nightly.sh -chunks 1 -games 20
#
# CONFORMANCE_SEED defaults to the days since 2026-01-01. -parity-games defaults
# to 2000, or to -games when that is smaller.
set -euo pipefail

usage() { sed -n '2,17p' "$0" | sed 's/^# \{0,1\}//'; }

chunks=20
games=2000
parity_games=
while [ $# -gt 0 ]; do
	case "$1" in
	-chunks) chunks=${2:?wire-nightly.sh: -chunks needs a count}; shift 2 ;;
	-games) games=${2:?wire-nightly.sh: -games needs a count}; shift 2 ;;
	-parity-games) parity_games=${2:?wire-nightly.sh: -parity-games needs a count}; shift 2 ;;
	-h | --help) usage; exit 0 ;;
	*) usage >&2; exit 2 ;;
	esac
done
if [ -z "$parity_games" ]; then
	parity_games=$((games < 2000 ? games : 2000))
fi

root=$(git rev-parse --show-toplevel)
cd "$root"

if [ ! -s .cards/cards.lock ]; then
	echo "wire-nightly.sh: no .cards corpus: every corpus test would skip (make fetch-cards compile-cards)" >&2
	exit 1
fi

seed=${CONFORMANCE_SEED:-$((($(date -u +%s) - $(date -u -d 2026-01-01 +%s)) / 86400))}
echo "seed $seed"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
chunk_pairs=10
failed=0
found="$root/wire-found-$(date -u +%F).tsv"

# run NAME COMMAND...: prints NAME PASS or FAIL, and the end of the log on a failure.
run() {
	local name=$1 log="$work/$1.log"
	shift
	if "$@" >"$log" 2>&1; then
		echo "$name PASS"
		return 0
	fi
	echo "$name FAIL"
	tail -20 "$log"
	return 1
}

# chunk N: one chunk of pairs. A test that did not run to a pass is a failure.
chunk() {
	local n=$1 output="$work/chunk-output-$n.log" status=0
	CONFORMANCE_SEED=$seed CONFORMANCE_FIRST=$((n * chunk_pairs)) CONFORMANCE_PAIRS=$chunk_pairs \
		go test -v -timeout 5m -run '^TestConformanceChunk$' ./testkit/conformance >"$output" 2>&1 || status=$?
	cat "$output"
	if grep -q '^FOUND' "$output"; then
		grep '^FOUND' "$output" | cut -f2- >>"$found"
	fi
	if grep -q -- '--- SKIP' "$output" || ! grep -q -- '^--- PASS: TestConformanceChunk ' "$output"; then
		return 1
	fi
	return "$status"
}

fuzz() {
	go build -trimpath -o "$work/cardfuzz" ./cmd/cardfuzz &&
		"$work/cardfuzz" -dir .cards -manabrew -games "$games" -seed "$seed" -workers 4 \
			-state "$work/fuzz.state" -failures "$work/fuzz.jsonl" || return 1
	if [ -s "$work/fuzz.jsonl" ]; then
		cp "$work/fuzz.jsonl" "${found%.tsv}-fuzz.jsonl"
		return 1
	fi
}

for ((n = 0; n < chunks; n++)); do
	run "chunk-$n" chunk "$n" || failed=1
done
run manabrew-fuzz fuzz || failed=1
run v2-parity scripts/sb-parity.sh -games "$parity_games" -seed "$seed" || echo "v2-parity is reported, and does not fail this run"

exit "$failed"
