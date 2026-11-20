#!/usr/bin/env bash
# dirac-evidence.sh <out>: play the evidence corpus in lockstep and write one verdict,
# coverage and reach file per run under <out>. Exit 0 when every run passes
# kfuzz -absolute, then scripts/dirac-first-refusals.py evidence <out> writes the lists.
#
#   the corpus  fdn, blb and pauper real decks, one synthetic pool per Standard list and per draft
#               pool (dirac/cardset/draftpools.txt, dr-<code>), and one
#               focus pool per list in dirac/support/focus (fx-<set>: std-<set> dealt around
#               the cards that list names, kfuzz -focus); 1500 pairs each, seed 1, the random policy
#   the forms   surfaces autopay and full, mulligans 0 and 1
#   the pods    a run named pod-<pool> (POOLS="pod-cmd5 pod-edh") plays <pool>'s decks as
#               four-seat pods, PODS pods (one game each, 0: 1500) cut to CMD_MINI cards: its
#               -evidence rows carry the seat count, which scripts/dirac-first-refusals.py
#               evidence --pools pod-<pool> copies into dirac/support/evidence, and its
#               evidence-games row carries the clean pod games; a pod run writes no coverage,
#               so it names no card's plays
#
# POOLS, SURFACES and MULLIGANS narrow a run, and only the named runs' files change.
# PLAY_REFUSED=1 plays the refused cards too; GATE_SLICE=1 plays a deck pool's supported cards
# only, as the gate does. WORKERS is games in parallel.
# The caller caps the process, for example:
#   systemd-run --user --scope -q -p MemoryMax=16G -p CPUQuota=800% env GOWORK=off GOMAXPROCS=8 GOMEMLIMIT=12GiB WORKERS=8 bash scripts/dirac-evidence.sh /tmp/evidence
set -uo pipefail
out=${1:?usage: dirac-evidence.sh <out>}
top=$(git rev-parse --show-toplevel)
cd "$top" || exit 2
mkdir -p "$out"
out=$(readlink -f "$out")
export GOWORK=off

make dirac-tables >"$out/tables.log" 2>&1 || { tail -5 "$out/tables.log"; exit 2; }
go build -trimpath -o "$out/kfuzz" ./dirac/oracle/cmd/kfuzz || exit 2

if [ -n "${POOLS:-}" ]; then
	pools=$POOLS
else
	pools="fdn blb pauper $("$out/kfuzz" -list-synth | grep -E '^(std|dr)-' | tr '\n' ' ')"
	for f in dirac/support/focus/fx-*.txt; do [ -e "$f" ] && pools="$pools $(basename "$f" .txt)"; done
fi
surfaces=${SURFACES:-autopay full}
mulligans=${MULLIGANS:-0 1}
workers=${WORKERS:-4}

fail=0
for pool in $pools; do
	case $pool in
	std-* | dr-* | tail-*) what=(-synth "$pool") ;;
	fx-*) what=(-synth "std-${pool#fx-}" -focus "$top/dirac/support/focus/$pool.txt") ;;
	pod-*) what=(-kernel "${pool#pod-}" -seats 4 -pods "${PODS:-1500}" -cmd-mini "${CMD_MINI:-60}") ;;
	*) what=(-kernel "$pool") ;;
	esac
	for surface in $surfaces; do
		for m in $mulligans; do
			run=$out/$pool.$surface.m$m
			extra=()
			case $pool in
			std-* | dr-* | tail-* | fx-*) extra=(-reach "$run.reach") ;;
			*) [ -n "${GATE_SLICE:-}" ] || extra=(-evidence "$run.evidence") ;;
			esac
			[ -n "${PLAY_REFUSED:-}" ] && extra+=(-play-refused)
			# a pod run writes no coverage or branches: kfuzz refuses them with -seats above 2
			case $pool in
			pod-*) measured=() ;;
			*) measured=(-coverage "$run.coverage" -branches "$run.branches") ;;
			esac
			"$out/kfuzz" "${what[@]}" -first-pair 0 -pairs 1500 -seed 1 -keep-going \
				-surface "$surface" -mulligans "$m" -shrink 0 -workers "$workers" -absolute \
				-implicates-all "${measured[@]}" -out "$run.repros" "${extra[@]}" >"$run.out" 2>&1
			rc=$?
			echo "$rc" >"$run.rc"
			if [ "$rc" != 0 ]; then
				fail=1
				echo "FAIL $pool $surface m$m: $(grep -m1 "^$pool: .* clean" "$run.out" || tail -1 "$run.out")"
			fi
		done
	done
done
[ "$fail" = 0 ] && echo "evidence corpus: every run passed"
exit "$fail"
