#!/usr/bin/env bash
# pr-cards.sh: source me. Links the main checkout's .cards into this worktree when it has none (agentctl open-pr's
# pr-<id> worktrees carry tracked files only), so corpus tests run rather than skip.
if [ ! -e .cards ]; then
  main=$(cd "$(git rev-parse --git-common-dir)/.." && pwd)
  [ -d "$main/.cards/cardsfolder" ] && ln -s "$main/.cards" .cards
fi
[ -d .cards/cardsfolder ] || { echo "pr-cards: no .cards corpus (the corpus tests would skip)"; exit 3; }
# dirac's card tables are keyed on this tree's table sources (gorge.TableSourceHash), so a ticket that touches dirac
# needs its own build; without it every dirac test fails "no card tables for this binary" (11-17: two tickets'
# prechecks went red on that, not on their change). About 20 s; a key already built is reused.
systemd-run --user --scope -q -p MemoryMax=6G -p CPUQuota=800% make dirac-tables >/dev/null 2>&1 ||
  { echo "pr-cards: make dirac-tables failed"; exit 3; }
