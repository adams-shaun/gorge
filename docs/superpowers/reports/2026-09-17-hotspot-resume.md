# Fresh-session resume prompt: continue hotspot optimization

Paste the following into a fresh session with `/tmp/gorge` as the workspace.

---

Continue profiling-led performance work on gorge, the pure-Go Magic rules
engine. The long-term motivation is bot self-play and eventual MCTS/I-MCTS
throughput.

Work in small, evidence-backed iterations:

1. Establish a fresh baseline on the actual checkout.
2. Identify one measured hotspot.
3. Present evidence, alternatives, a recommended design, and verification.
4. Wait for my approval before implementing that optimization.
5. Implement test-first, verify correctness, and remeasure.
6. Repeat for the next hotspot until I stop the session.

Do not commit, push, force-push, rebase, merge, create a PR, or deploy without
a new explicit request. Preserve user changes and keep unrelated correctness
fixes separate.

## Read first

1. Read `AGENTS.md` and all applicable skills.
2. Inspect status, branch, HEAD, upstream, origin, and merge-base. Do not assume
   the checkpoint below is still current.
3. Read:
   - `docs/superpowers/reports/2026-09-17-hotspot-optimization.md`
   - `docs/superpowers/reports/2026-09-17-trigger-pruning.md`
   - `docs/superpowers/specs/2026-09-17-rules-test-parallelism-design.md`
   - `docs/superpowers/plans/2026-09-17-rules-test-parallelism.md`
4. Treat this document as a handoff, not proof of current state. Revalidate
   anything relied upon.

## Published checkpoint

Verified 2026-09-17:

- Workspace: `/tmp/gorge`
- Branch: `perf/hotspot-optimization-2026-09-17`
- HEAD and remote branch:
  `79b2121bb59d6d33995b45c0bef0825615590a1a`
- Rebased onto `origin/main`:
  `8bce166943e3d09cce81acad4797a6106630ad5c`
- Origin: `git@github.com:adams-shaun/gorge.git`
- Corpus cache SHA-256:
  `f6f87777b35f7270ac8caafe8c34fbe9cffd3055f1efaa16256aa77cdf636477`
- Local ordinary-test toolchain: Go 1.25.11.
- Open PR: https://github.com/adams-shaun/gorge/pull/1
- No merge or deployment was performed.
- This refreshed resume document is intentionally generated after the push
  and may be the only uncommitted file.

Commits above `origin/main`:

```text
a617c4b perf: remove rules hot-path allocation overhead
6d8a49f docs: record hotspot sync verification and resume handoff
abe0f71 perf: prune impossible printed-trigger scans
91afbcc test: adapt hotspot parity check to upstream botbench
db84d5c perf: reuse cost statics during legal action scans
79b2121 test: share corpus and parallelize rules suite
```

The feature branch was rebased before the latest implementation. Publishing
therefore used `git push --force-with-lease`, after fetching the remote tip,
with strict host verification against GitHub's published SSH keys in
`/tmp/gorge-push-check.Y39KB6/github_known_hosts`. Do not assume that
temporary file still exists, and never disable host verification.

## Completed engine optimizations

### Batch 1: allocation cleanup

- Reject impossible granted-Ward events/targets before expensive discovery.
- Return trigger/replacement faces without per-object singleton allocations.
- Iterate filter alternatives and predicates lazily.
- Reuse immutable normalizers in `ParseCost` and `ProducedCounts`.
- Cache pure Phase syntax separately from replay-visible diagnostics.
- Use the reusable BoardSeat adapter in botbench while preserving View
  fallback and full-game parity.

See `2026-09-17-hotspot-optimization.md` for measurements.

### Batch 2: trigger eligibility pruning

- Cache immutable face/event eligibility on each engine.
- Reject impossible printed trigger kinds before dynamic gates.
- Preserve the deterministic all-zone source walk, Phase diagnostics,
  live/look-back matching, LKI, batching, state-trigger latches, APNAP order,
  granted Ward checks, and clone ownership.
