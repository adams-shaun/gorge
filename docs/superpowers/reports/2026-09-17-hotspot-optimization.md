# Hotspot optimization: first measured batch

Date: 2026-09-17. Baseline: `eb51fff`. Forge corpus pin:
`95f04e8a04c8925fa97cb226fc3341cabcc90a53`.

Performance figures describe the first batch before the later origin sync.
The synchronization checkpoint below records the final branch and validation;
upstream subsequently changed game behavior, so fresh profiling is required
before treating these percentages or recorded outcomes as current.

## Outcome

The first batch removes unnecessary granted-Ward discovery, per-object face
slice allocations, repeated filter tokenization, repeated Phase parsing and
two repeatedly constructed mana normalizers. Botbench also uses the existing
reusable Board adapter when its seat supports it, retaining the View fallback.
No policy, game-state mutation, event kind, or replay golden was changed.

The strongest measurement is allocation reduction, not elapsed time. This is
a busy shared machine; CPU affinity does not isolate it from other workloads.

| Warm direct-Board workload | Games/batch | Before MB/game | After MB/game | Fewer allocated bytes | Fewer allocation objects |
|---|---:|---:|---:|---:|---:|
| Death-n-Taxes / Dimir Tempo | 50 | 26.13 | 4.56 | 82.5% | 95.6% |
| Four Legacy decks | 10 | 123.70 | 13.60 | 89.0% | 97.5% |
| Goblins / Green Stompy | 30 | 64.17 | 5.91 | 90.8% | 97.8% |

MB means decimal megabytes allocated, **not retained heap**. Values are medians
over six runs per version and workload, organized as two series of three
pairs. Each series reversed run order in its middle pair. Allocation counts
per game fell from approximately 1,342,233 to 58,578; 6,995,831 to 174,155;
and 2,636,397 to 57,348, respectively.

All 12 batches for each workload matched per-game seeds, intent/event counts,
turns, winners, draw flags and final chain heads, as well as aggregate event-kind
counts. These comparisons cover the same 90 distinct games repeatedly, not
1,080 independent seeds. They compare recorded summaries and hashes, not full
stored event arrays. Separately, `make sim` verified 20/20 event replays.

## Method and timing caveats

The diagnostic probe was built with Go 1.26.4, linux/amd64, and run with
`GOMAXPROCS=1`, `GOMEMLIMIT=5GiB`, pinned to CPU 15 for repeated comparisons.
The corpus and a complete warm-up game are loaded before timing. New-game
construction remains inside the measured batch. Both versions use the same
existing rules, bot, and direct-Board adapter. Exact allocation counters come
from `runtime.MemStats` deltas; pprof allocation attribution is sampled.

Decks and seeds:

- Duel: `death-n-taxes,dimir-tempo`, seeds 0–49.
- Four seats: `death-n-taxes,dimir-tempo,eldrazi-stompy,mono-black-aggro`, seeds 0–9.
- Aggro: `mono-red-goblins,mono-green-stompy`, seeds 0–29.

These are constructed games without mulligans, not Commander measurements.

| Workload | First series, paired baseline/final elapsed ratios | Second series, paired ratios |
|---|---|---|
| Duel | 3.62×, 3.61×, 4.00× | 3.45×, 3.25×, 4.23× |
| Four seats | 3.96×, 5.15×, 5.28× | 4.41×, 4.44×, 4.37× |
| Aggro | 6.72×, 8.69×, 6.55× | 7.18×, 6.47×, 6.75× |

The first series overlapped some of our verification jobs; the second ran
after those jobs finished. External load persisted in both. The artifact
prefix `isolated` names the second series but **does not mean an idle or
isolated machine**. These are observed ratios, not confidence intervals,
portable speedup promises, or estimates of CUDA performance.

## Iterations and safeguards

The following stages each ran the same 50-game duel probe, with matching game
summaries and chain heads. Intermediate profiles were short; allocation
changes are more useful than their non-monotonic wall times.

| Cumulative stage | MB/game | Allocation objects/game |
|---|---:|---:|
| Baseline | 26.130 | 1,342,235 |
| Ward eligibility guards | 13.751 | 572,257 |
| Trigger-face values | 8.726 | 258,181 |
| Replacement-face pointer | 8.321 | 207,652 |
| Mana normalizer reuse | 6.384 | 192,519 |
| Lazy filter iteration | 5.251 | 122,889 |
| Phase syntax memo | 4.564 | 58,580 |

### Ward eligibility

`rules/trigger_match.go`: granted Ward now rejects events other than
`TargetsChosen`, and objects absent from that event's target IDs, before
building printed-keyword maps or deriving characteristics. The existing
object visitation and trigger queue order stay intact. The ordinary trigger
scan still runs, including diagnostics and state-trigger handling.

