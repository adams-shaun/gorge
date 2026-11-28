#!/usr/bin/env bash
# sentinel-append.sh <failing-file> <note>: append the failing oracle tests to dirac/oracle/testdata/sentinels.txt,
# under "# Named by <note>". Run from the repo root (oracle.yml's auto-revert step does, st-qhealth-05).
#
# A name is a line of <failing-file> matching ^Test[A-Za-z0-9_]*$. Names already listed are dropped, and so are
# names no dirac/oracle/*_test.go declares (a test the reverted culprit added is gone, and listing it would fail
# TestSentinelsExist). At most SENTINEL_APPEND_MAX (default 3) are kept, lightest first by
# dirac/oracle/testdata/ci-shard-seconds.txt (an unlisted test weighs 0.5), ties by name. No clock, no network.
set -euo pipefail
export LC_ALL=C

usage() {
	echo "sentinel-append: $*" >&2
	echo "usage: sentinel-append.sh <failing-file> <note>" >&2
	exit 2
}

[ $# -eq 2 ] || usage "want 2 arguments, got $#"
failing=$1
note=$2
sent=dirac/oracle/testdata/sentinels.txt
secs=dirac/oracle/testdata/ci-shard-seconds.txt
max=${SENTINEL_APPEND_MAX:-3}
[ -f "$failing" ] && [ -r "$failing" ] || usage "cannot read $failing"
[ -f "$sent" ] || usage "no $sent"
[[ $max =~ ^[0-9]+$ ]] || usage "SENTINEL_APPEND_MAX=$max is not a number"

mapfile -t names < <(grep -E '^Test[A-Za-z0-9_]*$' "$failing" | sort -u || true)

declare -A listed=() declared=()
while IFS= read -r line; do
	line=${line%%#*}
	line=$(printf '%s' "$line" | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//')
	if [ -n "$line" ]; then
		listed[$line]=1
	fi
done < "$sent"
shopt -s nullglob
files=(dirac/oracle/*_test.go)
shopt -u nullglob
if [ ${#files[@]} -gt 0 ]; then
	while IFS= read -r n; do
		declared[$n]=1
	done < <(sed -nE 's/^func (Test[A-Za-z0-9_]*)\(.*/\1/p' "${files[@]}")
fi

nlisted=0
ngone=0
cand=()
for n in "${names[@]}"; do
	if [ -n "${listed[$n]:-}" ]; then
		nlisted=$((nlisted + 1))
	elif [ -z "${declared[$n]:-}" ]; then
		ngone=$((ngone + 1))
	else
		cand+=("$n")
	fi
done

# Lightest first, ties by name.
ordered=()
if [ ${#cand[@]} -gt 0 ]; then
	weights=/dev/null
	[ -f "$secs" ] && weights=$secs
	mapfile -t ordered < <(printf '%s\n' "${cand[@]}" | awk '
		FILENAME == ARGV[1] { if ($0 !~ /^[[:space:]]*#/ && NF >= 2) w[$1] = $2; next }
		{ print (($1 in w) ? w[$1] : 0.5), $1 }' "$weights" - | sort -k1,1n -k2,2 | awk '{ print $2 }')
fi
kept=("${ordered[@]:0:max}")
cut=("${ordered[@]:max}")

if [ ${#kept[@]} -eq 0 ]; then
	echo "sentinel-append: nothing to add (${#names[@]} failing, $nlisted listed, $ngone gone)"
	exit 0
fi

# A file without a final newline gets one, so the blank line separates.
if [ -s "$sent" ] && [ -n "$(tail -c 1 "$sent")" ]; then
	echo >> "$sent"
fi
{
	echo
	echo "# Named by $note"
	printf '%s\n' "${kept[@]}"
} >> "$sent"
echo "sentinel-append: added ${#kept[@]} of ${#names[@]} (${kept[*]})"
if [ ${#cut[@]} -gt 0 ]; then
	echo "sentinel-append: over the cap of $max, not added: ${cut[*]}"
fi
