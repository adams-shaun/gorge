#!/usr/bin/env bash
# ci-test-shard.sh <shard> <shards>: run one shard of CI's test suite from the repository root (ci.yml, job test, which
# runs this through scripts/ci-infra-retry.sh). GITHUB_EVENT_NAME comes from the runner.
#
# The nightly run takes every package, experimental ones included.
# The nightly also holds dirac/docs/support.md and dirac/kgen's pins byte for byte; a PR or queue batch fails
# only on a fall. Exact pins in a batch failed PRs each green alone: two +1s pinned 24642, the batch 24643 (11-17).
# KGEN_NIGHTLY=1 runs dirac/kgen's single-P table build test, which costs a full corpus build.
#
# No set -e: the test calls collect their exit codes in rc.
set -uo pipefail
if [ "$GITHUB_EVENT_NAME" = schedule ]; then export CI_FULL=1 DIRAC_SUPPORT_STRICT=1 DIRAC_PIN_STRICT=1 KGEN_NIGHTLY=1; fi
# A failed deal must fail the shard: without set -e it would leave pkgs empty, which reads as an empty (green) shard.
pkgs=$(bash scripts/ci_shard.sh $1 "$2") || exit
echo "$pkgs" | wc -l | xargs echo "packages:"
count= tags=
# The SpellBench and ManaBrew interface tests (build tags) run nightly only.
if [ "$GITHUB_EVENT_NAME" = schedule ]; then count=-count=1; tags="-tags=spellbench,manabrew"; fi
if [ -z "$pkgs" ]; then echo "shard $1 is empty"; exit 0; fi
# On a merge-queue batch (ci.yml sets QUARANTINE) every call leaves out the tests listed in scripts/flaky.tsv, and the
# job's next step runs them where they cannot fail it. Elsewhere q is empty and they run, and gate, here.
q=
if [ -n "${QUARANTINE:-}" ]; then q=$(bash scripts/ci-flaky.sh skip-regex) || exit; fi
# dirac/kgen runs uncached (-count=1): its tests open the corpus ~88 times, so go spent 190-300 s hashing those
# files for a test-result cache that never hit in a batch (0 of 6), against ~112 s of tests (11-17).
# 11-20: kgen runs beside the shard's other packages, not after them. Serial, it added its ~224 s (median of
# the 11-20 batches) to shard 0's ~200 s of other packages, and shard 0 was the slowest shard; together the
# shard is bound by its single largest package instead. Its output goes to a file and is printed after.
# The packages in scripts/ci-splits.tsv run as one entry per row (ci_shard.sh): the package, minus every
# row's regex, and <package>#<label>, the row's regex (label defaults to A; 11-20 engine/rules, 11-21
# testkit/conformance, 11-27 dirac/kgen, bots/azmcts and engine/effects, engine/rules as three rows). The
# balancer places each; this step runs whichever it holds as a call of its own, beside the rest. A package's
# whole-minus-regex call runs once per package (on its first row), not once per row. kgen keeps its own
# branch (both halves uncached) and the loop below skips it, so it runs once; every split call logs to a
# file printed after.
kgen=github.com/adams-shaun/gorge/dirac/kgen
pids=() logs=() rc=0 ex=(-e "$kgen" -e "$kgen#A")
solo() { local log=$1; shift; go test "$@" >"$log" 2>&1 & pids+=($!); logs+=("$log"); }
# No pipe: under pipefail, a writer cut off by grep -q's early exit reads as "not held", and that call would not run.
has() { grep -qx "$1" <<<"$pkgs"; }
kgen_rx=$(awk -F'\t' -v k="$kgen" '$1 == k { print $2 }' scripts/ci-splits.tsv)
if has "$kgen"; then
  if [ -n "$kgen_rx" ]; then solo kgen-test.log -count=1 $tags -timeout 20m -skip "^(TestHeads${q:+|$q})\$|$kgen_rx" "$kgen"
  else solo kgen-test.log -count=1 $tags -timeout 20m -skip "^(TestHeads${q:+|$q})\$" "$kgen"; fi
fi
if [ -n "$kgen_rx" ] && has "$kgen#A"; then solo kgen-a-test.log -count=1 $tags -timeout 20m -run "$kgen_rx" ${q:+-skip "^($q)\$"} "$kgen"; fi
# The skip for a package's whole call is the union of all its rows' regexes, so every top-level test runs
# in exactly one call: the first matching #entry, or the whole call.
declare -A whole_rx=()
while IFS=$'\t' read -r sp rx _; do
  case $sp in '' | '#'*) continue ;; esac
  whole_rx[$sp]="${whole_rx[$sp]:-}${whole_rx[$sp]:+|}$rx"
done <scripts/ci-splits.tsv
declare -A ran_whole=()
while IFS=$'\t' read -r sp rx label; do
  case $sp in '' | '#'*|"$kgen") continue ;; esac
  label=${label:-A}
  ex+=(-e "$sp" -e "$sp#$label"); n=${sp##*/}
  if [ -z "${ran_whole[$sp]:-}" ]; then
    ran_whole[$sp]=1
    if has "$sp"; then solo "$n-test.log" $count $tags -timeout 20m -skip "^(TestHeads${q:+|$q})\$|${whole_rx[$sp]}" "$sp"; fi
  fi
  if has "$sp#$label"; then solo "$n-$label-test.log" $count $tags -timeout 20m -run "$rx" ${q:+-skip "^($q)\$"} "$sp"; fi
done <scripts/ci-splits.tsv
rest=$(printf '%s\n' $pkgs | grep -vx "${ex[@]}" || true)
if [ -n "$rest" ]; then go test $count $tags -timeout 20m -skip "^(TestHeads${q:+|$q})\$" $rest || rc=1; fi
for i in "${!pids[@]}"; do wait "${pids[$i]}" || rc=1; cat "${logs[$i]}"; done
exit $rc
