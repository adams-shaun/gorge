#!/usr/bin/env bash
# pr-regen.sh FILE...: rebuild generated files over a merged tree (agentctl open-pr, when a ticket's change conflicts
# with the base only in them: the base's copy is checked out first). Exit 2 for a file it cannot rebuild.
set -uo pipefail
for f; do
  case $f in
  dirac/docs/support.md) ;;
  *) echo "pr-regen: $f is not a generated file this script rebuilds"; exit 2 ;;
  esac
done
source scripts/pr-cards.sh
cap=(systemd-run --user --scope -q -p MemoryMax=4G -p CPUQuota=400% env GOMAXPROCS=4 GOMEMLIMIT=3GiB)
"${cap[@]}" env DIRAC_UPDATE_SUPPORT=1 go test -timeout 5m -run '^TestSupportDocIsCurrent$' ./dirac/kgen >/dev/null 2>&1
"${cap[@]}" go test -timeout 5m -run '^TestSupportDocIsCurrent$' ./dirac/kgen || { echo "pr-regen: support.md did not regenerate"; exit 1; }
