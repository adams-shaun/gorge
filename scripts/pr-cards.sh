#!/usr/bin/env bash
# pr-cards.sh: source me. Links this tree's corpus into this worktree when it has none (agentctl open-pr's
# pr-<id> worktrees carry tracked files only), so corpus tests run rather than skip. A tree that moves FORGE_REF
# (a card-corpus PR) gets the per-ref corpus from scripts/link-cards.sh, not the main checkout's stale one:
# testutil.SharedCorpusDir panics on a pin mismatch, and the main checkout's .cards belongs to its own pin
# (2026-11-24: every FORGE_REF bump failed pr-precheck on that panic).
if [ ! -e .cards ]; then
  pr_top=$(git rev-parse --show-toplevel)
  main=$(cd "$(git rev-parse --git-common-dir)/.." && pwd)
  pr_ref=$(awk '/^FORGE_REF/ {print $3}' "$pr_top/Makefile")
  if [ -d "$main/.cards/cardsfolder" ] && grep -q "\"ref\": *\"$pr_ref\"" "$main/.cards/cards.lock" 2>/dev/null; then
    ln -s "$main/.cards" .cards
  else
    bash "$pr_top/scripts/link-cards.sh" || exit 3
  fi
fi
[ -d .cards/cardsfolder ] || { echo "pr-cards: no .cards corpus (the corpus tests would skip)"; exit 3; }
# dirac's card tables are keyed on this tree's table sources (gorge.TableSourceHash), so a ticket that touches dirac
# needs its own build; without it every dirac test fails "no card tables for this binary" (11-17: two tickets'
# prechecks went red on that, not on their change). About 20 s; a key already built is reused.
systemd-run --user --scope -q -p MemoryMax=6G -p CPUQuota=800% make dirac-tables >/dev/null 2>&1 ||
  { echo "pr-cards: make dirac-tables failed"; exit 3; }
