# Rules Test Parallelism Design

## Goal

Reduce the wall time of the full `rules` test suite while retaining its exact
test coverage, skip behavior, deterministic engine behavior, and per-test
timing visibility through `gotestsum`.

The reference run used `GOMAXPROCS=10` and completed 1,420 tests with one
documented skip in 322.99 seconds. Four external test-process shards completed
the same tests in 123.70 seconds, but duplicated package-wide state and used an
estimated 2.39 GiB aggregate peak RSS.

## Design

### Shared corpus

`internal/testutil.CorpusRegistry` will load the repository corpus once per Go
test binary with `sync.Once`. The cached outcome will contain the registry,
resolved corpus path, missing-corpus state, and any load error. Every caller
will still perform its own `t.Skip` or `t.Fatalf`, so the once callback never
captures a `testing.TB` or calls `Goexit`.

`OpenCorpusRegistry` remains uncached. Commands and tests that explicitly ask
to open a particular corpus continue to receive a fresh registry.

The cached registry is immutable after publication. Tests that currently
modify a card or face obtained from the corpus will copy the definition before
changing it. An audit will cover direct slice, map, card, face, ability, static,
trigger, replacement, keyword, and token mutations.

### Shared census

The normal parameter census already uses `censusOnce`, so no replacement cache
is needed. Native parallel tests in one process will share `censusBase` and
`censusReads`. Tests only read that published result.

The deleted-consumer probe passes a non-nil `drop` map and will continue to run
an independent scan and census. Sharing that result would invalidate what the
test is designed to prove.

### Native test parallelism

Parallelism will remain inside one `rules` test process. Audited independent
top-level tests will opt in with `t.Parallel()`, and Go's `-parallel` setting
will cap concurrent test execution. The measurement command will use
`GOMAXPROCS=10` and `-parallel=10`.

The first set will prioritize corpus-heavy tests and the slowest independent
top-level tests. Tests that mutate package globals, process state, shared card
definitions, or intentionally coordinate through shared state will remain
serial until isolated. Parallel subtests already present in the invariant
suite will be retained.

No production engine behavior, event ordering, replay state, or card-pipeline
dependency changes are in scope.

## Testing and measurement

Implementation will proceed test-first for the corpus cache:

1. Add focused tests proving concurrent callers share one successful load and
   that a cached failure is returned consistently.
2. Run those tests before implementation and confirm the expected failure.
3. Implement the cache and rerun the focused tests.
4. Run impacted packages only: `internal/testutil` and `rules`.
5. Run the complete `rules` suite through `gotestsum` with
   `GOMAXPROCS=10`, `-parallel=10`, and per-test JSON output.
6. Compare test count, skip count, failures, wall time, CPU utilization, peak
   RSS, and per-`TestFooBarThing` runtimes with the 322.99-second reference.

Per the user's instruction, no race-test run is included.

## Safety constraints

- Never cache evaluated rules state, engine state, or mutable game state.
- Never mutate shared corpus definitions.
- Preserve clean-checkout missing-corpus skip semantics.
- Preserve the conformance lane's documented known-red behavior.
- Do not commit, push, rebase, merge, create a PR, or deploy without a new
  explicit request.
