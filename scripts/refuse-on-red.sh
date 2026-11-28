#!/usr/bin/env bash
# refuse-on-red.sh BASE: a rule-cited engine change that breaks lockstep refuses the cards
# it diverges on instead of being reverted (invariant 13). It plays the dirac gate's
# committed FDN and BLB slices, both surfaces, shortened; a bad game becomes the
# `kgen -refuse` commands scripts/refuse-list.sh prints. Report-only unless
# REFUSE_ON_RED=apply.
set -uo pipefail

base=${1:-}
[ -n "$base" ] || { echo "usage: refuse-on-red.sh BASE" >&2; exit 2; }

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

na() { echo "refuse-on-red: n/a ($1)"; exit 0; }
person() { echo "refuse-on-red: needs a person: $*"; exit 1; }

# 1. Only a rules change with a stated ground is a refusal case: a non-test Go file
# under engine/, no dirac/ file, and a commit message that names the rule or oracle.
changed=$(git diff --name-only "$base"...HEAD) || person "cannot diff $base...HEAD"
engine_go=$(grep -E '^engine/.*\.go$' <<<"$changed" | grep -v '_test\.go$' || true)
[ -n "$engine_go" ] || na "no non-test Go change under engine/"
grep -q '^dirac/' <<<"$changed" && na "the change touches dirac/"
msgs=$(git log --format=%B "$base"..HEAD) || person "cannot read the commit messages of $base..HEAD"
ground=$(grep -m1 -E '^(Rule: CR [0-9]|Oracle: (forge|xmage|ruling) )' <<<"$msgs" || true)
[ -n "$ground" ] || na "no Rule: CR N or Oracle: ... line in a commit message"

# 2. Nothing to play without the corpus and dirac's tables.
corpus_check=${REFUSE_ON_RED_CORPUS_CHECK:-'[ -d .cards/cardsfolder ] && [ -d .cards/kernel-tables ]'}
bash -c "$corpus_check" >/dev/null 2>&1 || na "no corpus"

# 3. The gate's step 4 slices, shortened. The cap is pr-precheck.sh's.
pairs=${REFUSE_ON_RED_PAIRS:-300}
cap=(systemd-run --user --scope -q -p MemoryMax=4G -p CPUQuota=400% env GOMAXPROCS=4 GOMEMLIMIT=3GiB)
# Where no scope can start (no systemd-run, or no user bus: "Failed to connect to bus"),
# keep the Go limits and say that the memory and CPU cap is gone.
if ! "${cap[@]}" true >/dev/null 2>&1; then
	cap=(env GOMAXPROCS=4 GOMEMLIMIT=3GiB)
	echo "refuse-on-red: cannot start a systemd scope; kfuzz runs without the memory and CPU cap" >&2
fi
kfuzz=(go run ./dirac/oracle/cmd/kfuzz)
[ -z "${REFUSE_ON_RED_KFUZZ:-}" ] || read -r -a kfuzz <<<"${REFUSE_ON_RED_KFUZZ}"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/refuse-on-red.XXXXXX") || person "cannot make a temporary directory"
trap 'rm -rf "$tmp"' EXIT

fsets=(); fsurfs=(); flogs=()
for set_code in fdn blb; do
	for surface in autopay full; do
		log=$tmp/$set_code-$surface.log
		rc=0
		"${cap[@]}" "${kfuzz[@]}" -kernel "$set_code" -first-pair 0 -pairs "$pairs" -seed 1 \
			-keep-going -surface "$surface" -mulligans 1 -shrink 0 -workers 4 \
			-absolute -implicates-all >"$log" 2>&1 || rc=$?
		if grep -q '^bad-game' "$log"; then
			fsets+=("$set_code"); fsurfs+=("$surface"); flogs+=("$log")
		elif [ "$rc" != 0 ]; then
			person "kfuzz $set_code $surface failed (rc $rc): $(tail -3 "$log" | tr '\n' ' ')"
		fi
	done
done
if [ ${#fsets[@]} -eq 0 ]; then
	echo "refuse-on-red: lockstep holds ($pairs pairs a slice)"
	exit 0
fi

# 4. The commands the bad games name, per failing slice. One card can diverge on both
# surfaces of a set: de-duplicate by the card -refuse names before counting it twice.
why="gorge follows $ground; dirac not yet"
commands=()
cards_seen=$'\n'
refuse_card() {
	local s=${1#* -refuse \"}
	printf '%s' "${s%%\" -why *}"
}
for i in "${!fsets[@]}"; do
	rc=0
	out=$(bash "$here/refuse-list.sh" --why "$why" "${fsets[$i]}" "${fsurfs[$i]}" "${flogs[$i]}" 2>"$tmp/refuse-list.err") || rc=$?
	if [ "$rc" != 0 ]; then
		reason=$(grep -m1 'no card named' "$tmp/refuse-list.err" || true)
		[ -n "$reason" ] || reason=$(tail -1 "$tmp/refuse-list.err")
		person "${reason:-refuse-list failed (rc $rc)}"
	fi
	while IFS= read -r line; do
		[ -n "$line" ] || continue
		card=$(refuse_card "$line")
		case $cards_seen in *$'\n'"$card"$'\n'*) continue ;; esac
		cards_seen+=$card$'\n'
		commands+=("$line")
	done <<<"$out"
done
cards=${#commands[@]}
max=${REFUSE_ON_RED_MAX:-12}
if [ "$cards" -gt "$max" ]; then
	person "$cards cards is more than REFUSE_ON_RED_MAX ($max); a fix that breaks that many cards is not a refusal case"
fi

# 5. Report the commands, or run them and commit the refusals on this branch.
case "${REFUSE_ON_RED:-report}" in
report)
	echo "refuse-on-red: would refuse $cards cards (report-only):"
	if [ "$cards" -gt 0 ]; then printf '%s\n' "${commands[@]}"; fi
	;;
apply)
	# One command per card, not per failing slice: kgen -refuse fails on an entry
	# that exists, so a card chosen on two surfaces is refused once. The report pass
	# above already de-duplicated and rejected unnamed games and the card cap.
	read -r -a runner <<<"${REFUSE_LIST_RUN:-bash -c}"
	if [ "$cards" -gt 0 ]; then
		for line in "${commands[@]}"; do
			"${runner[@]}" "$line" || person "refuse-list --apply failed: $line"
		done
	fi
	# -A stages an add, a modification and a deletion alike, and needs no porcelain
	# parsing (git C-quotes a path with a space, which ${line:3} would not unquote).
	git add -A -- dirac || person "cannot stage the refusals"
	git diff --cached --quiet -- dirac && person "apply produced no staged refusals"
	# The pathspec keeps anything else already staged out of the refusal commit.
	git commit -m "dirac: refuse $cards cards the rules fix diverges on" -m "$ground" -- dirac >/dev/null ||
		person "cannot commit the refusals"
	echo "refuse-on-red: refused $cards cards"
	;;
*)
	person "unknown REFUSE_ON_RED=${REFUSE_ON_RED} (want report or apply)"
	;;
esac
exit 0
