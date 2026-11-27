#!/usr/bin/env bash
# green-main.sh [max_pages]: print the head sha of the newest green oracle.yml run on main.
#
# "Green" is the test oracle.yml's bisect step uses: a run that concluded success whose oracle* jobs are all
# success or skipped, with at least one success. Most runs on main skip the oracle jobs, so the script pages
# through up to max_pages (default 5) pages of 100 completed runs, newest first, reading each successful run's
# jobs once. It prints the winner's head_sha on stdout and exits 0; with no winner in the window it prints
#   green-main: no green oracle run on main in <n> runs
# on stderr and exits 3. Any other non-zero exit is an API failure. Test hook: GH (default gh).
set -euo pipefail

gh=${GH:-gh}
max_pages=${1:-5}
repo=${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is not set}

# One line per oracle* job set: "<bad> <ran>".
jobs_jq='[.jobs[] | select(.name | startswith("oracle"))]
  | "\([.[] | select(.conclusion != "skipped" and .conclusion != "success")] | length) \([.[] | select(.conclusion == "success")] | length)"'

n=0
for ((page = 1; page <= max_pages; page++)); do
  runs=$("$gh" api "repos/$repo/actions/workflows/oracle.yml/runs?branch=main&status=completed&per_page=100&page=$page" \
    --jq '.workflow_runs[] | "\(.id) \(.head_sha) \(.conclusion)"')
  count=0
  while read -r id sha conclusion; do
    [ -n "$id" ] || continue
    count=$((count + 1))
    n=$((n + 1))
    [ "$conclusion" = success ] || continue
    counts=$("$gh" api "repos/$repo/actions/runs/$id/jobs?per_page=100" --jq "$jobs_jq")
    read -r bad ran <<<"$counts"
    if [ "$bad" = 0 ] && [ "$ran" -gt 0 ]; then
      printf '%s\n' "$sha"
      exit 0
    fi
  done <<<"$runs"
  # A short page is the last one.
  [ "$count" -eq 100 ] || break
done

echo "green-main: no green oracle run on main in $n runs" >&2
exit 3
