#!/usr/bin/env bash
# ci-fast.sh <base>: the fast stage of a pull request's CI (Mergify two-step CI, operator 11-16). It runs what
# `test-targets.sh list` selects for the diff against <base>: the packages the change touches, every package whose
# tests import them, and the named test groups. The full suite runs once per Mergify batch, on mergify/merge-queue/*
# (ci.yml), not on every pull request.
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
    go test -timeout 15m -run "^($rx)\$" ./dirac/oracle || rc=1
  fi
fi
if [ -n "$whole" ]; then
  # shellcheck disable=SC2086
  go test -timeout 20m $whole || rc=1
fi
while IFS=$'\t' read -r m p r; do
  [ "$m" = named ] || continue
  go test -timeout 10m -run "$r" "$p" || rc=1
done <<<"$list"
[ -n "$list" ] || echo "ci-fast: the diff selects no tests"
exit $rc
