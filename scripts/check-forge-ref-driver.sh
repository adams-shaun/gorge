#!/usr/bin/env bash
# check-forge-ref-driver.sh <base>: refuse a FORGE_REF move the Forge oracle driver does not contain.
#
# gorge-compliance's Forge oracle (xproof, the farm) runs a driver commit on our fork, FORGE_ORACLE_REF in
# gorge-compliance develop's Makefile, and its forge-oracle-setup.sh refuses any FORGE_REF that is not an ancestor
# of it. On 2026-11-25 gorge moved FORGE_REF to 5bc5851a54 before the driver had it; gorge-compliance's pin follow
# carried it over and every xproof cycle failed for hours. So the order of a bump is: merge the new FORGE_REF commit
# into the driver on the fork, land the FORGE_ORACLE_REF bump in gorge-compliance develop, then move FORGE_REF here
# (Makefile comment above FORGE_REF).
#
# The check runs only when the diff against <base> moves FORGE_REF. It asks the fork compare API for
# compare/<driver>...<FORGE_REF>: `behind` or `identical` means the driver contains FORGE_REF. FORGE_DRIVER_REF
# overrides the driver commit (a bump checked before the compliance PR lands).
#
# Exit: 0 unchanged or contained; 1 not contained; 2 usage or environment (no gh, the API could not answer).
set -euo pipefail

base=${1:?usage: check-forge-ref-driver.sh <base>}
fork=${FORGE_FORK:-adams-shaun/forge}
compliance=${FORGE_COMPLIANCE_REPO:-adams-shaun/gorge-compliance}

die() {
	echo "check-forge-ref-driver: $*" >&2
	exit 2
}
pin() { awk -v n="$1" '$1 == n && ($2 == "?=" || $2 == "=") { v = $3; sub(/#.*/, "", v); print v; exit }'; }

new=$(pin FORGE_REF <Makefile)
old=$(git show "$base:Makefile" 2>/dev/null | pin FORGE_REF) || old=
[[ $new =~ ^[0-9a-f]{40}$ ]] || die "FORGE_REF is not a full commit sha: '$new'"
if [ "$new" = "$old" ]; then
	echo "check-forge-ref-driver: FORGE_REF unchanged (${new:0:10})"
	exit 0
fi

command -v gh >/dev/null 2>&1 || die "gh is needed"
driver=${FORGE_DRIVER_REF:-}
src="FORGE_DRIVER_REF"
if [ -z "$driver" ]; then
	content=$(gh api "repos/$compliance/contents/Makefile?ref=develop" --jq .content) ||
		die "cannot read $compliance develop's Makefile"
	driver=$(printf '%s' "$content" | tr -d '\n' | base64 -d | pin FORGE_ORACLE_REF)
	src="$compliance develop FORGE_ORACLE_REF"
fi
[[ $driver =~ ^[0-9a-f]{40}$ ]] || die "driver ($src) is not a full commit sha: '$driver'"

errf=$(mktemp "${TMPDIR:-/tmp}/check-forge-ref-driver.XXXXXX")
trap 'rm -f "$errf"' EXIT
if ! status=$(gh api "repos/$fork/compare/$driver...$new" --jq .status 2>"$errf"); then
	cat "$errf" >&2
	if grep -Eq 'HTTP (404|422)|No commit found' "$errf"; then
		echo "check-forge-ref-driver: FORGE_REF ${new:0:10} or the driver ${driver:0:10} is not in the $fork network" >&2
		exit 1
	fi
	die "the GitHub compare API could not answer"
fi
case $status in
behind | identical)
	echo "check-forge-ref-driver: FORGE_REF ${old:0:10} -> ${new:0:10} is contained in the driver ${driver:0:10} ($src)"
	;;
*)
	echo "check-forge-ref-driver: FORGE_REF ${new:0:10} is not contained in the Forge oracle driver ${driver:0:10} ($src; compare: $status)." >&2
	echo "Merge ${new:0:10} into the driver on $fork and land the FORGE_ORACLE_REF bump in $compliance develop first." >&2
	exit 1
	;;
esac
