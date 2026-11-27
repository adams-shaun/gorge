#!/usr/bin/env bash
# check-forge-ref-driver.sh <base>: refuse a FORGE_REF move the Forge oracle driver does not contain.
#
# gorge-compliance's Forge oracle (xproof, the farm) runs a driver commit on our fork (its FORGE_ORACLE_REF), and
# forge-oracle-setup.sh refuses any FORGE_REF that is not an ancestor of it. On 2026-11-25 gorge moved FORGE_REF to
# 5bc5851a54 before the driver had it; gorge-compliance's pin follow carried it over and every xproof cycle failed for
# hours. The fork's oracle-driver branch is the driver line: gorge-compliance's forge-driver-follow workflow merges a
# FORGE_REF that gorge main or an open gorge pull request names into it (no person needed; a run every 15 min, or
# `gh workflow run forge-driver-follow.yml -R adams-shaun/gorge-compliance`), and its gorge-pin-follow then sets
# FORGE_ORACLE_REF to that branch with the pin bump.
#
# The check runs only when the diff against <base> moves FORGE_REF. It asks the fork compare API for
# compare/<driver>...<FORGE_REF>: `behind` or `identical` means the driver contains FORGE_REF. FORGE_DRIVER_REF
# overrides the driver (a branch or a commit; default oracle-driver).
#
# Exit: 0 unchanged or contained; 1 not contained; 2 usage or environment (an unreadable <base>:Makefile, no gh, the
# API could not answer).
set -euo pipefail

base=${1:?usage: check-forge-ref-driver.sh <base>}
fork=${FORGE_FORK:-adams-shaun/forge}

die() {
	echo "check-forge-ref-driver: $*" >&2
	exit 2
}
pin() { awk -v n="$1" '$1 == n && ($2 == "?=" || $2 == "=") { v = $3; sub(/#.*/, "", v); print v; exit }'; }

# The checkout may not carry <base>: a broken or shallow workspace store makes git show fail, so a FORGE_REF that has
# not moved reads as moved (old comes back empty) and the step used to die on the gh line before the compare API could
# answer, blaming a missing gh for an unreadable base. This happened twice: 2026-11-26 16:27Z on metal2-gorge-box-2 for
# PR #4816 (4f8d2c6491 is the merge ref's own parent and its Makefile still failed to read), and 2026-11-27 17:25Z for
# PR #5621, which does not touch FORGE_REF at all. Fetch the base when the checkout cannot read it; if it is still
# unreadable, die before the gh check with the true cause rather than guessing that the pin moved. The verdicts below
# are unchanged and a real move still goes to the compare API.
if ! git show "$base:Makefile" >/dev/null 2>&1; then
	echo "check-forge-ref-driver: base ${base:0:10} is not readable in the checkout; fetching it from origin" >&2
	git fetch -q origin "$base" 2>/dev/null || true
fi
if ! git show "$base:Makefile" >/dev/null 2>&1; then
	die "cannot read Makefile at base $base; refusing to guess whether FORGE_REF moved (fetch the base, e.g. actions/checkout fetch-depth: 0)"
fi

new=$(pin FORGE_REF <Makefile)
old=$(git show "$base:Makefile" 2>/dev/null | pin FORGE_REF) || old=
[[ $new =~ ^[0-9a-f]{40}$ ]] || die "FORGE_REF is not a full commit sha: '$new'"
if [ "$new" = "$old" ]; then
	echo "check-forge-ref-driver: FORGE_REF unchanged (${new:0:10})"
	exit 0
fi

command -v gh >/dev/null 2>&1 || die "gh is needed"
driver=${FORGE_DRIVER_REF:-oracle-driver}
[[ $driver =~ ^[A-Za-z0-9._-]+$ ]] || die "driver is not a branch name or commit: '$driver'"

errf=$(mktemp "${TMPDIR:-/tmp}/check-forge-ref-driver.XXXXXX")
trap 'rm -f "$errf"' EXIT
if ! status=$(gh api "repos/$fork/compare/$driver...$new" --jq .status 2>"$errf"); then
	cat "$errf" >&2
	if grep -Eq 'HTTP (404|422)|No commit found' "$errf"; then
		echo "check-forge-ref-driver: FORGE_REF ${new:0:10} or the driver $driver is not in the $fork network" >&2
		exit 1
	fi
	die "the GitHub compare API could not answer"
fi
case $status in
behind | identical)
	echo "check-forge-ref-driver: FORGE_REF ${old:0:10} -> ${new:0:10} is contained in the driver $driver on $fork"
	;;
*)
	echo "check-forge-ref-driver: FORGE_REF ${new:0:10} is not contained in the Forge oracle driver $driver on $fork (compare: $status)." >&2
	echo "Wait for gorge-compliance's forge-driver-follow to merge it (every 15 min; or: gh workflow run forge-driver-follow.yml -R adams-shaun/gorge-compliance), then re-run this check." >&2
	exit 1
	;;
esac
