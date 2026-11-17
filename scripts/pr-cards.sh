#!/usr/bin/env bash
# pr-cards.sh: source me. Links the main checkout's .cards into this worktree when it has none (agentctl open-pr's
# pr-<id> worktrees carry tracked files only), so corpus tests run rather than skip.
if [ ! -e .cards ]; then
  main=$(cd "$(git rev-parse --git-common-dir)/.." && pwd)
  [ -d "$main/.cards/cardsfolder" ] && ln -s "$main/.cards" .cards
fi
[ -d .cards/cardsfolder ] || { echo "pr-cards: no .cards corpus (the corpus tests would skip)"; exit 3; }
