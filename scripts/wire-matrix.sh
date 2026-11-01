#!/usr/bin/env bash
# wire-matrix.sh commit|nightly|pr: runs each engine / wire / consumer cell that
# has a test, one line per cell, and exits non-zero if any cell fails. A skipped
# test is a failure, so a missing corpus never reads green.
#
# docs/site/architecture/wires.md is the same table for people to read.
# TestWireMatrixPageMatchesScript fails on a cell in one and not the other.
#
#   scripts/wire-matrix.sh commit
#
# WIRE_MATRIX_PORT is the port the HTTP agent cell binds (default 8097).
# CONFORMANCE_SEED is the nightly fuzz seed (default: days since 2026-01-01).
set -euo pipefail

usage() { sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//'; }

tier=${1:-}
case "$tier" in
commit | nightly | pr) ;;
-h | --help) usage; exit 0 ;;
*) usage >&2; exit 2 ;;
esac

root=$(git rev-parse --show-toplevel)
cd "$root"

if [ ! -s .cards/cards.lock ]; then
	echo "wire-matrix.sh: no .cards corpus: every corpus test would skip (make fetch-cards compile-cards)" >&2
	exit 1
fi

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
port=${WIRE_MATRIX_PORT:-8097}
seed=${CONFORMANCE_SEED:-$((($(date -u +%s) - $(date -u -d 2026-01-01 +%s)) / 86400))}
failed=0

# row CELL TIERS COMMAND...: runs COMMAND when TIERS holds this run's tier.
row() {
	local cell=$1 tiers=$2
	log="$work/${cell//\//-}.log"
	shift 2
	case " $tiers " in
	*" $tier "*) ;;
	*) return 0 ;;
	esac
	if "$@" >"$log" 2>&1 && ! grep -q -- '--- SKIP' "$log"; then
		echo "$cell PASS"
	else
		echo "$cell FAIL"
		grep -- '--- SKIP' "$log" || true
		tail -20 "$log"
		failed=1
	fi
}

# tests PKG NAME...: runs the named tests of one package, each of which must pass.
# go test exits 0 on a name that matches nothing, so each one is looked for.
tests() {
	local pkg=$1 name pattern output="$work/tests.log"
	shift
	pattern=$(IFS='|'; echo "$*")
	go test -v -timeout 5m -run "^($pattern)\$" "$pkg" | tee "$output" || return 1
	for name in "$@"; do
		grep -q -- "^--- PASS: $name " "$output" || { echo "$name did not pass"; return 1; }
	done
}

# The hosted-bots cell is several commands.
hosted_bots() {
	tests ./engine/rules TestHeads &&
		make sim &&
		tests ./bots/all TestRegisteredBotsConsumePaymentPlans
}

fuzz() {
	go build -trimpath -o "$work/cardfuzz" ./cmd/cardfuzz &&
		"$work/cardfuzz" -dir .cards -manabrew -games 2000 -seed "$seed" -workers 4 \
			-state "$work/fuzz.state" -failures "$work/fuzz.jsonl" &&
		[ ! -s "$work/fuzz.jsonl" ]
}

row gorge/native/harness 'commit nightly pr' tests ./testkit/conformance TestNativeConformanceGorgeSelf
row gorge/native/hosted-bots 'commit nightly pr' hosted_bots
row gorge/native/http-agent 'commit nightly pr' scripts/examples-smoke.sh "$port"
row gorge/manabrew/harness 'commit nightly pr' tests ./testkit/conformance TestManaBrewConformanceGorgeSelf TestManaBrewNormaliserLines TestManaBrewCatchesAWrongLife TestManaBrewCatchesAMissingAction
row gorge/manabrew/bot 'commit nightly pr' go test -v -timeout 10m ./server/manabrew/mbtest ./server/host/manabrewhttp
row gorge/manabrew/fuzz 'nightly' fuzz
row gorge/v2/harness 'commit nightly pr' go test -v -timeout 15m ./server/spellbench/... ./server/host/sbv2http
row gorge/v2/stdio-agent 'commit nightly pr' tests ./server/spellbench/seat TestV2AgentSeatPlaysGorgeAgent TestSpellbenchSeatAppliesAgentPick TestSpellbenchSeatAnswersUnposableDecisionWithFallback

exit "$failed"