The regression probe's irrelevant Ward call fell from six allocations to
zero. Existing granted-Ward tests exercise actual matching and payment.

### Face traversal

`roomTriggerFaces` returns two fixed slots plus a count; `replacementFace`
returns the selected face pointer. The ordinary single-face case no longer
allocates a slice per visited object. Room active/alternate order and
transform destination-face selection remain unchanged. Synthetic empty
scans fell from 80 trigger allocations and 81 replacement allocations to zero.

### Filter and mana parsing

`effects/filter.go` uses lazy alternative/conjunction iteration. It rejects
irrelevant same-name context prefixes before scanning predicates and avoids
allocating a split to find the last raw-name predicate. This retains raw
commas in card names, empty/trailing clauses and fail-closed unknown predicates.
The tested simple filter paths fell from three to seven allocations to zero.

`rules.ParseCost` and `cards.ProducedCounts` share immutable normalizers rather
than constructing them per call. Parsed mutable costs are **not** cached.

### Phase syntax

Both diagnostic scanning and phase matching use an engine-local cache of
pure parsed syntax. Diagnostic bookkeeping remains separate; a clone copies
the existing once-per-spec diagnostic state but starts with an empty syntax
cache. Tests pin both diagnostic behavior and independent writable caches.
Repeated diagnostic scans fell from six/eight allocations to zero.

### Botbench adapter

`cmd/botbench/main.go` uses `BoardFromGameInto` for `seat.BoardSeat`, retaining
View projection for other seats. Two full-game tests compare every intent and
outcome between Board and View and assert that capable seats use Board.

A separate native-botbench comparison used Go 1.25.11 for **both** binaries,
the same optimized engine, 50 games, one worker, CPU 14, and startup included.
All three View/Board text reports were byte-identical. Observed elapsed times
were 10.20/7.16, 11.15/6.60, and 11.01/6.63 seconds. These also experienced
shared-machine variance. Do not combine them with the probe's engine-only
allocation reductions: the probe already used Board on both sides.

## Remaining hotspots

A longer final-only duel profile covers 250 games, 122,434 intents and 658,610
events, with 16.30 seconds of CPU samples. Its first 50 result records match
the corresponding shorter run. It is for hotspot attribution, not a new
baseline/final comparison.

| CPU path | Inclusive share | Interpretation |
|---|---:|---|
| `legalActions` | 33.7% | Candidate construction and legality/payment work |
| `checkFaceTriggers` | 32.9% | Still walks every object and its trigger faces |
| `costStatics` | 9.5% | Repeated all-zone collection inside legal-action pricing |
| `BoardFromGameInto` | 8.6% | Observation construction after the View bypass |

Inclusive percentages overlap; notably `costStatics` is inside legal-action
work. The prior 50-game final profile put legal actions at 39.8%, trigger
scanning at 29.1%, cost collection at 10.7%, and `ParseCost` at 6.6%. Workload
mix and sampling variation matter even after allocation cleanup.

Follow-up final-only profiles broadened the sample to 50 four-seat games
(337,689 events, 11.23 seconds of CPU samples) and 150 aggro games (399,638
events, 11.56 seconds of CPU samples). Their initial 10 and 30 result records,
respectively, match the earlier comparison runs exactly.

| CPU path, inclusive | Duel, 250 games | Four seats, 50 games | Aggro, 150 games |
|---|---:|---:|---:|
| `checkFaceTriggers` | 32.9% | 46.3% | 30.4% |
| `legalActions` | 33.7% | 23.6% | 23.4% |
| `costStatics` | 9.5% | 7.4% | 3.2% |
| `BoardFromGameInto` | 8.6% | 5.5% | 16.4% |

Trigger scanning is the most consistently large remaining path, and overtakes
legal actions in the larger-seat and aggro workloads. This broader evidence
moves trigger-scan pruning ahead of cost-static caching for the next design
discussion. No pruning or cost-static cache has been implemented.

The longer duel profile attributes approximately 19.7% of allocated space to
`events.growEvents`, 10.8% directly to `staticEffects`, 8.7% to remaining
`strings.Replacer` construction and 8.4% to legal-action candidate appends.
The smaller profile traces most remaining replacer construction to
`effects.effMana`; only two normalizers were hoisted in this batch.

### Next design discussion: conservative trigger-scan pruning

The longer duel profile spends 1.31 seconds at the face-selection call, 0.47
seconds entering the face loop, and 1.18 seconds checking trigger matches.
Avoiding irrelevant sources may matter more than further tiny allocation
changes. However, `Always`, once-per-spec Phase diagnostics, look-back
observers, Room faces and granted Ward prevent simply skipping all
bookkeeping events or hidden-zone objects.

