#!/usr/bin/env bash
# unrefuse-stale.sh <dir> [short-sha]: turn what TestRefusedEntriesStillDiverge recorded under
# DIRAC_UNREFUSE_STALE=<dir> (scripts/regen-main.sh) into the unrefusals a stale refusal asks
# for. Each stale.tsv line (card, slug, the pairs played, the pool searched) moves the card's
# recorded clean game into dirac/oracle/testdata/repro as its proof, runs kgen -unrefuse on the
# card, and appends the proof row to dirac/support/unrefused.tsv — the row TestUnrefusalNeedsProof
# demands for a card that had a refused entry and no longer has one. Nothing went stale (no
# stale.tsv): do nothing. GORGE_ROOT overrides the repo root (the test runs it on a fake tree),
# KGEN the kgen command (the test's stub).
#
# A refusal entry a lockstep repro test requires is never unrefused: do-not.md says a stale
# entry whose pinned game moved is re-recorded, not unrefused, and deleting one can only turn
# a regen red (PR #5462: nine MBC cards whose Test<Something>ReprosLockstep asserts their entry
# exists). required_lockstep_test names the test that pins a card, so the loop can keep it.
set -euo pipefail
[ $# -ge 1 ] || { echo "usage: unrefuse-stale.sh <dir> [short-sha]" >&2; exit 2; }
dir=$1
[ -f "$dir/stale.tsv" ] || { echo "unrefuse-stale: no stale.tsv under $dir: nothing went stale"; exit 0; }
root=${GORGE_ROOT:-$(git rev-parse --show-toplevel)}
kgen=${KGEN:-go run ./dirac/cmd/kgen}
short=${2:-$(git -C "$root" rev-parse --short HEAD 2>/dev/null || echo unknown)}

# required_lockstep_test <card>: print the name of a dirac/oracle test
# Test<Something>ReprosLockstep whose body contains the card's quoted name, or nothing.
# These tests read dirac/support/refused and fail when the entry is gone, so an entry any
# of them names must stay. The function name is the suffix, not one file: the pin tests
# live in many *_test.go files, so scan them all.
required_lockstep_test() {
	local f files=()
	for f in "$root"/dirac/oracle/*_test.go; do
		if [ -f "$f" ]; then files+=("$f"); fi
	done
	[ ${#files[@]} -gt 0 ] || return 0
	awk -v q="\"$1\"" '
		FNR == 1 { infn = 0 }
		/^func Test[A-Za-z0-9_]*ReprosLockstep\(/ { infn = 1; fn = $2; sub(/\(.*/, "", fn); next }
		infn && /^}/ { infn = 0 }
		infn && index($0, q) > 0 { print fn; exit }
	' "${files[@]}"
}

while IFS=$'\t' read -r card slug played pool; do
	case $card in '' | \#*) continue ;; esac
	pinned=$(required_lockstep_test "$card") ||
		{ echo "unrefuse-stale: cannot scan dirac/oracle for a lockstep test pinning $card" >&2; exit 1; }
	if [ -n "$pinned" ]; then
		printf 'unrefused=no card=%s test=%s\n' "$card" "$pinned"
		continue
	fi
	src="$dir/$slug.repro.json"
	dst="dirac/oracle/testdata/repro/${slug}_auto_unrefused.repro.json"
	mv "$src" "$root/$dst"
	(cd "$root" && $kgen -unrefuse "$card")
	printf '%s\t%s\tauto-unrefused by regen-main: no divergence in %s pairs of %s at %s\n' \
		"$card" "$dst" "$played" "$pool" "$short" >> "$root/dirac/support/unrefused.tsv"
done < "$dir/stale.tsv"
