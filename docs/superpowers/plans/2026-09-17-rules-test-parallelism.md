# Rules Test Parallelism Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce the full `rules` suite wall time by sharing immutable corpus/census work inside one test binary and running audited top-level tests concurrently.

**Architecture:** `internal/testutil.CorpusRegistry` publishes one immutable registry result through a `sync.Once` cache whose callback never receives a `testing.TB`. The existing `rules` census singleton remains unchanged, corpus-mutating fixtures copy their card before mutation, and selected high-cost top-level tests opt into Go's native `t.Parallel()` scheduler.

**Tech Stack:** Go 1.25.11, standard library `sync`/`sync/atomic`, `testing`, `gotestsum` JSON timing output.

**Spec:** `docs/superpowers/specs/2026-09-17-rules-test-parallelism-design.md`

## Global Constraints

- Use `GOMAXPROCS=10` and `-parallel=10` for final timing.
- Run only `internal/testutil` and `rules`; do not run race tests.
- Keep `OpenCorpusRegistry(dir)` uncached.
- Preserve clean-checkout missing-corpus skips and corpus-load failures.
- Keep the non-nil-`drop` census path independent.
- Never mutate a registry after it is published to concurrent tests.
- Do not commit, push, rebase, merge, create a PR, or deploy.

---

### Task 1: Process-wide corpus cache

**Files:**
- Modify: `internal/testutil/decks.go:162-213`
- Modify: `internal/testutil/corpusenv_test.go`

**Interfaces:**
- Consumes: `OpenCorpusRegistry(dir string) (*cards.Registry, error)`
- Produces: `corpusRegistryCache.get(func() corpusRegistryResult) corpusRegistryResult`
- Preserves: `CorpusRegistry(t testing.TB) *cards.Registry`

- [x] **Step 1: Write the failing cache tests**

Add tests in package `testutil` that create a local `corpusRegistryCache`. The
success test launches 16 goroutines behind a start channel; each calls `get`
with a loader that increments an `atomic.Int32` and returns the same
`cards.NewRegistry()` pointer. Assert that the loader count is exactly one and
every result contains that pointer. The failure test calls `get` twice with a
loader returning a sentinel error and asserts one loader call plus
`errors.Is(result.err, sentinel)` on both results.

```go
func TestCorpusRegistryCacheSharesConcurrentLoad(t *testing.T) {
	var cache corpusRegistryCache
	var calls atomic.Int32
	want := cards.NewRegistry()
	start := make(chan struct{})
	got := make(chan corpusRegistryResult, 16)
	for range 16 {
		go func() {
			<-start
			got <- cache.get(func() corpusRegistryResult {
				calls.Add(1)
				return corpusRegistryResult{reg: want}
			})
		}()
	}
	close(start)
	for range 16 {
		if result := <-got; result.reg != want {
			t.Fatalf("registry = %p, want %p", result.reg, want)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("loader calls = %d, want 1", got)
	}
}
```

- [x] **Step 2: Verify RED**

Run:

```sh
go test ./internal/testutil -run '^TestCorpusRegistryCache' -count=1
```

Expected: compilation fails because `corpusRegistryCache` and
`corpusRegistryResult` do not exist.

- [x] **Step 3: Implement the cache**

In `decks.go`, define:

```go
type corpusRegistryResult struct {
	reg     *cards.Registry
	dir     string
	missing bool
	err     error
}

type corpusRegistryCache struct {
	once   sync.Once
	result corpusRegistryResult
}

func (c *corpusRegistryCache) get(load func() corpusRegistryResult) corpusRegistryResult {
	c.once.Do(func() { c.result = load() })
	return c.result
}
```

Add one package-level cache. Move git-root resolution, `.cards` existence
checking, and `OpenCorpusRegistry` into a callback returning
`corpusRegistryResult`. After `get`, `CorpusRegistry` performs the caller's
`t.Skip` or `t.Fatalf`; the callback must not retain or invoke `t`.

- [x] **Step 4: Verify GREEN and existing locator behavior**

Run:

```sh
go test ./internal/testutil -run '^(TestCorpusRegistryCache|TestCorpusRegistryResolvesUnderAnExportedGitDir)$' -count=1
```

Expected: PASS with one successful corpus load in the process.

