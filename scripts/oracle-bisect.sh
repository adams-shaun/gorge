#!/usr/bin/env bash
# oracle-bisect.sh <good> <bad> <tests-file>: find the first-parent merge that reddened the oracle.
#
# <tests-file> lists the failing test names, one per line. The script bisects first-parent history between
# <good> (the head of the last oracle run whose oracle jobs passed) and <bad>, and at each step builds dirac's
# tables and runs only those tests. A step with no corpus or no tables is skipped (exit 125, ENV), as is one
# that fails with no `--- FAIL` line (a build break is not the verdict). It prints
#   CULPRIT <sha> PR <n> <subject>
# (and writes culprit= and pr= to $GITHUB_OUTPUT when set) and exits 0; exit 3: no culprit found (all steps skipped).
#
# oracle-bisect.sh --step <tests-file> is one step, run by git bisect from the repo root. Test hooks:
# ORACLE_ENV_CHECK (default: test -s .cards/cards.lock), ORACLE_STEP_BUILD (default: make dirac-tables) and
# ORACLE_STEP_TEST (default: go test over ./dirac/oracle; it gets the -run expression in $RX).
set -euo pipefail

env_check=${ORACLE_ENV_CHECK:-test -s .cards/cards.lock}
step_build=${ORACLE_STEP_BUILD:-make dirac-tables}
# Single-quoted: $RX is set only inside --step and expanded by `bash -c` there (set -u: an unquoted
# default expanded it at startup and killed every bisect, oracle run 45297496438).
step_test=${ORACLE_STEP_TEST:-'go test -timeout 15m -run "^($RX)\$" ./dirac/oracle'}

names() { grep -E '^Test[A-Za-z0-9_]*$' "$1" | sort -u; }

if [ "${1:-}" = --step ]; then
	f=${2:?usage: oracle-bisect.sh --step <tests-file>}
	bash -c "$env_check" || { echo "oracle-bisect: ENV: no corpus at this step"; exit 125; }
	bash -c "$step_build" >&2 || { echo "oracle-bisect: ENV: tables did not build at this step"; exit 125; }
	RX=$(names "$f" | paste -sd'|')
	[ -n "$RX" ] || { echo "oracle-bisect: no test names in $f"; exit 125; }
	export RX
	out=$(mktemp)
	trap 'rm -f -- "$out"' EXIT
	if bash -c "$step_test" >"$out" 2>&1; then exit 0; fi
	tail -n 40 "$out" >&2
	if grep -q -- '--- FAIL' "$out"; then exit 1; fi
	echo "oracle-bisect: step failed without a --- FAIL line: skipping" >&2
	exit 125
fi

good=${1:?usage: oracle-bisect.sh <good> <bad> <tests-file>}
bad=${2:?usage: oracle-bisect.sh <good> <bad> <tests-file>}
tests=$(readlink -f "${3:?usage: oracle-bisect.sh <good> <bad> <tests-file>}")
self=$(readlink -f "$0")
[ -n "$(names "$tests")" ] || { echo "oracle-bisect: $tests names no tests"; exit 2; }

# Never fail silently: name the line that stopped the script.
trap 'echo "oracle-bisect: line $LINENO exited $?" >&2' ERR
git bisect reset >/dev/null 2>&1 || true
trap 'git bisect reset >/dev/null 2>&1 || true' EXIT
git bisect start --first-parent "$bad" "$good" >/dev/null
log=$(mktemp)
if ! git bisect run bash "$self" --step "$tests" >"$log" 2>&1; then
	tail -n 30 "$log"
	echo "oracle-bisect: no culprit found (bisect could not decide)"
	exit 3
fi
# A finished `git bisect run` leaves the first bad commit at refs/bisect/bad. Read it there, not from the run's
# text: under set -e and pipefail a grep that matched nothing ended the script with exit 1 and no output after a
# 36-minute bisect (oracle run 45685389788, git 2.56 on the runner).
sha=$(git rev-parse --verify -q 'refs/bisect/bad^{commit}' || true)
[ -n "$sha" ] || sha=$(grep -oE '^[0-9a-f]{40} is the first bad commit' "$log" | cut -d' ' -f1 || true)
[ -n "$sha" ] || { tail -n 30 "$log"; echo "oracle-bisect: no culprit found"; exit 3; }
subject=$(git log -1 --format=%s "$sha")
pr=$(sed -nE 's/^Merge pull request #([0-9]+).*/\1/p; s/.*\(#([0-9]+)\)$/\1/p' <<<"$subject" | head -n 1)
echo "CULPRIT $sha PR ${pr:-unknown} $subject"
if [ -n "${GITHUB_OUTPUT:-}" ]; then { echo "culprit=$sha"; echo "pr=${pr:-}"; } >>"$GITHUB_OUTPUT"; fi
