#!/usr/bin/env bash
# unrefuse-stale.sh <dir> [short-sha]: turn what TestRefusedEntriesStillDiverge recorded under
# DIRAC_UNREFUSE_STALE=<dir> (scripts/regen-main.sh) into the unrefusals a stale refusal asks
# for. Each stale.tsv line (card, slug, the pairs played, the pool searched) moves the card's
# recorded clean game into dirac/oracle/testdata/repro as its proof, runs kgen -unrefuse on the
# card, and appends the proof row to dirac/support/unrefused.tsv — the row TestUnrefusalNeedsProof
# demands for a card that had a refused entry and no longer has one. Nothing went stale (no
# stale.tsv): do nothing. GORGE_ROOT overrides the repo root (the test runs it on a fake tree),
# KGEN the kgen command (the test's stub).
set -euo pipefail
[ $# -ge 1 ] || { echo "usage: unrefuse-stale.sh <dir> [short-sha]" >&2; exit 2; }
dir=$1
[ -f "$dir/stale.tsv" ] || { echo "unrefuse-stale: no stale.tsv under $dir: nothing went stale"; exit 0; }
root=${GORGE_ROOT:-$(git rev-parse --show-toplevel)}
kgen=${KGEN:-go run ./dirac/cmd/kgen}
short=${2:-$(git -C "$root" rev-parse --short HEAD 2>/dev/null || echo unknown)}
while IFS=$'\t' read -r card slug played pool; do
	case $card in '' | \#*) continue ;; esac
	src="$dir/$slug.repro.json"
	dst="dirac/oracle/testdata/repro/${slug}_auto_unrefused.repro.json"
	mv "$src" "$root/$dst"
	(cd "$root" && $kgen -unrefuse "$card")
	printf '%s\t%s\tauto-unrefused by regen-main: no divergence in %s pairs of %s at %s\n' \
		"$card" "$dst" "$played" "$pool" "$short" >> "$root/dirac/support/unrefused.tsv"
done < "$dir/stale.tsv"
