#!/usr/bin/env bash
# pr-precheck.sh BASE: the checks that turned pull requests red in their fast stage (11-17: the function-shape
# ratchets, the generated support page, the PR's own new tests), run on the branch before agentctl open-pr pushes
# it. A few seconds to a few minutes; the PR's fast stage and its Mergify batch run the rest. The last line printed
# is the verdict: "pr-precheck: ok" or the first failure.
set -uo pipefail
base=${1:?usage: pr-precheck.sh BASE}
source scripts/pr-cards.sh
cap=(systemd-run --user --scope -q -p MemoryMax=4G -p CPUQuota=400% env GOMAXPROCS=4 GOMEMLIMIT=3GiB)
log=$(mktemp); trap 'rm -f "$log"' EXIT
fail() { echo "pr-precheck: $*"; exit 1; }
changed=$(git diff --name-only "$base"...HEAD)
if ! "${cap[@]}" go test -timeout 3m -run '^TestComplexFunctionsOnlyShrink$|^TestLongFunctionsOnlyShrink$|^TestFuncCeilingsLowerOnly$|^TestCodeShapeOnlyShrinks$' ./internal/codeshape >"$log" 2>&1; then
  # TestCodeShapeOnlyShrinks holds the tree-wide counts (stringLiteralCompares and the rest): a dirac ticket added two
  # string compares and failed its batch's test (0), as no affected-package rule selects ./internal/codeshape (#662, 11-17).
  cat "$log"; fail "codeshape: $(grep -o -E '[^ ]+ \([^)]*\) grew to [a-z]+ [0-9]+, above its ceiling of [0-9]+|[A-Za-z]+: measured [0-9]+, above the frozen ceiling of [0-9]+' "$log" | head -1)"
fi
# The public boundary scans every tracked file, so no affected-package rule selects it: a test fixture holding a
# Forge script line verbatim passed the seat's checks and failed fast's TestNoVerbatimForgeLines (#613, 11-17).
if ! "${cap[@]}" go test -timeout 3m -run '^TestPublicBoundary$|^TestNoVerbatimForgeLines$' ./internal/archtest >"$log" 2>&1; then
  cat "$log"; fail "public boundary: $(grep -m1 -E 'holds a line of|private|verbatim' "$log" | sed 's/^[[:space:]]*//' | cut -c1-200)"
fi
# The wall-clock ratchet in ./testkit/testutil scans every _test.go file, and no affected-package rule selects it.
# On 11-18, #1705 added a 5 ms poll sleep in server/gorged and failed its batch's test (2) on
# TestNoNewSubSecondWallClockThresholdInTests.
if grep -q '_test\.go$' <<<"$changed"; then
  if ! "${cap[@]}" go test -timeout 3m -run '^TestNoNewSubSecondWallClockThresholdInTests$' ./testkit/testutil >"$log" 2>&1; then
    cat "$log"; fail "wall-clock ratchet: $(grep -m1 -E 'holds [0-9]+ sub-second' "$log" | sed 's/^[[:space:]]*//' | cut -c1-200)"
  fi
fi
if grep -q '^dirac/' <<<"$changed"; then
  # The support ledger's own checks (dirac/kgen/support_test.go): a refusal against evidence main credited since the
  # branch cut fails TestEvidenceAndRefusedAreDisjoint though the PR touches neither test (#503, 11-17).
  ledger='^(TestSupportDocIsCurrent|TestEvidenceAndRefusedAreDisjoint|TestRefusedEntriesWellFormed|TestRefusedReproParses|TestSupportHasNoScriptText)$'
  "${cap[@]}" go test -timeout 5m -run "$ledger" ./dirac/kgen >"$log" 2>&1 || { cat "$log"; fail "support ledger: $(grep -m1 -A1 -- '--- FAIL' "$log" | tr '\n' ' ') (a stale page: scripts/pr-regen.sh dirac/docs/support.md)"; }