- This is not an object index and stores no evaluated match or mutable
  game-state membership.

See `2026-09-17-trigger-pruning.md` for measurements.

### Batch 3: call-scoped cost-static membership

Commit: `db84d5c`.

- `legalActions` lazily collects RaiseCost, ReduceCost, and SetCost static
  membership once, in one deterministic zone walk.
- The snapshot is reused only within that `legalActions` call.
- It is not stored on `Engine`, so later legality/payment checks and fixture
  mutations observe current state.
- Applicability, conditions, targets, X, amounts, commander tax, payment, and
  final costs are still recomputed per candidate.
- Direct callers outside `legalActions` retain fresh collection.
- `TestLegalActionsReusesCostStaticMembership` guards allocation behavior.

Measured cost-static collection CPU share:

| workload | before | after |
|---|---:|---:|
| duel | 9.99% | 1.21% |
| four seats | 9.33% | 1.55% |
| aggro | 3.63% | 0.47% |

All 450 profiled games matched baseline game summaries, final heads, and event
counts.

## Test-suite acceleration

Commit: `79b2121`.

- `internal/testutil.CorpusRegistry` now loads once per test binary.
- The cache callback does not retain a `testing.TB`; every caller still
  performs its own skip/fatal handling.
- `OpenCorpusRegistry(dir)` remains uncached.
- The shared registry is immutable. The one rules test that modified a corpus
  face, Vexing Devil's added Lifelink, now copies the card, face, and keyword
  slice first.
- The normal parameter census already used `censusOnce`; native parallel
  tests share it. The non-nil-`drop` deleted-consumer probe still recomputes
  independently by design.
- Twenty-five measured high-cost top-level rules tests now call
  `t.Parallel()`. Go's `-parallel` limit controls concurrency.

Full `rules` suite, `gotestsum`, `GOMAXPROCS=10`, `-parallel=10`:

| metric | previous one-process | native parallel/shared |
|---|---:|---:|
| gotestsum tests | 1,420 pass, 1 skip | 1,420 pass, 1 skip |
| measured wall | 322.99 s | 22.63 s |
| speedup | 1.00x | 14.27x |
| user CPU | 597.79 s | 146.83 s |
| system CPU | 20.49 s | 2.36 s |
| CPU utilization | 191% | 659% |
| peak RSS | 849 MiB | 500 MiB |
| sum of top-level elapsed | 320.89 s | 32.50 s |

The earlier four-process shard experiment took 123.70 s and had a roughly
2.39 GiB aggregate peak-RSS upper bound. The native/shared run is 5.47x faster
in wall time and avoids per-process corpus/census duplication.

Slowest top-level tests in the new run:

```text
15.37s TestInvariantsUnderSeedFuzz
 2.20s TestHeads
 2.15s TestRepoDecksPlayAtEverySeatCount
 1.97s TestRepoCommanderDecksPlayAndCastTheirCommander
 1.59s TestTestBotOnlyActivatesInAMainPhase
 1.54s TestRepoDeckGamesReplayExactly
 1.40s TestCR601NoMandatoryCounterCastOnEmptyStack
 1.30s TestBotMatchIsDeterministicAcrossRuns
 1.29s TestEveryRepoDeckIsFullySupported
 1.21s TestNoTargetDecisionOffersAnIllegalTarget
```

Artifacts:

```text
/tmp/gorge-rules-gotestsum-20260917.{out,json,time,status}
/tmp/gorge-rules-top-level-runtimes-20260917.tsv
/tmp/gorge-rules-subtest-runtimes-20260917.tsv
/tmp/gorge-rules-sharded.37kaJ9/
/tmp/gorge-rules-native-parallel-20260917.{out,json,time,status}
/tmp/gorge-rules-native-parallel-top-level-runtimes-20260917.tsv
```

Temporary artifacts may disappear. Recreate them rather than treating absence
as a blocker.

