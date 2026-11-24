#!/usr/bin/env bash
# pr-cards-link.sh: run me from a worktree root. Gives the worktree a .cards at ITS Makefile's FORGE_REF when it has
# none. The main checkout's corpus is reused only when its cards.lock names this tree's pin; otherwise (a branch that
# bumps FORGE_REF) the per-ref store scripts/link-cards.sh builds is used. Linking the main checkout's corpus blindly
# made testutil.SharedCorpusDir panic on the pin mismatch in the public-boundary step (2026-11-24).
[ -e .cards ] && exit 0
ref=$(awk '/^FORGE_REF/ {print $3}' Makefile)
main=$(cd "$(git rev-parse --git-common-dir)/.." && pwd)
if [ -d "$main/.cards/cardsfolder" ] && [ -n "$ref" ] && grep -q -- "$ref" "$main/.cards/cards.lock" 2>/dev/null; then
  ln -s "$main/.cards" .cards
else
  bash scripts/link-cards.sh >&2 || exit 1
fi
