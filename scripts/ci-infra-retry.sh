#!/usr/bin/env bash
# ci-infra-retry.sh --log LOGFILE [--name LABEL] -- CMD [ARGS...]: run a CI step's command, and run it ONCE more when it
# died of an infrastructure fault (ci.yml's test shards and dirac-slices).
#
# The command's stdout and stderr are merged, shown live and written to LOGFILE. On a non-zero exit the log is
# classified: a test verdict in it (a failed test, a bad game, a build or compile error) never retries; otherwise the
# first row of ci-infra-signatures.txt that matches a line of the log names the fault, LOGFILE moves to LOGFILE.1 and
# the command runs again, its exit code being this script's. No match: the command's own exit code. LABEL defaults to
# the command's first word. CI_INFRA_RETRY=0 turns the retry off. Usage error: exit 2.
#
# LC_ALL=C is set on the classifying commands only, so the wrapped command keeps the caller's environment.
set -uo pipefail

usage() {
	echo "usage: ci-infra-retry.sh --log LOGFILE [--name LABEL] -- CMD [ARGS...]" >&2
	exit 2
}

log= name= sep=
while [ $# -gt 0 ]; do
	case $1 in
	--log) [ $# -ge 2 ] || usage; log=$2; shift 2 ;;
	--name) [ $# -ge 2 ] || usage; name=$2; shift 2 ;;
	--) sep=1; shift; break ;;
	*) usage ;;
	esac
done
if [ -z "$log" ] || [ -z "$sep" ] || [ $# -eq 0 ]; then usage; fi
name=${name:-$1}
signatures=$(dirname "$0")/ci-infra-signatures.txt

# attempt runs the command once into $log and returns the command's exit code, not tee's.
attempt() {
	"$@" 2>&1 | tee "$log"
	return "${PIPESTATUS[0]}"
}

# verdict: the log says something about the code, so a second run is not allowed.
verdict() {
	LC_ALL=C grep -E -q -e '^--- FAIL: ' -e $'^bad-game\t' -e '\[build failed\]' -e '^[^ ]+\.go:[0-9]+:[0-9]+: ' "$log"
}

# signature prints the name of the first infrastructure fault the log matches; it fails when none does.
signature() {
	local sig re
	while IFS=$'\t' read -r sig re; do
		if LC_ALL=C grep -E -q -e "$re" "$log"; then
			printf '%s\n' "$sig"
			return 0
		fi
	done < <(LC_ALL=C awk -F'\t' '!/^#/ && $1 != "" && $2 != "" { print $1 "\t" $2 }' "$signatures")
	return 1
}

attempt "$@"
rc=$?
if [ "$rc" -eq 0 ] || [ "${CI_INFRA_RETRY:-1}" = 0 ] || verdict || ! hit=$(signature); then exit "$rc"; fi

echo "::warning::ci-infra-retry: $name hit $hit (exit $rc); retrying once"
mv -f "$log" "$log.1"
attempt "$@"
rc=$?
echo "ci-infra-retry: $name attempt 2 exit $rc"
exit "$rc"
