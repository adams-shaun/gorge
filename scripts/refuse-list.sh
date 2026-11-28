#!/usr/bin/env bash
# refuse-list.sh: turn a lockstep failure log into the kgen -refuse commands that refuse
# the cards it names (invariant 13's second answer, one step).
#
#   scripts/refuse-list.sh [--apply] [--why TEXT] <set> <surface> <kfuzz-log>...
#
# Reads only the "bad-game<TAB>" lines `kfuzz -implicates-all` prints (fields: bad-game,
# pair, seed, outcome, class, implicated cards joined by |, recent sources joined by |,
# table). Each bad game chooses one card: the first implicated card, else the first
# recent source. Prints one command per distinct card, sorted by name, at the lowest pair
# that chose it, in the form scripts/dirac-gate.sh prints:
#
#   go run ./dirac/cmd/kgen -refuse "<card>" -why "<why>" -repro <set>:<surface>:<pair>
#
# <why> defaults to "diverges from gorge at <set> <surface> pair <pair>". The card and the
# why are escaped for a double-quoted bash word (\ " $ `), so the line runs as printed.
#
# --apply runs each command in order instead of printing it, through $REFUSE_LIST_RUN
# (a command; the line is its last argument; default `bash -c`), and stops at the first
# that fails with its exit code. Nothing is built and no corpus is read without --apply.
#
# Exit: 0 ok (also when no bad game is in the input); 2 usage; 3 a bad game names no
# card (nothing is run under --apply); else the failing command's code under --apply.
set -euo pipefail

usage() {
	echo "usage: scripts/refuse-list.sh [--apply] [--why TEXT] <set> <surface> <kfuzz-log>..." >&2
	exit 2
}

apply=0
why=
why_set=0
while [ $# -gt 0 ]; do
	case $1 in
	--apply) apply=1; shift ;;
	--why)
		[ $# -ge 2 ] || usage
		why=$2; why_set=1; shift 2 ;;
	--) shift; break ;;
	-*) usage ;;
	*) break ;;
	esac
done
[ $# -ge 3 ] || usage
set_code=$1 surface=$2
shift 2
for f in "$@"; do
	[ -f "$f" ] && [ -r "$f" ] || { echo "refuse-list: cannot read $f" >&2; usage; }
done

# One record a bad game: "N<TAB>card<TAB>pair" when it names a card, "U<TAB>pair" when not.
games=$(LC_ALL=C awk -F'\t' '
	/^bad-game\t/ {
		card = $6; sub(/\|.*/, "", card)
		if (card == "") { card = $7; sub(/\|.*/, "", card) }
		if (card == "") print "U\t" $2
		else print "N\t" card "\t" $2
	}' "$@")

if [ -z "$games" ]; then
	echo "refuse-list: no bad game in the input" >&2
	exit 0
fi

unnamed=$(LC_ALL=C awk -F'\t' '$1 == "U" { print $2 }' <<<"$games" | LC_ALL=C sort -n -u)
# card<TAB>lowest pair, sorted by card name.
chosen=$(LC_ALL=C awk -F'\t' '$1 == "N" { print $2 "\t" $3 }' <<<"$games" |
	LC_ALL=C sort -t$'\t' -k1,1 -k2,2n |
	LC_ALL=C awk -F'\t' '$1 != prev { print; prev = $1 }')

# esc escapes $1 for the inside of a double-quoted bash word.
esc() {
	local s=$1
	s=${s//\\/\\\\}
	s=${s//\"/\\\"}
	s=${s//\$/\\\$}
	s=${s//\`/\\\`}
	printf '%s' "$s"
}

lines=()
if [ -n "$chosen" ]; then
	while IFS=$'\t' read -r card pair; do
		w=$why
		[ "$why_set" = 1 ] || w="diverges from gorge at $set_code $surface pair $pair"
		lines+=("go run ./dirac/cmd/kgen -refuse \"$(esc "$card")\" -why \"$(esc "$w")\" -repro $set_code:$surface:$pair")
	done <<<"$chosen"
fi

status=0
if [ -n "$unnamed" ]; then
	while read -r pair; do
		echo "refuse-list: pair $pair: no card named" >&2
	done <<<"$unnamed"
	status=3
fi

if [ "$apply" = 1 ] && [ "$status" = 0 ]; then
	read -r -a runner <<<"${REFUSE_LIST_RUN:-bash -c}"
	for l in "${lines[@]}"; do
		rc=0
		"${runner[@]}" "$l" || rc=$?
		if [ "$rc" != 0 ]; then
			echo "refuse-list: failed ($rc): $l" >&2
			exit "$rc"
		fi
	done
elif [ "$apply" = 0 ]; then
	for l in "${lines[@]}"; do
		printf '%s\n' "$l"
	done
fi

ngames=$(awk 'END { print NR }' <<<"$games")
ncards=${#lines[@]}
echo "refuse-list: $ngames bad games, $ncards cards" >&2
exit "$status"
