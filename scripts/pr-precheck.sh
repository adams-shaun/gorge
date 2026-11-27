#!/usr/bin/env bash
# pr-precheck.sh BASE: the checks that turned pull requests red in their fast stage (11-17: the function-shape
# ratchets, the generated support page, the PR's own new tests), run on the branch before agentctl open-pr pushes
# it. A few seconds to a few minutes; the PR's fast stage and its Mergify batch run the rest. The last line printed
# is the verdict: "pr-precheck: ok" or the first failure.
set -uo pipefail
base=${1:?usage: pr-precheck.sh BASE}
source scripts/pr-cards.sh
cap=(systemd-run --user --scope -q -p MemoryMax=4G -p CPUQuota=400% env GOMAXPROCS=4 GOMEMLIMIT=3GiB)
# go vet type-checks the test packages; a cold cache over ./dirac ./dirac/oracle ./engine/rules runs two compiles plus vet
# in one scope and hit the 4G cap (11-26: OOM-killed, verdict empty). 8G/6GiB matches the dirac lockstep tests' cap.
vetcap=(systemd-run --user --scope -q -p MemoryMax=8G -p CPUQuota=400% env GOMAXPROCS=4 GOMEMLIMIT=6GiB)
log=$(mktemp); trap 'rm -f "$log"' EXIT
# vet_reason RC LOG: the verdict text for a failed go vet. An OOM kill writes no file:line (rc 137 is SIGKILL; go prints
# "signal: killed" for a killed child), so it is named; any other diagnostic-less failure shows the last log line.
vet_reason() {
  local why
  why=$(grep -m1 -E '^[^ #].*\.go:[0-9]+' "$2" | cut -c1-200)
  if [ -z "$why" ] && { [ "$1" = 137 ] || grep -q 'signal: killed' "$2"; }; then why="killed (memory cap 8G; rc $1)"; fi
  [ -n "$why" ] || why="rc $1: $(grep -v '^[[:space:]]*$' "$2" | tail -1 | cut -c1-200)"
  echo "$why"
}
fail() { echo "pr-precheck: $*"; exit 1; }
# A pass is stamped by the hash of the branch's own diff against $base (the merge-base diff). When main moves and a
# CLEAN merge or rebase leaves that diff unchanged, the stamp matches and the checks below are not run again: the
# merge queue's batch is the gate for the merge itself (11-26: a GLM subagent merged main and re-ran the whole
# battery in a loop, each time main had moved again). PR_PRECHECK_FORCE=1 re-runs. The key is read on stdin.
already_passed() {
  read -r key
  [ -n "$key" ] && [ -e "${PRECHECK_STAMP_DIR:-$HOME/.cache/gorge-precheck}/$key" ]
}
own_key=$(git diff --binary "$base"...HEAD | sha256sum | cut -d' ' -f1)
if [ -z "${PR_PRECHECK_FORCE:-}" ] && already_passed <<<"$own_key"; then
  echo "pr-precheck: ok (this diff already passed; main moving does not re-run it, the merge queue gates the merge)"
  exit 0