Investigate event-kind eligibility and immutable face metadata while retaining
the existing deterministic visitation and trigger order. Diagnostics must
still appear at the same event, state triggers must still be checked, and
Room/transform and live/look-back distinctions must survive. Compare this
with a maintained object index before choosing a design; the latter has a
larger invalidation surface. Obtain approval in the next brainstorming session
before implementation.

### Smaller follow-up: cost-static membership

`rules/statics.go` scans every eligible zone separately for RaiseCost,
ReduceCost and SetCost, repeatedly for each action being priced. Cache only
the ordered membership lists for a fixed event/log version; continue
evaluating dynamic conditions, amounts, targets, controller gates and announced
X on every call. Build all three lists in one ordered walk. Clones must own
fresh scratch; a rebuild must not overwrite a list an outer evaluation is
still ranging over.

This offers a clearer correctness boundary than caching final legal actions
or costs. Preserve alive-seat order, fixed zone order, per-face static order,
EffectZone rules and the shared stack's single visit. Test zone moves,
controller/face changes, player loss, target/X repricing and clone divergence.
Audit re-entrant event emission and direct-mutation test fixtures before
choosing an invalidation scheme. `handEngine` and `onBoard` explicitly bypass
events during test setup, including after `New` has enumerated legal actions.
A durable log-length-only cache can therefore retain the pre-fixture board.
One alternative to evaluate is reuse bounded to one action-enumeration call,
with ordinary recomputation outside that scope, rather than changing numerous
fixtures just to accommodate a cache. `events.Emit` appends before Apply,
and normal production membership changes are event-backed; dynamic conditions
still must not be cached. No cost-static cache is implemented here.

At 9.5% sampled CPU, even eliminating that path entirely would only yield
about 1.10× whole-workload throughput if everything else were unchanged;
real savings will be smaller. This is an incremental experiment, not another
Ward-sized gain.

### Search-specific work remains separate

The original clone probe found that the first append to a cloned log copies
its historical array: at one turn-11 boundary, approximately 385 KB for log
clone plus append versus 55 KB for engine Clone alone. Those are earlier
measurements, not remeasured improvements in this batch. A branch-friendly
immutable prefix/local suffix design would address a real MCTS cost, but
requires a separate design for history readers, hash parity and clone safety.

These profiles do not establish CUDA acceleration. The measured rules work
is branch-heavy traversal of Go objects, strings, maps and event history;
it cannot be offloaded by adding a CUDA flag. Batched neural policy/value
evaluation is a separate, GPU-friendly boundary. GPU-resident rules rollouts
would require another simulator representation/backend and parity testing,
not the source changes in this report. No CUDA implementation or GPU
benchmark was performed in this batch.

## Verification and known existing failures

Completed verification for this batch:

- Full changed backend packages passed under `make test`: cards, effects,
  rules and botbench. Replay, seat, state and view also passed.
- Targeted tests passed on Go 1.25.11 and Go 1.26.4.
- Targeted rules race tests passed, including Ward, Phase-cache and clone paths.
- `make sim`: 20/20 verified replays; no golden heads were updated.
- `go vet -p=2 ./...`, `go run ./cmd/gentypes -check`, formatting and
  `git diff --check` passed.
- Independent read-only review found no critical or important issues. Its
  optional clone-cache independence assertion was added and verified.

The repository-wide gates on the original measured revision were **not green**:

- `cmd/repro` and feedback tests have an existing event-1 fixture mismatch
  (recorded shuffle versus replayed note), plus a stale fresh-snapshot summary
  expectation. The host overshoot fixture diverges at event 76, and associated
  gated tests time out waiting for that overshoot.
- Architecture checks reject existing `cmd/ledger` time usage and an existing
  resume-writer allowlist mismatch.
- Web lint reports three existing errors: undefined
  `DisplayMediaStreamOptions`, unused `withSteppers`, and an explicit `any`.

The replay/feedback/host failures were reproduced with a Go overlay containing
exact HEAD versions of every modified production file; each overlay file's
git blob hash was checked against HEAD. The architecture and web sources
responsible for the other failures are unchanged. Fixtures and unrelated
allowlists were not modified to make the gates appear green. The opt-in,
known-red conformance lane is not a claimed passing gate.

## Synchronization checkpoint

The user requested a local branch, synchronization with origin, and a resume
prompt for a fresh brainstorming session. The optimization commit is
`e169cd2` on `perf/hotspot-optimization-2026-09-17`, rebased without conflicts
onto fetched `origin/main` at `57867a7`. Nothing was pushed and no PR was
created. The accompanying resume prompt is
`docs/superpowers/reports/2026-09-17-hotspot-resume.md`.

