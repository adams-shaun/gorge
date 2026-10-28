#!/usr/bin/env bash
# sb-parity.sh [-games N] [-seed S]: the SpellBench v2 parity gate, in one
# build and three cardfuzz runs. A regression in either direction -- a bot
# whose v2 neutral world disagrees with its native play, or whose outcome
# shifts through the v2 seat -- turns this script non-zero.
#
#   twin autopay all  -min-agree 99.5
#   twin autopay off  -min-agree 99.5
#   outcome autopay all -max-outcome-gap 10
#
# Defaults: 200 games (the nightly size); -games 20 is the smoke size. -seed
# defaults to cardfuzz's 1. Every run uses 2 workers. The binary is built once
# with -trimpath into a temporary directory under bin/ (gitignored) and removed
# on exit.
#
# Usage:
#   scripts/sb-parity.sh                 # nightly size
#   scripts/sb-parity.sh -games 20       # smoke
#   scripts/sb-parity.sh -games 200 -seed 2
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
games=200
seed=1
while [ $# -gt 0 ]; do
	case "$1" in
	-games) games=${2:?sb-parity.sh: -games needs a count}; shift 2 ;;
	-seed) seed=${2:?sb-parity.sh: -seed needs a value}; shift 2 ;;
	-h | --help) sed -n '2,18p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
	*) echo "sb-parity.sh: unknown argument: $1 (want -games N or -seed S)" >&2; exit 2 ;;
	esac
done

if [ ! -d "$root/.cards" ]; then
	echo "sb-parity.sh: $root/.cards is missing; link the corpus first (see AGENTS.md)" >&2
	exit 1
fi

mkdir -p "$root/bin"
tmp=$(mktemp -d "$root/bin/sb-parity.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
bin=$tmp/cardfuzz
(cd "$root" && go build -trimpath -o "$bin" ./cmd/cardfuzz)

fail=0
# run LABEL -spellbench MODE -autopay MODE -min-agree|-max-outcome-gap VALUE
# Builds one result line per case; a failure also prints the failing detail
# lines cardfuzz wrote.
run() {
	local label=$1
	shift
	local log=$tmp/$label.log
	local rc=0
	"$bin" -dir "$root/.cards" -games "$games" -seed "$seed" -workers 2 \
		-failures "$tmp/failures-$label.jsonl" "$@" >"$log" 2>&1 || rc=$?

	local detail
	case "$label" in
	twin-*) detail=$(grep -m1 '^agreement ' "$log" || true) ;;
	outcome-*) detail=$(grep -m1 '^outcome parity:' "$log" || true) ;;
	esac
	[ -n "$detail" ] || detail="(no summary: see $log)"

	if [ "$rc" -eq 0 ]; then
		printf '%s seed=%d games=%d: PASS - %s\n' "$label" "$seed" "$games" "$detail"
		return 0
	fi
	fail=1
	printf '%s seed=%d games=%d: FAIL - %s\n' "$label" "$seed" "$games" "$detail"
	case "$label" in
	twin-*) grep -E '^  .*FAIL$' "$log" >&2 || true ;;
	outcome-*) grep -E '^cardfuzz: spellbench outcome ' "$log" >&2 || true ;;
	esac
	return 0
}

run twin-autopay-all -spellbench twin -autopay all -min-agree 99.5
run twin-autopay-off -spellbench twin -autopay off -min-agree 99.5
run outcome-autopay-all -spellbench outcome -autopay all -max-outcome-gap 10

exit "$fail"