fi
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
# The branch name on stdin: regen/* owns the derived records (scripts/regen-main.sh
# regenerates them wholesale) and mergify/merge-queue/* replays regen branches, so the derived step below skips
# them. Detached HEAD resolves to "HEAD": not exempt.
derived_exempt() {
  case $(cat) in
  regen/* | mergify/merge-queue/*) return 0 ;;
  *) return 1 ;;
  esac
}
changed=$(git diff --name-only "$base"...HEAD)
# A branch still carrying a rise in the derived records conflicts with regen/main the moment it merges (11-26: 25
# open wt/* PRs CONFLICTING after each regen merge, every one rewriting support.md or a pin table). pr-derived.sh
# --check flags one: a record the diff touches is stale iff the tolerant test passes on $base's copy — a rise
# regen/main owns; a fall is kept by design. The corpus lives only here (pr-cards.sh), so the CI lint job and
# queue-precheck cannot run this step (a path-only rule would block every legitimate fall).
if grep -qxF -f <(bash scripts/pr-derived.sh --list) <<<"$changed"; then
  if ! derived_exempt <<<"${PR_PRECHECK_BRANCH:-$(git rev-parse --abbrev-ref HEAD)}"; then
    rc=0; out=$(bash scripts/pr-derived.sh --check "$base") || rc=$?
    if [ "$rc" = 1 ]; then
      files=${out##*pr-derived: stale }
      files=${files%%$'\n'*}
      fail "derived records $files carry a rise that regen/main owns: run scripts/pr-derived.sh $base and commit what it changes (a fall is kept by design)"
    elif [ "$rc" != 0 ]; then
      fail "derived records check: $out"
    fi
  fi
fi
if ! "${cap[@]}" go test -timeout 3m -run '^TestCodeShapeOnlyShrinks$' ./internal/codeshape >"$log" 2>&1; then
  # TestCodeShapeOnlyShrinks holds the tree-wide counts (stringLiteralCompares and the rest): a dirac ticket added two
  # string compares and failed its batch's test (0), as no affected-package rule selects ./internal/codeshape (#662, 11-17).
  cat "$log"; fail "codeshape: $(grep -o -E '[A-Za-z]+: measured [0-9]+, above the frozen ceiling of [0-9]+' "$log" | head -1)"
fi
# go vet over the packages the diff touches (lint runs ./... ; a vet error in a changed package is the one a PR can
# cause). --diff-filter=ACMR leaves out deleted files, so a removed package is not asked for.
vetpkgs=$(git diff --name-only --diff-filter=ACMR "$base"...HEAD -- '*.go' | xargs -r -n1 dirname | sort -u | sed 's|^|./|')
if [ -n "$vetpkgs" ]; then
  # shellcheck disable=SC2086
  rc=0; "${vetcap[@]}" go vet $vetpkgs >"$log" 2>&1 || rc=$?
  if [ "$rc" != 0 ]; then
    cat "$log"
    why=$(vet_reason "$rc" "$log")
    fail "go vet: $why"
  fi
fi
# The base-relative function growth rule (cmd/funcgrowth, #5106) runs in lint and in every merge batch; run it here so a
# PR learns it grew a function past its allowance before the push, not in the queue (11-26: batch #5138's lint failed on
# featPoolPays 45 -> 52 although each PR passed this script). A tree without the rule yet skips the step. Two PRs that each
# grow one function inside its allowance can still cross the threshold together: only the batch sees that sum, so keep a
# function's headroom for the other PRs (extract a helper) instead of using all of it.
if [ -d cmd/funcgrowth ]; then
  "${cap[@]}" go run ./cmd/funcgrowth -base "$base" >"$log" 2>&1 || { cat "$log"; fail "funcgrowth: $(grep -m1 -E 'grew to|grew past' "$log" | sed 's/^[[:space:]]*//' | cut -c1-200)"; }
fi
# The public boundary scans every tracked file, so no affected-package rule selects it: a test fixture holding a
# Forge script line verbatim passed the seat's checks and failed fast's TestNoVerbatimForgeLines (#613, 11-17).
if ! "${cap[@]}" go test -timeout 3m -run '^TestPublicBoundary$|^TestNoVerbatimForgeLines$' ./internal/archtest >"$log" 2>&1; then
  cat "$log"; fail "public boundary: $(grep -m1 -E 'holds a line of|private|verbatim|script lines|not this tree.s pin' "$log" | sed 's/^[[:space:]]*//' | cut -c1-200)"
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
# No repro replay here (operator 11-24, "whatever bullshit gate we added remove it"): ~20 min of ./dirac/oracle per
# engine/dirac PR behind one machine-wide lock queued 21 tickets in landing. The merge queue's dirac-repros job replays
# the same set once per batch.
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
# Two stamps: the git-dir one records the exact head this script passed and the pre-push hook (.githooks/pre-push)
# reads it; the diff-keyed one is how main moving does not re-run the checks (already_passed above).
git rev-parse HEAD >"$(git rev-parse --git-path pr-precheck-ok)" 2>/dev/null || true
sd=${PRECHECK_STAMP_DIR:-$HOME/.cache/gorge-precheck}
mkdir -p "$sd" && : >"$sd/$own_key" && find "$sd" -type f -mtime +7 -delete 2>/dev/null
echo "pr-precheck: ok"