Origin first advanced to `2b452d3`, then to `57867a7` during verification.
The latter includes substantial sacrifice/unless-cost behavior, Draw
replacements, bot-policy changes and an updated host fixture. The branch
retains those upstream changes; its production diff remains the seven files
described in this report. No additional optimization was implemented during
synchronization.

Validation scope is deliberately explicit:

- At the first sync (`2b452d3` plus the optimization), a full `make test`
  completed. Changed backend packages passed, including the full rules
  package; all 22 failing test names matched the earlier run.
- On the final `57867a7`-based tree, full cards, effects, botbench, replay,
  seat, state and view package checks passed. The full host package reported
  just its committed-capture replay failure. Repro, feedback and architecture
  packages retained the failures described below.
- The final rules run passed with this selection (not a claim that the full
  rules suite was rerun after the second sync):
  `Test.*(Clone|Ward|Room|Phase|Replacement|Unless|Sacrifice|Cost|Target|Trigger|Filter)|TestEveryRepoDeck|TestRepoDecks|TestRepoDeckGames|TestHeads|TestFaceTriggerScan`.
- Final `make sim` verified 20/20 replays. Some heads differ from the earlier
  measurements because upstream changed rules/policy; they were not updated
  or hidden by this optimization commit.
- Final `go vet -p=1 ./...` and generated-type checks passed. Web sources
  were unchanged by the second sync; the first-sync lint run passed
  svelte-check and retained the same three ESLint errors.

The host's updated capture now diverges at **event 802**, recorded `move_zone`
versus replayed `choose`, rather than the historical event-76 mismatch. This
was independently reproduced using exact `57867a7` versions of **all seven**
modified production files through `/tmp/gorge-sync-baseline.IenA7V/overlay.json`.
Every extracted file's git blob hash was verified against that origin commit.
The repro/feedback event-1 mismatch, stale fresh-snapshot summary expectation,
architecture failures and web lint failures remain. These gates are not being
declared green. No fixture was regenerated by this branch.

The event-1 mismatch and fresh-snapshot summary failure were also reproduced
with the same exact-origin overlay on the final sync.

Final-sync logs are in `/tmp/gorge-hotspots.pfkHCN/`:
`final-sync-backend.log`, `final-sync-rules.log`, `final-sync-sim.log`,
`final-sync-baseline-host.log`, `final-sync-baseline-repro.log`; the first-sync full-suite and lint logs are
`post-sync-test.log` and `post-sync-lint.log`.

## Artifacts and reproduction

The durable record is this report plus the repository test sources in the
working tree. Raw diagnostics are local temporary artifacts and will not
survive cleanup of `/tmp`:

- Original probe source/module and investigation:
  `/tmp/gorge-profile-deep.FZ9z7k/{main.go,go.mod,REPORT.md}`.
- Optimization binaries, profiles, JSON summaries and verification logs:
  `/tmp/gorge-hotspots.pfkHCN/`.
- Cumulative stages: `baseline`, `ward`, `faces`, `replacements`,
  `normalizers`, `filters`, `phases`; each has a `*2` JSON/CPU/heap set.
- Repeated measurements: `repeat-*` and `isolated-*`; scripts `measure.sh`
  and `measure-botbench.sh`. `comparison.json` describes the first series only.
- Longer final profiles: `final-{duel,four,aggro}-long` with `.json`, `.cpu`,
  `.heap` and `.baseline.heap` suffixes.
- Verification: `test-lint.log`, `lint.log`, `race.log`, `sim.log`,
  `go126-tests.log`, `baseline-failures.log`, `baseline-overlay.json`.

With those artifacts still present:

```sh
go tool pprof -top -cum /tmp/gorge-hotspots.pfkHCN/phases \
  /tmp/gorge-hotspots.pfkHCN/final-duel-long.cpu
go tool pprof -alloc_space \
  -base /tmp/gorge-hotspots.pfkHCN/final-duel-long.baseline.heap \
  -top /tmp/gorge-hotspots.pfkHCN/phases \
  /tmp/gorge-hotspots.pfkHCN/final-duel-long.heap
taskset -c 15 bash /tmp/gorge-hotspots.pfkHCN/measure.sh another-series
```

Focused repository regressions, without the temporary probe:

```sh
GOMAXPROCS=2 GOMEMLIMIT=5GiB go test -p=1 \
  ./cards ./effects ./rules ./cmd/botbench \
  -run 'TestProducedCountsReuses|TestSimpleFilterMatching|TestGrantedWard|TestFaceTriggerScan|TestReplacementScan|TestPhaseDiagnostic|TestParseCostReuses|TestCloneStaysIndependent|TestHeads|TestPlayMatchUsesBoardSeatWithViewParity' \
  -count=1
make sim
```
