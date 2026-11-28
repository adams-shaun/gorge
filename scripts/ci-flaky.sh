#!/usr/bin/env bash
# ci-flaky.sh skip-regex | run <shard> <count>: the flaky-test quarantine listed in scripts/flaky.tsv. Run it from the
# repository root, as ci.yml and ci-test-shard.sh do.
#
# skip-regex prints the listed test names joined with "|" (nothing for an empty list). On a merge-queue batch
# ci-test-shard.sh adds it to its -skip expressions.
# run runs each listed test whose package is on shard <shard> of <count>, prints "QUARANTINE pass <package> <test>",
# or "QUARANTINE FAIL <package> <test>" and the last 30 lines of the test's output, and always exits 0.
#
# FLAKY_FILE names another list; FLAKY_SHARD_CMD is a command printing the shard's entries in place of ci_shard.sh.
set -euo pipefail
here=$(dirname "$0")
list=${FLAKY_FILE:-$here/flaky.tsv}
usage="usage: ci-flaky.sh skip-regex | run <shard> <count>"

case ${1:-} in
skip-regex)
	awk -F'\t' '!/^#/ && $2 != "" { s = s sep $2; sep = "|" } END { if (s != "") print s }' "$list"
	;;
run)
	trap 'exit 0' EXIT # nothing here may fail the batch
	shard=${2:?$usage} count=${3:?$usage}
	if ! entries=$(if [ -n "${FLAKY_SHARD_CMD:-}" ]; then bash -c "$FLAKY_SHARD_CMD"; else bash "$here/ci_shard.sh" "$shard" "$count"; fi); then
		echo "::warning::ci-flaky: could not list shard $shard of $count: no quarantined test ran"
		exit 0
	fi
	# The rows whose package is on this shard, a split entry's #<label> removed.
	rows=$(awk -F'\t' 'FNR == NR { sub(/#.*/, ""); on[$0] = 1; next } !/^#/ && $2 != "" && ($1 in on) { print $1 "\t" $2 }' \
		<(printf '%s\n' "$entries") "$list")
	while IFS=$'\t' read -r pkg test; do
		[ -n "$pkg" ] || continue
		if out=$(go test -count=1 -timeout 10m -run "^$test\$" "$pkg" 2>&1 </dev/null); then
			echo "QUARANTINE pass $pkg $test"
		else
			echo "QUARANTINE FAIL $pkg $test"
			tail -n 30 <<<"$out"
		fi
	done <<<"$rows"
	;;
*)
	echo "$usage" >&2
	exit 2
	;;
esac
