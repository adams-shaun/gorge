#!/usr/bin/env bash
# pr-derived.sh BASE: leave the derived records a ticket's rise moves at BASE's copy, so the pull request does not
# carry them. agentctl open-pr runs it on the replayed branch, before scripts/pr-precheck.sh, and commits what it
# leaves changed.
#   dirac/docs/support.md                      TestSupportDocIsCurrent passes a page stale by rises only
#   dirac/kgen/testdata/acceptance.tsv,        kgen's acceptance pins pass a rise outside DIRAC_PIN_STRICT=1
#   dirac/kgen/testdata/build_acceptance.tsv
#   dirac/kgen/testdata/remspan.tsv            TestRememberedSpanCensus passes a shrink (refused cards fewer) outside strict
# Every dirac ticket raised the same few lines of these files, so each batch merge put the open ones in conflict with
# main (11-17: 10 PRs dequeued per batch). A rise now reaches main through one regen PR (scripts/regen-main.sh). A fall
# fails the tolerant check on the base copy, so the ticket keeps its own copy and its reason. The nightly
# (DIRAC_SUPPORT_STRICT=1, DIRAC_PIN_STRICT=1) still holds main's copies exact.
set -uo pipefail
base=${1:?usage: pr-derived.sh BASE}
cap=(systemd-run --user --scope -q -p MemoryMax=4G -p CPUQuota=400% env GOMAXPROCS=4 GOMEMLIMIT=3GiB)
pins='^(TestCorpusAcceptance|TestSetAcceptance|TestCorpusBuildAcceptance|TestPinTablesParse)$'
changed() { git diff --name-only "$base" HEAD -- "$@"; }

# leave REGEX FILE...: take BASE's copy of the files; keep it when the tolerant tests pass on it, else restore HEAD's.
# PKG (default ./dirac/kgen) is the package whose tests judge them.
leave() {
  local rx=$1; shift
  git checkout -q "$base" -- "$@" || return 0
  if "${cap[@]}" env -u DIRAC_PIN_STRICT -u DIRAC_SUPPORT_STRICT go test -timeout 5m -run "$rx" "${PKG:-./dirac/kgen}" >/dev/null 2>&1; then
    echo "pr-derived: left $* at $base (a rise; scripts/regen-main.sh carries it to main)"
  else
    git checkout -q HEAD -- "$@"
    echo "pr-derived: kept $* (the tolerant check fails on $base's copy: a fall, which the ticket carries)"
  fi
}

pinfiles=(dirac/kgen/testdata/acceptance.tsv dirac/kgen/testdata/build_acceptance.tsv)
remspan=dirac/kgen/testdata/remspan.tsv
[ -n "$(changed dirac/docs/support.md "${pinfiles[@]}" "$remspan")" ] || exit 0
source scripts/pr-cards.sh
# The pins first: both tables in one step, as KGEN_REPIN writes them together.
[ -n "$(changed "${pinfiles[@]}")" ] && leave "$pins" "${pinfiles[@]}"
# remspan on its own: a ticket that refuses a card grows it (kept, it fails the tolerant check) while its acceptance
# pins still only rise (#632 11-17: one group kept both and the rise conflicted).
[ -n "$(changed "$remspan")" ] && leave '^TestRememberedSpanCensus$' "$remspan"
[ -n "$(changed dirac/docs/support.md)" ] && leave '^TestSupportDocIsCurrent$' dirac/docs/support.md
exit 0
