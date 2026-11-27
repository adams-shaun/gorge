#!/usr/bin/env bash
# ci-fast.sh [--precompile] <base> [listfile]: the fast stage of a pull request's CI (Mergify two-step CI, operator
# 11-16). It runs what
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
#
# S4 (11-26): on a self-hosted runner ci.yml also runs this script in --precompile mode as the fast job's corpus
# step, before the tests. It applies the same selection and skip rules, but every selected test call is built with
# -exec /bin/true (compiles, runs nothing) while ci-corpus.sh — otherwise the fast job's last serial prefix (14-83 s
# on a tree the box has not built, 2-5 s warm) — runs beside it, so that step costs max(corpus, compile) instead of
# their sum. The compile reads only the checkout and the box's Go build cache, never .cards, which ci-corpus.sh is
# about to link. Go keys a cached test *result* on the -exec command (cmd/go/internal/test: "execcmd"), so the
# precompile cannot record a pass the real run would then skip; only the compiled binaries are reused. ci.yml runs
# the selection once and hands it through to both, so the compile and the run cannot select different packages.
set -uo pipefail
precompile=0
if [ "${1:-}" = --precompile ]; then
  precompile=1
  shift
fi
listfile=${2:-${CI_FAST_LIST:-}}
if [ -n "$listfile" ] && [ -e "$listfile" ]; then
  list=$(cat "$listfile") || { echo "ci-fast: cannot read the selection file $listfile"; exit 1; }
elif [ "$precompile" = 1 ]; then
  echo "ci-fast: --precompile needs a readable selection file"; exit 2
else
  list=$(BASE=$1 bash scripts/test-targets.sh list) || { echo "ci-fast: test-targets.sh list failed"; exit 1; }
fi
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
# gotest: the one place a selected test call is made. In --precompile mode it builds the same test binaries with
# -exec /bin/true and runs none of them — only the checkout and the box's Go build cache are read, never .cards, so
# it is safe beside ci-corpus.sh. Go's test result cache is keyed on the -exec command, so this records no result the
# real run below could mistake for a pass; it shares only the compiled binaries, which is the point. (The .ds4
# always-gate below keeps its literal `go test` line, pinned by scripts/ci_fast_ds4_guard_test.go; it reads only git
# ls-files, never .cards, so running it in --precompile mode too is safe and it also warms ./testkit/testutil.)
gotest() {
  if [ "$precompile" = 1 ]; then command go test -exec /bin/true "$@" >/dev/null; else command go test "$@"; fi
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
    bg oracle gotest -timeout 15m -short -run "^($rx)\$" ./dirac/oracle
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
    bg kgen gotest -timeout 20m -short -count=1 ./dirac/kgen
  fi
fi
if [ -n "$whole" ]; then
  # shellcheck disable=SC2086
  bg batch gotest -timeout 20m -short $whole
fi
# --precompile (ci.yml's corpus step): the corpus and dirac's tables for this tree run beside the builds above.
# Neither reads what the other writes — the builds never read .cards, ci-corpus.sh never reads a test binary — so
# they overlap exactly.
if [ "$precompile" = 1 ]; then
  bg corpus bash scripts/ci-corpus.sh
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
  gotest -timeout "$to" -short -run "$r" "$p" || rc=1
done <<<"$list"
# Derived records: freshness tests for generated files, which `test-targets.sh list` does not select — the diff
# changes an input (a manifest entry, a wire type, a decision kind), never the generated file itself. Three cases
# in ~3 h on 11-26: #5248 added a StatDefiler entry under dirac/manifest/ without regenerating
# dirac/kop/manifest_gen.go (fast and lint green, the queue's precheck red on TestManifestGenIsCurrent and
# TestKopOrdinalsUnchanged); #5315 added Decision.DivideTotal without regenerating docs/site/api/openapi.json/yaml
# and schemas/gorge-protocol.schema.json (the queue red on TestOpenAPISchemasMatchGo); #4896 was a derived-record
# rise that only pr-precheck's pr-derived.sh caught. Both tests are cheap (measured 11-26 on main:
# TestManifestGenIsCurrent 46 s, the four TestOpenAPI checks 12 s) and are diff-scoped: a web-only or oracle-only
# PR must not pay a ./dirac compile. A pathspec `dir/*` matches recursively (git's fnmatch has no PATHNAME flag).
# The diff is captured, not piped into grep -q: under pipefail, git diff's next write after grep exits at the first
# match gets SIGPIPE and the if reads false, silently skipping exactly on the large PRs (pr-precheck.sh 11-26).
# S4c (11-27): when the diff's selection already runs that package as a `whole` row, the batch call below runs the
# same test under the same -short, so the separate run is a duplicate — two copies of TestManifestGenIsCurrent
# (46 s each) competed on one box on run 45919671255: the batch's ./dirac finished at the same second as the derived
# run, 88 s in, for a test the batch was already running. Launch it only when the package is not already selected.
whole_pkg() { grep -qx "$1" <<<"$whole"; }
if [ -n "$(git diff --name-only "$1"...HEAD -- 'dirac/manifest/*' 'dirac/kop/*' 'dirac/cmd/kmanifest/*' 'dirac/kgen/*' 2>/dev/null)" ]; then
  if whole_pkg ./dirac; then
    echo "ci-fast: the batch's ./dirac run already includes TestManifestGenIsCurrent; not running it twice"
  else
    echo "ci-fast: the diff touches a manifest or kmanifest input; running TestManifestGenIsCurrent"
    bg derived-manifest gotest -timeout 5m -short -run '^TestManifestGenIsCurrent$' ./dirac
  fi
fi
if [ -n "$(git diff --name-only "$1"...HEAD -- 'server/*' 'engine/decision/*' 'docs/site/api/*' 2>/dev/null)" ]; then
  if whole_pkg ./server/host/httpapi; then
    echo "ci-fast: the batch's ./server/host/httpapi run already includes the OpenAPI freshness checks; not running them twice"
  else
    echo "ci-fast: the diff touches a wire type or decision kind; running the four OpenAPI freshness checks"
    bg derived-openapi gotest -timeout 5m -short -run '^TestOpenAPI(RoutesMatchSpec|SchemasMatchGo|YAMLIsRendered|ProtocolSchemaIsRendered)$' ./server/host/httpapi
  fi
fi
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
