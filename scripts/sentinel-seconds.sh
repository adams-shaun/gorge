#!/usr/bin/env bash
# sentinel-seconds.sh [run-id...]: print dirac/oracle/testdata/sentinel-seconds.txt, the mean top-level "--- PASS"
# time of each oracle sentinel over the green oracle-sentinels jobs of the given ci.yml runs (default: the last 5
# queue batches with one). scripts/ci-sentinels.sh deals the sentinels on it.
# Refresh: scripts/sentinel-seconds.sh > dirac/oracle/testdata/sentinel-seconds.txt
set -euo pipefail
export LC_ALL=C
repo=$(gh repo view --json nameWithOwner --jq .nameWithOwner)
runs=("$@")
if [ ${#runs[@]} -eq 0 ]; then
	mapfile -t runs < <(gh run list --workflow ci.yml -L 100 --json databaseId,headBranch \
		--jq '.[] | select(.headBranch | startswith("mergify/merge-queue/")) | .databaseId')
fi
tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT
used=() jobs=0
for r in "${runs[@]}"; do
	[ ${#used[@]} -lt 5 ] || [ $# -gt 0 ] || break
	ids=$(gh run view "$r" --json jobs --jq '.jobs[] | select((.name | startswith("oracle-sentinels")) and .conclusion == "success") | .databaseId')
	[ -n "$ids" ] || continue
	used+=("$r")
	for j in $ids; do
		gh api "repos/$repo/actions/jobs/$j/logs" | tr -d '\r' | sed -nE 's/^[^ ]*Z --- PASS: (Test[^ /]*) \(([0-9.]+)s\)$/\1 \2/p' >>"$tmp/pass"
		jobs=$((jobs + 1))
	done
done
[ -s "$tmp/pass" ] || { echo "sentinel-seconds: no green oracle-sentinels job in runs ${runs[*]}" >&2; exit 1; }
echo "# <test> <seconds>: the mean top-level \"--- PASS\" time of each oracle sentinel in the oracle-sentinels jobs of"
echo "# ci.yml runs ${used[*]} ($(date -u +%F), $jobs green jobs)."
echo "# scripts/ci-sentinels.sh deals the sentinels on it, heaviest first, to the least-loaded process; a sentinel it"
echo "# does not list weighs DEFAULT_SECONDS (10). Refresh: scripts/sentinel-seconds.sh [run-id...] > this file."
awk '{ s[$1] += $2; n[$1]++ } END { for (t in s) printf "%s %.1f\n", t, s[t] / n[t] }' "$tmp/pass" | sort