Verification performed after the changes:

- Selected 25 parallel tests, `GOMAXPROCS=10 -parallel=10 -count=3`:
  PASS in 47.462 s.
- Full rules gotestsum run: 1,420 pass, one documented skip, exit 0.
- Direct `CorpusRegistry` consumer packages passed:
  `internal/testutil`, `botpolicy`, `cmd/botbench`, `cmd/gorged`,
  `cmd/repro`, `effects`, `host`, `host/httpapi`, `seat`, and `view`.
- `git diff --check`: PASS.
- Per explicit user instruction, no race tests were run.

## Next performance iteration

Do not assume the next target from old percentages. Rebuild and profile the
current HEAD first. Batch 3 materially changed the legal-action profile, so
the pre-batch ranking is only a lead:

| pre-batch path | duel | four seats | aggro |
|---|---:|---:|---:|
| legal actions | 41.11% | 33.13% | 30.95% |
| trigger scan | 17.46% | 25.88% | 12.74% |
| cost-static collection | 10.87% | 10.40% | 3.47% |
| Board projection | 10.79% | 11.12% | 23.05% |

Cost-static collection is now about 0.5-1.6%, so likely candidates are:

- the largest remaining subpath inside legal-action generation;
- Board projection, especially the aggro workload;
- remaining `staticEffects` allocations;
- a newly exposed hotspot from fresh profiles.

Inspect callers and ownership before proposing a cache. Prefer call-scoped
reuse or elimination of repeated work over durable engine caches. Never cache
evaluated applicability, targets, X, conditions, costs, derived mutable state,
or event-visible ordering.

The cloned-log append cost remains a separate architectural candidate:
turn-11 sampling previously measured about 385,416 allocated bytes for clone
plus append versus 55,224 bytes for `Engine.Clone` alone. A persistent-prefix
log requires its own design approval and exhaustive history-reader, hash,
clone, replay, and branch-independence tests. Do not mix it into a smaller
hotspot iteration.

## Measurement discipline

- Build a fresh baseline from the current checkout before editing.
- Record revision, corpus checksum, Go version, workload, affinity, and
  environment.
- Use the same toolchain for both sides of any comparison. Historical engine
  probes used Go 1.26.4; ordinary verification currently uses Go 1.25.11.
- Run benchmark workers one at a time with `GOMAXPROCS=1`,
  `GOMEMLIMIT=5GiB`, fixed decks/seeds, unchanged bot policy, and unchanged
  observation adapter.
- Check whether CPU 15 affinity is available before reusing it.
- Exclude corpus load and one full warm-up game; include new-engine
  construction and cache population.
- Historical workloads:
  - duel: death-n-taxes / dimir-tempo, 250 games;
  - four seats: those plus eldrazi-stompy / mono-black-aggro, 50 games;
  - aggro: mono-red-goblins / mono-green-stompy, 150 games.
- Collect CPU/heap profiles separately from alternating unprofiled timing and
  allocation pairs. Reverse pair order across repetitions.
- Compare every game summary, final chain head, and event-kind count.
- Treat shared-host elapsed ratios as local evidence, not portable speedups.
- Do not run correctness tests alongside profiling workers.

## Correctness constraints

- All game-state mutation goes through `events.Apply`.
- Preserve event ordinals, order, hashes, replay behavior, LKI, APNAP, hidden
  information, deterministic map handling, and clone ownership.
- No cgo or third-party dependency in the rules/card core.
- Never commit Forge scripts or token scripts.
- The conformance lane remains intentionally known-red; follow `AGENTS.md`
  if any leaf unexpectedly turns green.
- Run impacted packages only unless broader verification is requested.
- Do not run race tests unless the user changes the current instruction.
- Use `gotestsum` for full rules-suite timing and retain exact
  `TestFooBarThing` metrics.

Start by reporting the verified git state and fresh profile evidence. Then
recommend exactly one next optimization with trade-offs and a verification
plan, and wait for approval before editing.
