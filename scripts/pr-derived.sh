#!/usr/bin/env bash
# pr-derived.sh BASE: leave the derived records a ticket's rise moves at BASE's copy, so the pull request does not
# carry them. agentctl open-pr runs it on the replayed branch, before scripts/pr-precheck.sh, and commits what it
# leaves changed. --list prints the records (the one home of the list); --check BASE runs the same groups but only
# reports, always restoring HEAD's copy, and exits 1 with "pr-derived: stale <files>" when a record the diff touches
# is still a rise (the tolerant test passes on BASE's copy) — a fall is kept by design. scripts/pr-precheck.sh runs
# --check, because only it has the corpus (pr-cards.sh) that separates a rise from a fall.
#   dirac/docs/support.md                      TestSupportDocIsCurrent passes a page stale by rises only
#   dirac/kgen/testdata/acceptance.tsv,        kgen's acceptance pins pass a rise outside DIRAC_PIN_STRICT=1
#   dirac/kgen/testdata/build_acceptance.tsv
#   dirac/kgen/testdata/remspan.tsv            TestRememberedSpanCensus passes a shrink (refused cards fewer) outside strict
# Every dirac ticket raised the same few lines of these files, so each batch merge put the open ones in conflict with
# main (11-17: 10 PRs dequeued per batch). A rise now reaches main through one regen PR (scripts/regen-main.sh). A fall
# fails the tolerant check on the base copy, so the ticket keeps its own copy and its reason. The nightly
# (DIRAC_SUPPORT_STRICT=1, DIRAC_PIN_STRICT=1) still holds main's copies exact.
set -uo pipefail
cap=(systemd-run --user --scope -q -p MemoryMax=4G -p CPUQuota=400% env GOMAXPROCS=4 GOMEMLIMIT=3GiB)
pins='^(TestCorpusAcceptance|TestSetAcceptance|TestCorpusBuildAcceptance|TestPinTablesParse)$'
# The one home of the derived-record list: every group below is built from it, and --list prints it for
# scripts/pr-precheck.sh's step (which needs to know when a diff touches any of them).
derived=(dirac/docs/support.md dirac/kgen/testdata/acceptance.tsv dirac/kgen/testdata/build_acceptance.tsv dirac/kgen/testdata/remspan.tsv)
support=${derived[0]}
pinfiles=("${derived[1]}" "${derived[2]}")
remspan=${derived[3]}
stale=()
if [ "${1:-}" = --list ]; then
  printf '%s\n' "${derived[@]}"
  exit 0
fi
mode=
if [ "${1:-}" = --check ]; then
  mode=check
  base=${2:?usage: pr-derived.sh --check BASE}
  git diff --quiet HEAD -- "${derived[@]}" || { echo "pr-derived: --check needs the derived files committed"; exit 2; }
else
  base=${1:?usage: pr-derived.sh BASE}
fi
# The working tree, not HEAD: pr-regen writes remspan.tsv as a side effect of
# KGEN_REPIN=1 without staging it, so a HEAD-only diff misses the rewrite and
# the caller's `git add -u` then commits it (a rise regen/main owns).
changed() { git diff --name-only "$base" -- "$@"; }

# leave REGEX FILE...: take BASE's copy of the files; keep it when the tolerant tests pass on it, else restore
# HEAD's. In --check mode nothing is kept: a pass records the files as stale (a rise regen/main owns) and HEAD's
# copy is always restored. PKG (default ./dirac/kgen) is the package whose tests judge them.
leave() {
  local rx=$1; shift
  git checkout -q "$base" -- "$@" || return 0
  if "${cap[@]}" env -u DIRAC_PIN_STRICT -u DIRAC_SUPPORT_STRICT go test -timeout 5m -run "$rx" "${PKG:-./dirac/kgen}" >/dev/null 2>&1; then
    if [ -z "$mode" ]; then
      echo "pr-derived: left $* at $base (a rise; scripts/regen-main.sh carries it to main)"
    else
      stale+=("$@")
      git checkout -q HEAD -- "$@"
    fi
  else
    git checkout -q HEAD -- "$@"
    echo "pr-derived: kept $* (the tolerant check fails on $base's copy: a fall, which the ticket carries)"
  fi
}

finish() {
  if [ -n "$mode" ] && [ "${#stale[@]}" -gt 0 ]; then
    echo "pr-derived: stale ${stale[*]}"
    exit 1
  fi
  exit 0
}
[ -n "$(changed "$support" "${pinfiles[@]}" "$remspan")" ] || finish
source scripts/pr-cards.sh
# The pins first: both tables in one step, as KGEN_REPIN writes them together.
[ -n "$(changed "${pinfiles[@]}")" ] && leave "$pins" "${pinfiles[@]}"
# remspan on its own: a ticket that refuses a card grows it (kept, it fails the tolerant check) while its acceptance
# pins still only rise (#632 11-17: one group kept both and the rise conflicted).
[ -n "$(changed "$remspan")" ] && leave '^TestRememberedSpanCensus$' "$remspan"
[ -n "$(changed "$support")" ] && leave '^TestSupportDocIsCurrent$' "$support"
finish
