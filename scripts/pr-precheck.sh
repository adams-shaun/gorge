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
# gofmt, the lint job's first step (.github/workflows/ci.yml): #1964 printed "pr-precheck: ok" here and then failed
# lint on ./dirac/turn_gen.go and ./dirac/combat_mustbeblocked.go, needing a hand gofmt commit and a requeue. First,
# before any heavy step. --diff-filter=ACMR is the "still exist" half: a file the branch deletes is not there for
# gofmt to read.
unformatted=$(git diff --name-only --diff-filter=ACMR "$base"...HEAD -- '*.go' | xargs -r gofmt -l)
if [ -n "$unformatted" ]; then
  fail "gofmt: run gofmt -w $unformatted"
fi
# The gate for the golden-head step below, on a `git diff --name-only` list: the golden-head tests and the
# corpus-wide ratchets it runs live in ./engine/rules, so the affected-package steps never select them when the
# change lands in engine/{events,effects,state,cards} or cmd/forgec. #1758 appended events.CrewAction, pr-precheck
# said ok, and batch #1798 failed on the 10 TestSetHeads goldens it moved; #1710 left stale TestTakeInitiative
# exceptions. internal/precheckgate runs this function on fake diff lists.
heads_gate() {
  grep -q -E '^(engine/(events|effects|state|rules|cards)/|cmd/forgec/)'
}
# The gate for the repro-replay step below: any non-test, non-doc change under engine/ or dirac/ can move a recorded
# game, so a refused card's repro may stop diverging (TestRefusedEntriesStillDiverge, the *Repros*Lockstep stale
# checks). Only the queue batch's dirac-repros job replayed them: batch #4212 (11-24) went red on 8 refused cards one
# of its PRs had fixed, and #2551 reddened #4184 and #4200 the same way. awk reads all its input, so pipefail never
# sees a SIGPIPE (see the TestDiracFileNames step).
repros_gate() {
  awk '/^(engine|dirac)\// && !/(_test\.go|\.md)$/ {f = 1} END {exit !f}'
}
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
  # #1836 added dirac/stallclass_twoseatcauses.go — stallclass is core, not a topic — and pr-precheck said ok: no
  # step here runs TestDiracFileNames, and the batch failed on it. Only an added file can introduce a bad name, so
  # --no-renames surfaces a rename into one as an added file too; the topic list is dirac/filenames_test.go's
  # (doc.go lists the topics). The list is captured first: with `| grep -q` under pipefail, git diff's next write
  # after grep exits at the first match gets SIGPIPE (exit 141) and the if reads false — the step silently skips
  # exactly on the large dirac PRs it exists for.
  added=$(git diff --name-only --no-renames --diff-filter=A "$base"...HEAD -- 'dirac/')
  if grep -q '\.go$' <<<"$added"; then
    "${cap[@]}" go test -timeout 3m -run '^TestDiracFileNames$' ./dirac >"$log" 2>&1 || { cat "$log"; fail "dirac file names: $(grep -m1 -E 'neither core|named for where|ends in a number' "$log" | sed 's/^[[:space:]]*//' | cut -c1-200) — rename it <topic>_<mechanic>.go (doc.go lists the topics; rounds of one mechanic share a file, no digits)"; }
  fi
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
# The golden-head tests and the corpus-wide ratchets the census step never selects: they live in ./engine/rules (and
# TestKnownApproximationsOnlyShrinks in ./testkit/testutil), so a change in engine/{events,effects,state,cards} or
# cmd/forgec moves them without a changed file in those packages — #1758's events.CrewAction said ok here and failed
# batch #1798 on the 10 TestSetHeads goldens it moved; #1710 left stale TestTakeInitiative exceptions behind.
# GORGE_UPDATE_SETHEADS re-pins TestSetHeads, GORGE_UPDATE_HEADS TestHeads; the commit message must name the first
# diverging event (GORGE_DUMP_SETHEADS=<dir>, then diff the dump against base).
if heads_gate <<<"$changed"; then
  heads='^(TestSetHeads|TestHeads|TestEveryRepoDeckIsFullySupported|TestEveryRepoDeckParamsAreRead|TestEveryStandardSetParamsAreRead|TestTakeInitiative.*|TestKnownApproximationsOnlyShrinks)$'
  # #1787 read EffectZone/ExcludeZone in engine/ and pr-precheck said ok: the known-key census tests live in
  # ./engine/rules, which the census step above never selects from a change outside it, and the batch failed on 21
  # stale engine/effects/*_params.go tables.
  if grep -qE '^(engine/|cmd/forgec/)' <<<"$changed"; then
    heads='^(TestSetHeads|TestHeads|TestEveryRepoDeckIsFullySupported|TestEveryRepoDeckParamsAreRead|TestEveryStandardSetParamsAreRead|TestTakeInitiative.*|TestKnownApproximationsOnlyShrinks|Test[A-Za-z0-9_]*KnownKeysMatchTheCensus)$'
  fi
  if ! "${cap[@]}" go test -timeout 5m -run "$heads" ./engine/rules ./testkit/testutil >"$log" 2>&1; then
    cat "$log"
    why="re-pin with GORGE_UPDATE_SETHEADS=1 (or GORGE_UPDATE_HEADS=1 for TestHeads), and name the first diverging event in the commit message (GORGE_DUMP_SETHEADS=<dir>, then diff the dump against base)"
    if grep -q 'KnownKeysMatchTheCensus' "$log"; then
      # The census's -update rewrites the *_params.go tables for the NEXT run only (the package var is loaded
      # before the rewrite), so it must be run twice: once with -update, once without to verify.
      why="regenerate the params tables: go test ./engine/rules -run 'KnownKeysMatchTheCensus' -update, then run it again without -update to verify (the rewrite lands for the NEXT run: the package var was loaded before it) — and $why"
    fi
    fail "golden heads: $(grep -o -E -- '--- FAIL: Test[A-Za-z0-9_/]+' "$log" | head -3 | sed 's/--- FAIL: //' | paste -sd' ') — $why"
  fi
fi
# The same set ci.yml's dirac-repros job replays (both parts): every repro test and TestRefusedEntriesStillDiverge.
if repros_gate <<<"$changed"; then
  names=$(go test -list 'Repro|^TestRefusedEntriesStillDiverge$' ./dirac/oracle 2>/dev/null | grep -E '^Test' | paste -sd'|')
  [ -n "$names" ] || fail "repro replay: go test -list found no repro tests in ./dirac/oracle"
  if ! "${cap[@]}" go test -timeout 20m -run "^($names)\$" ./dirac/oracle >"$log" 2>&1; then
    cat "$log"
    why="a refused card's recorded game no longer diverges: if the PR fixed it, add a row to dirac/support/unrefused.tsv and run go run ./dirac/cmd/kgen -unrefuse <card>; if the game moved, re-record it (see checkRefusedRepro)"
    grep -q 'stale' "$log" || why="a recorded game now diverges: fix the change, or refuse the card by name (kgen -refuse)"
    fail "repro replay: $(grep -o -E -- '--- FAIL: [A-Za-z0-9_/,.'"'"'-]+' "$log" | head -4 | sed 's/--- FAIL: //' | paste -sd' ') — $why"
  fi
fi
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
