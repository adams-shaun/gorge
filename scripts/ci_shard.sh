#!/usr/bin/env bash
# ci_shard.sh <index> <count> -- print the Go packages CI shard <index> of
# <count> runs, one per line.
#
# Shards are balanced on measured time: scripts/ci-pkg-times.tsv holds each package's seconds from a CI run
# (scripts/ci-pkg-times.sh <run-id> refreshes it). Packages go heaviest first to the least-loaded shard; on equal
# load, to the shard with fewer entries (then lower index, then import path), so a 0 s package's process launch is
# counted and the split is deterministic on every runner. A package with no measurement weighs DEFAULT_WEIGHT
# seconds. 11-16: the old split (two heavy packages pinned, the rest by hash) gave shards of 72 s to 269 s of package
# time; the slowest shard sets the PR's wall time.
#
# Shard index is 0-based. `ci_shard.sh all <count>` prints every package with
# its shard, which is how the self-check in CI proves no package was dropped.
set -euo pipefail
idx=${1:?usage: ci_shard.sh <index|all> <count>}
count=${2:?usage: ci_shard.sh <index|all> <count>}
times=$(dirname "$0")/ci-pkg-times.tsv

# Experimental, benchmark, training-tool, fuzz and external-adapter packages,
# plus box-calibrated measurement ratchets (corpusheap's heap ceilings are
# tuned to the maintainers' machine and fail on a GitHub runner). PRs and pushes skip
# them; the nightly run sets CI_FULL=1 and runs everything. `go vet ./...` in
# the lint job still compiles every one of them, so a build break is caught.
EXCLUDE='^github\.com/adams-shaun/gorge/(cmd/(cardfuzz|mtgsim|enginebench|keywordbench)|bots/spellbench(/.*)?|internal/corpusheap|testkit/bench)$'

# dirac's oracle is a full lockstep suite, run on its own before a merge to main.
pkgs=$(go list ./... | grep -v '/dirac/oracle$')
if [ "${CI_FULL:-0}" != 1 ]; then
	pkgs=$(printf '%s\n' "$pkgs" | grep -Ev "$EXCLUDE")
fi
# A package in ci-splits.tsv is one entry per row: the package, run minus every row's regex, and <package>#<label>
# (label defaults to A), the row's regex (ci.yml's test step). engine/rules has three rows (four groups; 11-27).
# Whole, engine/rules (242-269 s) and testkit/conformance (239 s) were the slowest shards' floors.
while IFS=$'\t' read -r sp _ label; do
	case $sp in '' | '#'*) continue ;; esac
	label=${label:-A}
	if printf '%s\n' "$pkgs" | grep -qx "$sp"; then pkgs=$(printf '%s\n' "$pkgs" "$sp#$label"); fi
done <"$(dirname "$0")/ci-splits.tsv"

printf '%s\n' "$pkgs" | awk -F'\t' -v idx="$idx" -v count="$count" -v def="${DEFAULT_WEIGHT:-3}" '
	FNR == NR { if ($0 !~ /^#/ && NF >= 2) w[$1] = $2; next }
	{ n++; p[n] = $0; wt[n] = ($0 in w) ? w[$0] + 0 : def }
	END {
		for (i = 1; i <= n; i++) o[i] = i
		for (i = 2; i <= n; i++) { k = o[i]; j = i - 1
			while (j >= 1 && (wt[o[j]] < wt[k] || (wt[o[j]] == wt[k] && p[o[j]] > p[k]))) { o[j + 1] = o[j]; j-- }
			o[j + 1] = k }
		for (s = 0; s < count; s++) { load[s] = 0; cnt[s] = 0 }
		for (i = 1; i <= n; i++) { k = o[i]; b = 0
			# Among equal loads prefer the shard with fewer entries: a 0 s package still costs a
			# process launch, and on ties the first minimum would otherwise collect every 0 s
			# package (11-27: shard 2 held 64 entries against 21 on the rest).
			for (s = 1; s < count; s++)
				if (load[s] < load[b] || (load[s] == load[b] && cnt[s] < cnt[b])) b = s
			load[b] += wt[k]; cnt[b]++; sh[k] = b }
		for (i = 1; i <= n; i++) {
			if (idx == "all") print sh[i], p[i]
			else if (sh[i] == idx) print p[i]
		}
	}' "$times" -