---

### Task 2: Enforce immutable shared corpus use

**Files:**
- Modify: `rules/sacrifice_unless_pay_test.go:345-360`
- Audit: `rules/*_test.go`

**Interfaces:**
- Consumes: the shared `*cards.Registry` returned by `testutil.CorpusRegistry`
- Produces: no new production API; the Vexing Devil fixture owns its copied card and face slices

- [x] **Step 1: Run a mutation audit**

Search assignments and appends involving corpus-returned cards, faces,
abilities, statics, triggers, replacements, SVars, params, keywords, types,
registry cards, and token maps. Trace each candidate to determine whether the
card came from `CorpusRegistry` or from the local `card(t, script)` parser.

```sh
rg -n --pcre2 '(Faces|Abilities|Statics|Triggers|Repls|SVars|Params|Keywords|Types|Cards|Tokens).*\s(=|:=)|append\(' rules --glob '*_test.go'
```

The known shared-corpus mutation is
`TestEngineVexingDevilLifelinkSourceGainsLife`; inline-card mutations such as
Quicksilver Elemental's gained ability remain isolated.

- [x] **Step 2: Copy the Vexing Devil definition before mutation**

Copy the card, its `Faces` slice, the first face, and that face's `Keywords`
slice before appending `Lifelink`, then pass the copied card to `handEngine`.

```go
original := mustCorpusCard(t, reg, "Vexing Devil")
src := *original
src.Faces = append([]*cards.Face(nil), original.Faces...)
face := *original.Faces[0]
face.Keywords = append(append([]string(nil), original.Faces[0].Keywords...), "Lifelink")
src.Faces[0] = &face
e := handEngine(t, &src)
```

- [x] **Step 3: Verify the isolated fixture**

Run:

```sh
go test ./rules -run '^TestEngineVexingDevilLifelinkSourceGainsLife$' -count=2
```

Expected: PASS twice without changing the registry-owned face.

---

### Task 3: Add native parallel execution to measured high-cost tests

**Files:**
- Modify: `rules/fuzz_test.go`
- Modify: `rules/cr_combat_conformance_test.go`
- Modify: `rules/life_draw_trigger_test.go`
- Modify: `rules/altcast_test.go`
- Modify: `rules/cr_multiplayer_conformance_test.go`
- Modify: `rules/heads_test.go`
- Modify: `rules/acceptance_test.go`
- Modify: `rules/commander_decks_test.go`
- Modify: `rules/cr601_conformance_test.go`
- Modify: `rules/paramcensus_test.go`
- Modify: `rules/imprinted_payer_test.go`
- Modify: `rules/alternative_costs_test.go`
- Modify: `rules/rakdos_params_reflected_test.go`
- Modify: `rules/rakdos_params_energycost_test.go`
- Modify: `rules/phase_trigger_names_test.go`
- Modify: `rules/target_legality_test.go`
- Modify: `rules/priority_phase_test.go`

**Interfaces:**
- Consumes: Go test's native top-level parallel scheduler and the shared corpus/census caches
- Produces: audited tests that call `t.Parallel()` before fixture construction

- [x] **Step 1: Add `t.Parallel()` to the measured tests**

Add `t.Parallel()` as the first statement in these top-level tests:

```text
TestInvariantsUnderSeedFuzz
TestCR508CorpusRequirementsUnderAttackRestriction
TestArchiveAndClericClassControllerChoosesOrder
TestTaintedRemedyAndArchiveGainingPlayerChoosesOrder
TestAlternateAdditionalCostSpecialPayments
TestCR802BlockDeclarationsFollowAPNAP
TestHeads
TestRepoDecksPlayAtEverySeatCount
TestRepoDeckGamesReplayExactly
TestRepoCommanderDecksPlayAndCastTheirCommander
TestLifeLostAllObNixilisQueuesOnceForDamageAllPlayers
TestCR601NoMandatoryCounterCastOnEmptyStack
TestParamCensusDetectsADeletedConsumer
TestImprintedControllerHeroism
TestEveryRepoDeckParamsAreRead
TestSuspendOrdinaryExiledCardNeverGetsTheOffer
TestCorruptedGrafstoneReflectsGraveyardColours
TestOpeningHandRevealActionAndPlayFirstGate
TestWhirlerVirtuosoFixedEnergyCostGatesAndPays
TestLoseLifeAllBatchesOpponentsForObNixilis
TestCR800DepartedControllersStackObjectsCease
TestPhaseGateAppliesToChangesZoneAndSpellCast
TestNoTargetDecisionOffersAnIllegalTarget
TestTransmuteAndCyclingRealHandActivations
TestTestBotOnlyActivatesInAMainPhase
```

