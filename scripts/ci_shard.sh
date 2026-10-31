#!/usr/bin/env bash
# ci_shard.sh <index> <count> -- print the Go packages CI shard <index> of
# <count> runs, one per line.
#
# The heaviest packages each get a shard of their own (index 0..H-1 of the
# HEAVY list below), measured on the post-merge suite: rules ~170 s, host
# ~155 s. Every other package goes to one of the remaining
# shards by a stable hash of its import path, so adding a package never
# reshuffles the others. Rebalance HEAVY from the CI run times when a light
# shard outgrows the heavy ones.
#
# Shard index is 0-based. `ci_shard.sh all <count>` prints every package with
# its shard, which is how the self-check in CI proves no package was dropped.
set -euo pipefail
idx=${1:?usage: ci_shard.sh <index|all> <count>}
count=${2:?usage: ci_shard.sh <index|all> <count>}

HEAVY=(
	github.com/adams-shaun/gorge/engine/rules
	github.com/adams-shaun/gorge/server/host
)
nheavy=${#HEAVY[@]}
if [ "$count" -le "$nheavy" ]; then
	echo "ci_shard.sh: need more than $nheavy shards, got $count" >&2
	exit 2
fi
light=$((count - nheavy))

shard_of() {
	local p=$1 i
	for i in "${!HEAVY[@]}"; do
		if [ "${HEAVY[$i]}" = "$p" ]; then
			echo "$i"
			return
		fi
	done
	# cksum is POSIX and deterministic across runners.
	local h
	h=$(printf '%s' "$p" | cksum | cut -d' ' -f1)
	echo $((nheavy + h % light))
}

# Experimental, benchmark, training-tool, fuzz and external-adapter packages,
# plus box-calibrated measurement ratchets (corpusheap's heap ceilings are
# tuned to the operator's box and fail on a GitHub runner). PRs and pushes skip
# them; the nightly run sets CI_FULL=1 and runs everything. `go vet ./...` in
# the lint job still compiles every one of them, so a build break is caught.
EXCLUDE='^github\.com/adams-shaun/gorge/(cmd/(cardfuzz|mtgsim|enginebench|keywordbench)|bots/spellbench(/.*)?|internal/corpusheap|testkit/bench)$'

pkgs=$(go list ./...)
if [ "${CI_FULL:-0}" != 1 ]; then
	pkgs=$(printf '%s\n' "$pkgs" | grep -Ev "$EXCLUDE")
fi

printf '%s\n' "$pkgs" | while read -r p; do
	s=$(shard_of "$p")
	if [ "$idx" = all ]; then
		echo "$s $p"
	elif [ "$s" = "$idx" ]; then
		echo "$p"
	fi
done
