#!/usr/bin/env bash
# pr-oracle.sh <ref>: the whole of ./dirac/oracle at <ref>, before `make pr` opens a pull request to
# main, in a throwaway worktree so the checkout is never switched. CI plays the same tests in parts
# on the pull request. The caller caps the process.
#
# DRY=1 prints the step and SKIP_ORACLE=1 skips it.
set -euo pipefail
ref=${1:?usage: pr-oracle.sh <ref>}
if [ -n "${SKIP_ORACLE:-}" ]; then
	echo "pr-oracle: skipped (SKIP_ORACLE)"
	exit 0
fi
step=(bash scripts/ci-oracle-shard.sh 0 1 -timeout 60m)
if [ -n "${DRY:-}" ]; then
	echo "pr-oracle: on $ref, in a throwaway worktree: make dirac-tables && ${step[*]}"
	exit 0
fi

top=$(git rev-parse --show-toplevel)
[ -d "$top/.cards/cardsfolder" ] || { echo "pr-oracle: no .cards corpus: make fetch-cards compile-cards" >&2; exit 1; }
mkdir -p "$top/.worktrees"
wt=$(mktemp -d "$top/.worktrees/pr-oracle.XXXXXX")
trap 'git -C "$top" worktree remove --force "$wt" 2>/dev/null || rm -rf -- "$wt"' EXIT
git -C "$top" worktree add -q --detach "$wt" "$ref"
ln -s "$(readlink -f "$top/.cards")" "$wt/.cards"
pools=${GORGE_POOLS:-$top/.pools}
[ -d "$pools" ] && export GORGE_POOLS=$(readlink -f "$pools")

echo "pr-oracle: the whole of dirac/oracle on $ref ($(git -C "$top" rev-parse --short "$ref"))"
cd "$wt"
make dirac-tables
"${step[@]}"
