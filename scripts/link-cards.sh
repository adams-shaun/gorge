#!/usr/bin/env bash
# link-cards.sh: point this tree's .cards at the corpus of this tree's FORGE_REF, fetching and
# compiling it into a shared store when absent. Trees at the same ref share one corpus; a tree that
# bumps FORGE_REF gets its own. The store is $GORGE_CARD_STORE, default ~/.cache/gorge/cards.
set -euo pipefail
top=$(git rev-parse --show-toplevel)
cd "$top"
ref=$(awk '/^FORGE_REF/ {print $3}' Makefile)
[ -n "$ref" ] || { echo "link-cards: no FORGE_REF in Makefile" >&2; exit 1; }
store=${GORGE_CARD_STORE:-$HOME/.cache/gorge/cards}
corpus=$store/$ref
if [ -e .cards ] && [ ! -L .cards ]; then
	echo "link-cards: .cards is a real directory: move it to $corpus or remove it first" >&2
	exit 1
fi
mkdir -p "$store"
# -o: the lock must not stay held by anything the fetch leaves running.
(
	flock -o 9
	if [ ! -s "$corpus/cards.lock" ]; then
		make fetch-cards compile-cards CARDS_DIR="$corpus"
	fi
) 9>"$store/$ref.lock"
ln -sfn "$corpus" .cards
ls .cards/cardsfolder >/dev/null
echo "link-cards: .cards -> $corpus"
