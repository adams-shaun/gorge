#!/usr/bin/env bash
# pr-dirac-invariant.sh <base>: a pull request's dirac walk/cache invariant gate (class B, 11-28 census).
#
# docs/agents/red-batches-20261128.md classes the red merge batches; class B is a `perf(dirac)` pull request that
# relaxes a read-only invariant the heavy shards assert, so the queue is the first to see it and a whole batch is
# re-run. Two invariants define the class:
#
#   TestQuietProofEDH                 cast_quietseat_test.go: a proof walk must not change the State
#                                     ("the walk changed the State"), run beside dirac/mana_window.go's dry walk;
#   TestTyCacheExcludesNonObjectSources static_typecache_test.go: the derived-types cache must exclude a source whose
#                                     static reads more than objects ("the table is not cached with Duplicant excluded").
#
# Both are asserted only by the merge batch's dirac-slices / dirac test shards. This script runs them in the `fast`
# job when the diff touches a path that can break either: the proof and its walk, the caches, the State and prep they
# read, the payment/filter/keyword/kernel machinery the walk drives, and the edh slice pods the tests play. Any other
# diff exits before it runs a test, so a pull request that cannot affect the invariants pays one git diff.
#
# It adds a check; it drops, trims or skips nothing, and it runs through the same capped make target the rest of the
# repo uses (scripts/test-targets.sh one: 4 GB / 4 vCPU / the package's own timeout, no -count). Exit 0 when the diff
# is out of scope or the tests pass; make test-one's own exit (1 FAIL, 3 ENV, 4 EMPTY, 5 BUDGET, 75 BUSY) otherwise.
set -uo pipefail

base=${1:?usage: pr-dirac-invariant.sh <base>}
top=$(git rev-parse --show-toplevel) || exit 2
cd "$top" || exit 2
mb=$(git merge-base "$base" HEAD) || {
	echo "pr-dirac-invariant: cannot find the merge base with ${base:0:10}; fetch the base (checkout fetch-depth: 0)" >&2
	exit 2
}

# The paths a change to which can break one of the two invariants. `mana_window.go` is the walk the quiet proof
# verifies and `cast_quietseat.go` the proof (the carriers of red batches 46156810528 and 46154479584); `prep.go`,
# `engine.go` and `gen.go` hold the quiet facts and the prepared tables (46154479584's earlier carrier #5986 touched
# prep and engine);
# `state*.go` is the State a proof walk must not write and the owner of the type cache; the two `static_*cache*.go`
# files are the caches; `mana_*.go`, `filter_*.go`, `kw_*.go` and `kop/` are the payment walk, its filters, and the
# keyword/static table it and the type cache read; `oracle/testdata/slices` holds the edh pods both tests play.
paths=(
	'dirac/mana_window.go'
	'dirac/cast_quietseat.go'
	'dirac/prep.go'
	'dirac/engine.go'
	'dirac/gen.go'
	'dirac/state*.go'
	'dirac/static_typecache.go'
	'dirac/static_grantcache.go'
	'dirac/mana_*.go'
	'dirac/filter_*.go'
	'dirac/kw_*.go'
	'dirac/kop'
	'dirac/oracle/testdata/slices'
)
changed=$(git diff --name-only "$mb" -- "${paths[@]}")
if [ -z "$changed" ]; then
	echo "dirac walk/cache invariants: the diff touches none of the walk, cache, State or slice paths"
	exit 0
fi
echo "dirac walk/cache invariants: the diff touches:"
# shellcheck disable=SC2086 # one changed path per line
printf '  %s\n' $changed
exec make test-one PKG=./dirac RUN='^(TestQuietProofEDH|TestTyCacheExcludesNonObjectSources)$'
