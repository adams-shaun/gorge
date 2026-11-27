#!/usr/bin/env bash
# ci-shard-check.sh [<count>] -- assert scripts/ci_shard.sh places every test
# package on exactly one shard, drops none and invents none. <count> defaults to
# $SHARDS (ci.yml's env) or 6.
#
# ci_shard.sh's own comment promises that `all` "is how the self-check in CI
# proves no package was dropped"; this is that self-check, run by a step in
# ci.yml's prepare job. A test package that falls out of every shard never runs,
# and a batch where a test never ran is indistinguishable from one where it
# passed; a package placed twice runs twice and pays its cost twice. So:
#
#   1. every package ci_shard.sh splits (its own `pkgs` list, the same source
#      the splitter uses) has exactly one whole entry on exactly one shard;
#   2. every entry (whole or <package>#<label>) is placed exactly once, so no
#      package lands in two shards and two calls never duplicate a test;
#   3. every <package>#<label> entry is declared in ci-splits.tsv, and every
#      declared split row appears exactly once (no split call silently lost);
#   4. no entry names a package ci_shard.sh did not list, and every shard index
#      is in 0..<count>-1.
#
# Exit 0 when all hold; exit 1 with one line per violation.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
cd "$(dirname "$here")"
count=${1:-${SHARDS:-6}}
case $count in '' | *[!0-9]*) echo "ci-shard-check: bad shard count '$count'" >&2; exit 2 ;; esac

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

bash scripts/ci_shard.sh pkgs "$count" | sort -u >"$tmp/want"
bash scripts/ci_shard.sh all "$count" >"$tmp/all"
if [ ! -s "$tmp/want" ]; then echo "ci-shard-check: ci_shard.sh pkgs printed no packages" >&2; exit 1; fi

# The split entries ci-splits.tsv declares: one <package>#<label> per row,
# label defaulting to A exactly as ci_shard.sh defaults it.
awk -F'\t' '!/^#/ && $1 != "" { print $1 "#" ($3 == "" ? "A" : $3) }' scripts/ci-splits.tsv \
	| sort -u >"$tmp/declared"

errs=$(awk -v count="$count" -v wantfile="$tmp/want" -v declfile="$tmp/declared" '
	FILENAME == wantfile { want[$0] = 1; next }
	FILENAME == declfile { decl[$0] = 1; next }
	{
		if ($1 !~ /^[0-9]+$/ || $1 + 0 >= count) { print "shard index out of range: " $0; next }
		seen[$2]++
		base = $2; sub(/#.*/, "", base)
		if ($2 == base) whole[base]++
		else {
			label = $2; sub(/^[^#]*#/, "", label)
			split_seen[base "#" label] = 1
		}
		base_seen[base] = 1
	}
	END {
		for (e in seen) if (seen[e] != 1) print "entry placed " seen[e] " times: " e
		for (p in want) {
			if (!(p in whole)) print "package dropped from every shard (no whole entry): " p
			else if (whole[p] != 1) print "package has " whole[p] " whole entries: " p
		}
		for (b in base_seen) if (!(b in want)) print "entry for a package ci_shard.sh pkgs did not list: " b
		for (e in split_seen) if (!(e in decl)) print "split entry not declared in ci-splits.tsv: " e
		for (e in decl) {
			if (!(e in split_seen)) print "declared split entry missing from every shard: " e
			p = e; sub(/#.*/, "", p)
			if (!(p in want)) print "split row for a package ci_shard.sh pkgs did not list: " e
		}
	}
' "$tmp/want" "$tmp/declared" "$tmp/all")

if [ -n "$errs" ]; then
	printf '%s\n' "$errs"
	echo "ci-shard-check: FAIL: the test shard split does not cover every package exactly once" >&2
	exit 1
fi

echo "ci-shard-check: PASS: $(wc -l <"$tmp/want" | tr -d ' ') packages, $(wc -l <"$tmp/all" | tr -d ' ') entries on $count shards"