Do not add parallel markers to tests using `t.Setenv`, process-global state,
or shared definition mutation. Retain the existing parallel seed subtests.

- [x] **Step 2: Format and run the selected set repeatedly**

Run `gofmt` on the modified Go files, then:

```sh
GOMAXPROCS=10 go test ./rules -parallel=10 -run '^(TestInvariantsUnderSeedFuzz|TestCR508CorpusRequirementsUnderAttackRestriction|TestArchiveAndClericClassControllerChoosesOrder|TestTaintedRemedyAndArchiveGainingPlayerChoosesOrder|TestAlternateAdditionalCostSpecialPayments|TestCR802BlockDeclarationsFollowAPNAP|TestHeads|TestRepoDecksPlayAtEverySeatCount|TestRepoDeckGamesReplayExactly|TestRepoCommanderDecksPlayAndCastTheirCommander|TestLifeLostAllObNixilisQueuesOnceForDamageAllPlayers|TestCR601NoMandatoryCounterCastOnEmptyStack|TestParamCensusDetectsADeletedConsumer|TestImprintedControllerHeroism|TestEveryRepoDeckParamsAreRead|TestSuspendOrdinaryExiledCardNeverGetsTheOffer|TestCorruptedGrafstoneReflectsGraveyardColours|TestOpeningHandRevealActionAndPlayFirstGate|TestWhirlerVirtuosoFixedEnergyCostGatesAndPays|TestLoseLifeAllBatchesOpponentsForObNixilis|TestCR800DepartedControllersStackObjectsCease|TestPhaseGateAppliesToChangesZoneAndSpellCast|TestNoTargetDecisionOffersAnIllegalTarget|TestTransmuteAndCyclingRealHandActivations|TestTestBotOnlyActivatesInAMainPhase)$' -count=3
```

Expected: all selected tests pass in all three repetitions; the documented
conformance skip remains a skip when its environment flag is absent.

---

### Task 4: Impacted-package verification and timing report

**Files:**
- Inspect: all changed files
- Write timing artifacts only under `/tmp`

**Interfaces:**
- Consumes: `gotestsum --jsonfile`, `/usr/bin/time -v`
- Produces: per-top-level-test TSV and aggregate wall/CPU/RSS comparison

- [x] **Step 1: Verify source hygiene**

Run:

```sh
git diff --check
go test ./internal/testutil -count=1
```

Expected: both commands exit zero.

- [x] **Step 2: Run the complete impacted rules package**

Run one `gotestsum` process with `GOMAXPROCS=10`, `-parallel=10`, and
`-count=1`, writing stdout, JSON, status, and `/usr/bin/time -v` output to
`/tmp/gorge-rules-native-parallel-20260917.{out,json,status,time}`.

```sh
GOMAXPROCS=10 /usr/bin/time -v gotestsum \
  --jsonfile /tmp/gorge-rules-native-parallel-20260917.json \
  --format testname -- \
  ./rules -parallel=10 -count=1
```

Expected: 1,420 tests pass, one documented skip, exit zero.

- [x] **Step 3: Produce exact per-test timing output**

Parse the JSON events whose action is `pass`, `fail`, or `skip`, whose package
is `github.com/adams-shaun/gorge/rules`, and whose test name contains no `/`.
Sort descending by elapsed seconds and write:

```text
/tmp/gorge-rules-native-parallel-top-level-runtimes-20260917.tsv
```

The columns are elapsed seconds, action, and exact `TestFooBarThing` name.

- [x] **Step 4: Report the comparison**

Report exit status, passed/failed/skipped test counts, wall time, user/system
CPU, aggregate CPU utilization, peak RSS, and the slowest top-level tests.
Compare wall time and memory with both the 322.99-second single-process
reference and the 123.70-second four-process experiment. State that no race
test was run.
