#!/usr/bin/env bash
# ci-pkg-times.sh <run-id>: print "<package>\t<seconds>" for every package a CI run's test shards passed, the input
# ci_shard.sh balances on (scripts/ci-pkg-times.tsv).
set -euo pipefail
echo "# ci-pkg-times.tsv: measured seconds per package (go test's \"ok\" line, run $1). ci_shard.sh balances shards on it."
echo "# Refresh: scripts/ci-pkg-times.sh <run-id> > scripts/ci-pkg-times.tsv"
gh run view "$1" --log | grep -E $'^test \\([0-9]+\\)\t.*ok  \tgithub.com' \
  | sed -E 's/.*ok +\t?(github.com[^ \t]*)[ \t]+([0-9.]+)s.*/\1\t\2/' | awk -F'\t' '{printf "%s\t%.0f\n", $1, $2}' | sort
