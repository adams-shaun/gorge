#!/usr/bin/env bash
# ci-fast.sh <base>: the fast stage of a pull request's CI (Mergify two-step CI, operator 11-16). It runs what
# `test-targets.sh list` selects for the diff against <base>: the packages the change touches, every package whose
# tests import them, and the named test groups. The full suite runs once per Mergify batch, on mergify/merge-queue/*
# (ci.yml), not on every pull request. Every run here passes -short, so the slow whole-game self-play/search tests
# skip it; the Mergify batch runs the suite without -short and runs them (they declare it with testing.Short).
set -uo pipefail
list=$(BASE=$1 bash scripts/test-targets.sh list) || { echo "ci-fast: test-targets.sh list failed"; exit 1; }
printf '%s\n' "$list"
whole=$(awk -F'\t' '$1 == "whole" {print $2}' <<<"$list")
rc=0
# ./dirac/oracle whole is the full lockstep suite (over 20 min: 11-17 #443 timed out); the oracle job runs it nightly
# and the batch's dirac jobs cover it. Here it runs only the tests in its changed test files and, when the diff adds
# or edits a recording, every repro replay (the recording's pool test is not named by the diff).
if grep -qx './dirac/oracle' <<<"$whole"; then
  whole=$(grep -vx './dirac/oracle' <<<"$whole" || true)
  changed=$(git diff --name-only "$1"...HEAD -- 'dirac/oracle/*_test.go' 2>/dev/null)
  rx=$(for f in $changed; do [ -f "$f" ] && sed -n 's/^func \(Test[A-Za-z0-9_]*\)(t \*testing\.T).*/\1/p' "$f"; done | sort -u | paste -sd'|')
  if git diff --name-only "$1"...HEAD -- dirac/oracle/testdata | grep -q .; then rx=${rx:+$rx|}'Test[A-Za-z0-9]*Repros?[A-Za-z0-9]*Lockstep'; fi
  if [ -n "$rx" ]; then
    echo "ci-fast: ./dirac/oracle runs -run '^($rx)\$' only"
    go test -timeout 15m -short -run "^($rx)\$" ./dirac/oracle || rc=1
  fi
fi
# ./dirac/kgen runs uncached (-count=1): its tests open the corpus ~88 times, and go spent 190-300 s hashing those
# files for a test-result cache that a changed tree never hits, against ~110 s of tests (11-17; ci.yml's shards do the same).
if grep -qx './dirac/kgen' <<<"$whole"; then
  whole=$(grep -vx './dirac/kgen' <<<"$whole" || true)
  go test -timeout 20m -short -count=1 ./dirac/kgen || rc=1
fi
if [ -n "$whole" ]; then
  # shellcheck disable=SC2086
  go test -timeout 20m -short $whole || rc=1
fi
while IFS=$'\t' read -r m p r; do
  [ "$m" = named ] || continue
  # ./dirac/oracle's named row is the whole case above's replacement when only
  # its tests or recordings changed: every repro replay and the census guards.
  # It gets the same 15m cap, not the generic 10m: #2159 timed out at 10m after
  # one existing replay (The World Tree) took 7m43 under box load.
  to=10m
  [ "$p" = ./dirac/oracle ] && to=15m
  go test -timeout "$to" -short -run "$r" "$p" || rc=1
done <<<"$list"
[ -n "$list" ] || echo "ci-fast: the diff selects no tests"
exit $rc
