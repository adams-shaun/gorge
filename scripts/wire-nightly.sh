#!/usr/bin/env bash
# wire-nightly.sh [-first N] [-chunks N] [-games N] [-parity-games N] [-skip-fuzz] [-skip-parity]: the wires'
# nightly run, on a seed that moves each day. It plays chunks of games on every wire, then
# fuzzed decks through ManaBrew, then SpellBench v2 parity, printing one line per
# row. It exits non-zero if a chunk or the fuzz fails. A v2 parity failure is
# printed and does not change the exit status (docs/site/architecture/wires.md).
#
# -first lets CI shard the chunk range across jobs; a shard skips the fuzz and
# parity stages and one job runs them alone. Each chunk is a separate go test,
# so a shard is a few short invocations rather than one long one.
#
# A failed game writes wire-found-<date>.tsv in the repo root: wire, set, seed
# and pair, the columns of testkit/conformance/testdata/found.tsv. A fuzz failure
# writes cardfuzz's records to wire-found-<date>-fuzz.jsonl beside it, which
# cardfuzz -repro reads.
#
#   scripts/wire-nightly.sh                    # 20 chunks, 2000 fuzz games
#   CONFORMANCE_SEED=5 scripts/wire-nightly.sh -chunks 1 -games 20
#   scripts/wire-nightly.sh -first 5 -chunks 5 -skip-fuzz -skip-parity
#
# CONFORMANCE_SEED defaults to the days since 2026-01-01. -parity-games defaults
# to 2000, or to -games when that is smaller.
set -euo pipefail

usage() { sed -n '2,22p' "$0" | sed 's/^# \{0,1\}//'; }

chunks=20
first=0
games=2000
parity_games=
skip_fuzz=
skip_parity=
while [ $# -gt 0 ]; do
	case "$1" in
	-first) first=${2:?wire-nightly.sh: -first needs a count}; shift 2 ;;
	-chunks) chunks=${2:?wire-nightly.sh: -chunks needs a count}; shift 2 ;;
	-games) games=${2:?wire-nightly.sh: -games needs a count}; shift 2 ;;
	-parity-games) parity_games=${2:?wire-nightly.sh: -parity-games needs a count}; shift 2 ;;
	-skip-fuzz) skip_fuzz=1; shift ;;
	-skip-parity) skip_parity=1; shift ;;
	-h | --help) usage; exit 0 ;;
	*) usage >&2; exit 2 ;;
	esac
done
if [ -z "$parity_games" ]; then
	parity_games=$((games < 2000 ? games : 2000))
fi
if [ "$chunks" -eq 0 ] && [ -n "$skip_fuzz" ] && [ -n "$skip_parity" ]; then
	echo "wire-nightly.sh: nothing to do: -chunks 0 with both -skip-fuzz and -skip-parity" >&2
	exit 2
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
# A 10-pair chunk measured 27 s quiet on 4 cores (make test-one PKG=./testkit/conformance
# RUN='^TestConformanceChunk$' with CONFORMANCE_PAIRS=10). Its timeout is 2x that, so a
# genuine hang fails quickly instead of consuming the old 5 m budget.
chunk_timeout=1m
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
		go test -v -timeout "$chunk_timeout" -run '^TestConformanceChunk$' ./testkit/conformance >"$output" 2>&1 || status=$?
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

for ((n = first; n < first + chunks; n++)); do
	run "chunk-$n" chunk "$n" || failed=1
done
[ -n "$skip_fuzz" ] || run manabrew-fuzz fuzz || failed=1
[ -n "$skip_parity" ] || run v2-parity scripts/sb-parity.sh -games "$parity_games" -seed "$seed" || echo "v2-parity is reported, and does not fail this run"

exit "$failed"
