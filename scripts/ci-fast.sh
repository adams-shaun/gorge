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