fi
# A package's census and pin ratchets: #611 registered kw:Modular but left its pin in granted-keyword-gaps.txt, and
# TestGrantedKeywordHeadsAreRegisteredOrPinned failed in the batch (11-17). engine/rules' 225 run in ~32 s. The
# append-only ordinals and layouts too: #635 added an ask kind with no manifest record and TestAskKindsUnchanged
# failed fast and lint (11-17).
# #627 (11-17): TestNoNewFixedStateFields (dirac State layout) matched no census word and failed lint.
census='Census|Pinned|Ratchet|OnlyShrink|AreRegistered|KindsUnchanged|OrdinalsUnchanged|LayoutFrozen|NoNewFixedStateFields|HashPins'
for pkg in $(grep -E '\.(go|txt|tsv|json)$' <<<"$changed" | xargs -r -n1 dirname | sed -E 's#/testdata(/.*)?$##' | sort -u); do
  [ -n "$(ls "$pkg"/*_test.go 2>/dev/null)" ] || continue
  grep -q -E "^func Test[A-Za-z0-9_]*($census)" "$pkg"/*_test.go || continue
  "${cap[@]}" go test -timeout 5m -run "$census" "./$pkg" >"$log" 2>&1 || { cat "$log"; fail "census in $pkg: $(grep -o -E -- '--- FAIL: [A-Za-z0-9_/]+' "$log" | head -2 | sed 's/--- FAIL: //' | paste -sd' ')"; }
done
for pkg in $(grep '_test\.go$' <<<"$changed" | xargs -r -n1 dirname | sort -u); do
  rx=$(grep "^$pkg/[^/]*_test\.go$" <<<"$changed" | while read -r f; do [ -f "$f" ] && sed -n 's/^func \(Test[A-Za-z0-9_]*\)(t \*testing\.T).*/\1/p' "$f"; done | sort -u | paste -sd'|')
  [ -n "$rx" ] || continue
  "${cap[@]}" go test -timeout 5m -run "^($rx)\$" "./$pkg" >"$log" 2>&1 || { cat "$log"; fail "tests in $pkg: $(grep -o -E -- '--- FAIL: [A-Za-z0-9_/]+' "$log" | head -2 | sed 's/--- FAIL: //' | paste -sd' ')"; }
done
# The same new dirac tests as a fresh clone runs them (lint's fresh-clone-check: no corpus, no tables): each must skip,
# never fail. #747 (11-17) added a lockstep repro that failed "no card corpus" there; with .cards linked it passed.
fresh=$(grep -E '^dirac/.*_test\.go$' <<<"$changed" | xargs -r -n1 dirname | sort -u)
if [ -n "$fresh" ]; then
  scratch=$(mktemp -d -p "${PRECHECK_FRESH_DIR:-$HOME/.cache}" pr-precheck-fresh.XXXXXX)
  # cardset walks up from the cwd looking for .cards: one above the scratch directory is a corpus, and the step would
  # read green on tests that must skip (fresh-clone-check.sh has the same guard).
  d=$scratch
  while :; do
    [ ! -e "$d/.cards" ] || { rm -rf "$scratch"; fail "fresh clone: $d/.cards is above the scratch directory: set PRECHECK_FRESH_DIR"; }
    [ "$d" != / ] || break
    d=$(dirname "$d")
  done
  git archive HEAD | tar -x -C "$scratch"
  # A clone has a .git, as scripts/fresh-clone-check.sh gives its export: without one testutil.CorpusDirOrSkip fails
  # rather than skips, a false red on every PR with a new corpus test (the #788 seat found it, 11-17).
  git init -q "$scratch"
  for pkg in $fresh; do
    rx=$(grep "^$pkg/[^/]*_test\.go$" <<<"$changed" | while read -r f; do [ -f "$f" ] && sed -n 's/^func \(Test[A-Za-z0-9_]*\)(t \*testing\.T).*/\1/p' "$f"; done | sort -u | paste -sd'|')
    [ -n "$rx" ] || continue
    (cd "$scratch" && unset GORGE_CARDS GORGE_CARDS_DIR DIRAC_TABLES GORGE_POOLS KGEN_DECKS &&
      GOWORK=off GOFLAGS=-trimpath "${cap[@]}" go test -timeout 5m -run "^($rx)\$" "./$pkg") >"$log" 2>&1 ||
      { cat "$log"; rm -rf "$scratch"; fail "fresh clone, tests in $pkg (a corpus test must skip without .cards): $(grep -o -E -- '--- FAIL: [A-Za-z0-9_/]+' "$log" | head -2 | sed 's/--- FAIL: //' | paste -sd' ')"; }
  done
  rm -rf "$scratch"
fi
echo "pr-precheck: ok"
