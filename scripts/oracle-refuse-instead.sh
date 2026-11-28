#!/usr/bin/env bash
# oracle-refuse-instead.sh <culprit> <failing-file> <branch>: oracle.yml's bisect blamed <culprit>; when it is a
# rule-cited gorge change (refuse-on-red.sh --culprit applies), refuse the cards the committed lockstep slices
# implicate on a new <branch> and keep that branch only if the refusals turn the failing oracle tests green
# (invariant 13: a rules fix with a stated ground is refused around, not reverted). Run from the repo root on a
# clean tree at the head the oracle failed on.
#
# Exit 0: <branch> holds the refusal commit (and a sentinel commit). Exit 10: not a refusal case. Exit 11: no
# refusal was made. Exit 12: the refusals do not cure the failing tests. On any non-zero exit the tree is back at
# the starting head and <branch> is gone, so the caller's revert path starts from the tree it always did.
#
# Hooks: ORACLE_REFUSE_BUILD (default: make dirac-tables) and ORACLE_REFUSE_VERIFY (default: go test over
# ./dirac/oracle; it gets the -run alternation in $RX), each run with `bash -c`, as oracle-bisect.sh does.
set -uo pipefail
export LC_ALL=C

[ $# -eq 3 ] || { echo "usage: oracle-refuse-instead.sh <culprit> <failing-file> <branch>" >&2; exit 2; }
culprit=$1
failing=$2
branch=$3
build=${ORACLE_REFUSE_BUILD:-make dirac-tables}
# Single-quoted: $RX is expanded by `bash -c` (set -u: it is not set here yet).
verify=${ORACLE_REFUSE_VERIFY:-'go test -timeout 15m -run "^($RX)\$" ./dirac/oracle'}
sent=dirac/oracle/testdata/sentinels.txt

[ -f "$failing" ] && [ -r "$failing" ] || { echo "oracle-refuse-instead: cannot read $failing" >&2; exit 2; }
start=$(git rev-parse --verify -q HEAD) || { echo "oracle-refuse-instead: no HEAD" >&2; exit 2; }
created=0
tmp=$(mktemp -d "${TMPDIR:-/tmp}/oracle-refuse-instead.XXXXXX") || exit 2

# restore puts the tree back at the starting head: any commit on <branch>, anything staged or modified, and any
# untracked refusal file under dirac/support are dropped.
restore() {
	git switch -q --discard-changes --detach "$start" || echo "oracle-refuse-instead: cannot return to $start" >&2
	if [ "$created" = 1 ]; then
		git branch -q -D "$branch" || echo "oracle-refuse-instead: cannot delete $branch" >&2
	fi
	git checkout -q -- . 2>/dev/null || true
	git clean -fdq -- dirac/support || true
}
finish() {
	local rc=$?
	if [ "$rc" != 0 ]; then restore; fi
	rm -rf "$tmp"
	exit "$rc"
}
trap finish EXIT

# 1. A branch for the refusal commit, dirac's tables at this head, and refuse-on-red in apply mode on the
# culprit's own diff and messages.
git switch -q -c "$branch" || { echo "oracle-refuse-instead: cannot create $branch" >&2; exit 2; }
created=1
bash -c "$build" >&2 || { echo "oracle-refuse-instead: tables did not build" >&2; exit 2; }
ror_rc=0
REFUSE_ON_RED=apply bash scripts/refuse-on-red.sh --culprit "$culprit" >"$tmp/ror.out" 2>"$tmp/ror.err" || ror_rc=$?
cat "$tmp/ror.err" >&2
na=$(grep -m1 '^refuse-on-red: n/a' "$tmp/ror.out" || true)
if [ -n "$na" ]; then
	reason=${na#refuse-on-red: n/a }
	reason=${reason#(}
	echo "oracle-refuse-instead: n/a (${reason%)})"
	exit 10
fi
last=$(grep -v '^[[:space:]]*$' "$tmp/ror.out" | tail -n 1)
[ -n "$last" ] || last=$(grep -v '^[[:space:]]*$' "$tmp/ror.err" | tail -n 1)

# 2. Needs a person, or nothing was committed: no refusal.
case $ror_rc in
0 | 1) ;;
*)
	echo "oracle-refuse-instead: refuse-on-red.sh failed (rc $ror_rc): $last" >&2
	exit 2
	;;
esac
if [ "$ror_rc" = 1 ] || [ "$(git rev-parse HEAD)" = "$start" ]; then
	echo "oracle-refuse-instead: no refusal (${last:-refuse-on-red printed nothing})"
	exit 11
fi
cards=$(sed -nE 's/^refuse-on-red: refused ([0-9]+) cards$/\1/p' "$tmp/ror.out" | tail -n 1)
[ -n "$cards" ] || cards=$(git diff --name-only "$start" HEAD -- dirac | wc -l | tr -d ' ')

# 3. The refusals cure the failure only if every failing test now passes on rebuilt tables.
bash -c "$build" >&2 || { echo "oracle-refuse-instead: tables did not build after the refusals" >&2; exit 2; }
mapfile -t names < <(grep -E '^Test[A-Za-z0-9_]*$' "$failing" | sort -u || true)
if [ ${#names[@]} -eq 0 ]; then
	echo "oracle-refuse-instead: refusals do not cure (no test names in $failing)"
	exit 12
fi
RX=$(printf '%s\n' "${names[@]}" | paste -sd'|')
export RX
vrc=0
bash -c "$verify" >"$tmp/verify.out" 2>&1 || vrc=$?
fails=$(sed -nE 's/^[[:space:]]*--- FAIL: ([A-Za-z0-9_]+).*/\1/p' "$tmp/verify.out" | sort -u | paste -sd' ')
if [ "$vrc" != 0 ] || [ -n "$fails" ]; then
	tail -n 40 "$tmp/verify.out" >&2
	[ -n "$fails" ] || fails="${names[*]} (rc $vrc)"
	echo "oracle-refuse-instead: refusals do not cure $fails"
	exit 12
fi

# 4. The cured tests join the sentinel set, as the revert branch's do.
bash scripts/sentinel-append.sh "$failing" "oracle run ${GITHUB_RUN_ID:-local} (auto-refusal for #${PR:-?})" >&2 ||
	{ echo "oracle-refuse-instead: sentinel-append.sh failed" >&2; exit 2; }
if ! git diff --quiet -- "$sent"; then
	git add -- "$sent" || exit 2
	git commit -q -m "oracle: sentinel the tests #${PR:-?} reddened" -- "$sent" ||
		{ echo "oracle-refuse-instead: cannot commit $sent" >&2; exit 2; }
fi

# 5. The report the refusal PR body carries.
echo "oracle-refuse-instead: refused $cards cards; ${#names[@]} failing tests pass"
# Apply mode runs the kgen -refuse commands rather than printing them; then the refused files stand in.
grep -E -- '-refuse "' "$tmp/ror.out" || git diff --name-only "$start" HEAD -- dirac | sed 's/^/refused: /'
exit 0
