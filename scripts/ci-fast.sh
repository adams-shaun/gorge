#!/usr/bin/env bash
# ci-fast.sh <base>: the fast stage of a pull request's CI (Mergify two-step CI, operator 11-16). It runs what
# `test-targets.sh list` selects for the diff against <base>: the packages the change touches, every package whose
# tests import them, and the named test groups. The full suite runs once per Mergify batch, on mergify/merge-queue/*
# (ci.yml), not on every pull request. Every run here passes -short, so the slow whole-game self-play/search tests
# skip it; the Mergify batch runs the suite without -short and runs them (they declare it with testing.Short).
#
# The big go test calls (the oracle's changed-test run, the kgen -count=1 run and the whole-package batch) run side
# by side instead of one after another: measured 11-26, they are otherwise a serial prefix — on a dirac PR the
# oracle run (154.5 s) and kgen (41.0 s) ran alone before a 331 s batch, 227 s of the job's 564 s with the box's
# spare cores idle (the batch itself finished 1-2 packages at a time). Each call keeps its own flags and -timeout;
# their output interleaves in the step log, and every [kind] line names its call.
set -uo pipefail
list=$(BASE=$1 bash scripts/test-targets.sh list) || { echo "ci-fast: test-targets.sh list failed"; exit 1; }
printf '%s\n' "$list"
whole=$(awk -F'\t' '$1 == "whole" {print $2}' <<<"$list")
rc=0
pids=(); kinds=()
# bg KIND CMD...: start one big go test call in the background, labelled for the step log.
bg() {
  local kind=$1
  shift
  echo "ci-fast: [$kind] $*"
  "$@" &
  pids+=("$!"); kinds+=("$kind")
}
# ./dirac/oracle whole is the full lockstep suite (over 20 min: 11-17 #443 timed out); the oracle job runs it nightly
# and the batch's dirac jobs cover it. Here it runs only the tests in its changed test files and, when the diff adds
# or edits a recording, every repro replay (the recording's pool test is not named by the diff).
if grep -qx './dirac/oracle' <<<"$whole"; then
  whole=$(grep -vx './dirac/oracle' <<<"$whole" || true)
  changed=$(git diff --name-only "$1"...HEAD -- 'dirac/oracle/*_test.go' 2>/dev/null)
  rx=$(for f in $changed; do [ -f "$f" ] && sed -n 's/^func \(Test[A-Za-z0-9_]*\)(t \*testing\.T).*/\1/p' "$f"; done | sort -u | paste -sd'|')
  # CI_FAST_SKIP_ORACLE_REPLAY=1 (ci.yml's fast job sets it): a pull request does not replay every repro recording.
  # The queue batch's dirac-repros job replays them all on the merged tree before anything lands, and the seat's
  # precheck replayed them on the ticket's branch. Measured 11-20: 402 s of a 1266 s fast job on a dirac PR, with
  # both boxes saturated (8 runners on one batch, 4 on 54 queued PR runs). The changed test functions still run.
  if [ "${CI_FAST_SKIP_ORACLE_REPLAY:-0}" != 1 ] && git diff --name-only "$1"...HEAD -- dirac/oracle/testdata | grep -q .; then rx=${rx:+$rx|}'Test[A-Za-z0-9]*Repros?[A-Za-z0-9]*Lockstep'; fi
  if [ -n "$rx" ]; then
    echo "ci-fast: ./dirac/oracle runs -run '^($rx)\$' only"
    bg oracle go test -timeout 15m -short -run "^($rx)\$" ./dirac/oracle
  fi
fi
# ./dirac/kgen runs uncached (-count=1): its tests open the corpus ~88 times, and go spent 190-300 s hashing those
# files for a test-result cache that a changed tree never hits, against ~110 s of tests (11-17; ci.yml's shards do the same).
# CI_FAST_SKIP_KGEN=1 (ci.yml's fast job sets it) drops that run from a pull request: the queue batch runs the whole
# package (not -short) in shard 0 of the same commit's merge, so the -short run here is a strict subset of work that
# runs again before anything lands. Measured 11-20: 296 s of a 1266 s fast job on a dirac PR (0 of 25 packages cached).
# A local `make test-targeted` / precheck still runs it (the variable is unset there).
# The skip holds only when kgen is a reverse dependency: a diff that edits dirac/kgen itself runs it here. On 11-20
# #2579 (lower_tapextra.go) passed fast with kgen skipped, then reddened a 4-PR batch on four kgen refusal tests.
if grep -qx './dirac/kgen' <<<"$whole"; then
  whole=$(grep -vx './dirac/kgen' <<<"$whole" || true)
  if [ "${CI_FAST_SKIP_KGEN:-0}" = 1 ] && ! git diff --name-only "$1"...HEAD -- 'dirac/kgen/*' 2>/dev/null | grep -q .; then
    echo "ci-fast: ./dirac/kgen skipped here; the queue batch runs it in full"
  else
    bg kgen go test -timeout 20m -short -count=1 ./dirac/kgen
  fi
fi
if [ -n "$whole" ]; then
  # shellcheck disable=SC2086
  bg batch go test -timeout 20m -short $whole
fi
while IFS=$'\t' read -r m p r; do
  [ "$m" = named ] || continue
  # The oracle's named row is every repro replay; see CI_FAST_SKIP_ORACLE_REPLAY above.
  if [ "$p" = ./dirac/oracle ] && [ "${CI_FAST_SKIP_ORACLE_REPLAY:-0}" = 1 ]; then echo "ci-fast: ./dirac/oracle replays skipped here; the queue batch's dirac-repros runs them"; continue; fi
  # ./dirac/oracle's named row is the whole case above's replacement when only
  # its tests or recordings changed: every repro replay and the census guards.
  # It gets the same 15m cap, not the generic 10m: #2159 timed out at 10m after
  # one existing replay (The World Tree) took 7m43 under box load.
  to=10m
  [ "$p" = ./dirac/oracle ] && to=15m
  go test -timeout "$to" -short -run "$r" "$p" || rc=1
done <<<"$list"
# Always, whatever the diff selects: nothing under .ds4/ (seat scratch) is tracked. A seat that force-adds a .ds4
# report passes its own targeted tests and only a batch shard catches it (11-23: #3835 reddened batches 4024 and
# 4074 with .ds4/report-cs1.md). The test reads git ls-files only, in milliseconds. It and the named rows above run
# in the foreground while the big calls run; the waits below reap them all.
go test -timeout 2m -short -run '^TestNothingUnderDS4IsTracked$' ./testkit/testutil || rc=1
for i in "${!pids[@]}"; do
  if wait "${pids[$i]}"; then echo "ci-fast: [${kinds[$i]}] passed"; else echo "ci-fast: [${kinds[$i]}] FAILED"; rc=1; fi
done
[ -n "$list" ] || echo "ci-fast: the diff selects no tests"
exit $rc
