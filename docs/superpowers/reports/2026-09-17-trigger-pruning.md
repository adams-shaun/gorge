# Conservative trigger eligibility pruning

Date: 2026-09-17. Baseline: `9db4ca4`, on
`perf/hotspot-optimization-2026-09-17`, based on `origin/main` `57867a7`.
Corpus pin: `95f04e8a04c8925fa97cb226fc3341cabcc90a53`.

This is the second, separately approved optimization. It does not redo the
first batch, maintain an object index, cache costs, change bot policy, or
change event kinds, state mutation, fixtures, or golden heads. Work remains
local and uncommitted.

## Fresh baseline and chosen scope

The post-sync baseline rebuilt the unchanged diagnostic probe against
`9db4ca4`. Its three profiles covered 450 games, 264846 intents and 1396138
events. Trigger scanning remained the largest consistent target:

| Inclusive CPU path | Duel | Four seats | Aggro |
|---|---:|---:|---:|
| Trigger scan | 32.62% | 48.07% | 31.30% |
| Legal actions | 33.59% | 22.31% | 25.84% |
| Cost-static collection | 9.79% | 7.32% | 3.43% |
| Board projection | 9.27% | 5.73% | 17.80% |

These shares overlap. The first batch's pre-sync outcomes were not used as
this change's baseline: 13 of the 50 four-seat games changed across the sync.
The fresh baseline details are locally preserved at
`/tmp/gorge-hotspot-baseline.PBlRp2/REPORT.md`.

The user approved immutable face/event eligibility pruning while retaining
the deterministic all-zone walk. A per-trigger kind guard alone would leave
face setup overhead. A maintained object index could skip more objects but
would add invalidation and ordering obligations for zone movement, control,
face changes, Room unlocks and look-back snapshots. Cost caching and a
branch-friendly event log remain separate, unapproved follow-ups.

## Implementation and correctness boundary

`rules/trigger_eligibility.go` maps trigger modes to a conservative event-kind
mask. `triggerMatches` rejects impossible kinds before dynamic zone/phase
gates. The mapping follows the current matcher, including its narrow
`SpellAbilityCast` = `AbilityPush` behavior; it adds no rules support.

An engine-local map caches each immutable face's union of eligible kinds.
The ordinary scanner skips face-walk setup when that face is irrelevant;
unlocked Rooms still select and check both live faces in the existing order.
Empty faces need no cache entry. No source membership, controller, zone,
condition, target, firing limit, or evaluated match is cached.

Conservative exceptions keep the original path:

- Every face carrying nonblank `Phase$`, including valid and invalid specs.
  Diagnostic once-per-spec state remains separate. Hidden-zone diagnostics,
  source selection, ordering, and emission after the walk remain unchanged.
- `Always`, `LifeLostAll`, and unknown modes. State-trigger latches and
  life/damage batching stay authoritative.
- Event kinds beyond the 64-bit mask go through the full matcher rather than
  being lost to a truncated shift. Extending an existing matcher's accepted
  kinds also requires updating its mask and matrix test.

Granted Ward is still checked when printed-face work is skipped. The
live/look-back split, LKI capture, trigger keys, pending queue, APNAP drain,
and all non-face trigger hooks are unchanged. The live queue owner owns the
pure metadata even when the observer is a snapshot. Clones start with empty,
independent writable syntax caches; no cache is stored on a shared snapshot
or compiled card.

## Measurements

Both binaries use Go 1.26.4 linux/amd64, `GOMAXPROCS=1`, `GOMEMLIMIT=5GiB`,
CPU 15, fixed decks/seeds, and the direct Board adapter. Corpus loading and
one complete warm-up game are excluded. New game construction and per-engine
cache population are included. These are constructed games without mulligans.
Benchmark workers ran serially; correctness gates did not overlap the paired,
long-profile or clone measurement series. The earlier smoke run is parity
evidence only, not timing evidence.

| Profile | Games | Baseline CPU samples | Candidate CPU samples | Baseline trigger CPU | Candidate trigger CPU |
|---|---:|---:|---:|---:|---:|
| Death-n-Taxes / Dimir Tempo | 250 | 15.42 s | 12.60 s | 5.03 s (32.62%) | 2.20 s (17.46%) |
| Four Legacy decks | 50 | 14.48 s | 8.27 s | 6.96 s (48.07%) | 2.14 s (25.88%) |
| Goblins / Green Stompy | 150 | 12.81 s | 9.50 s | 4.01 s (31.30%) | 1.21 s (12.74%) |

All 450 candidate results match the fresh baseline's per-game seed,
intent/event count, turn, winner/draw and final chain head; aggregate event
kind counts also match. This compares summaries and hashes, not stored full
event arrays. Repeated comparisons below revisit the first 90 of these games,
not an additional independent seed population.

Three unprofiled baseline/candidate pairs per workload reversed run order in
the middle pair. Every pair matched results/heads and event-kind counts.
Allocated bytes are exact runtime.MemStats deltas, not sampled attribution
and not retained memory. MB below is decimal; figures are medians over runs.

| Workload | Games/run | Before MB/game | After MB/game | Extra bytes/game | Observed paired elapsed ratios (before/after) |
|---|---:|---:|---:|---:|---|
| Duel | 50 | 4.574192 | 4.574696 | 504 | 1.21, 1.02, 1.49 |
| Four seats | 10 | 13.596932 | 13.598106 | 1174 | 1.54, 1.66, 1.66 |
| Aggro | 30 | 5.931644 | 5.931854 | 210 | 1.30, 1.52, 1.27 |

Allocation objects/game increase from 58615.96 to 58620.24 (duel),
172351.80 to 172362.20 (four seats), and 57432.03 to 57434.83 (aggro).
This change trades a small cache allocation for less CPU work; it is not an
allocation reduction. The shared machine remains noisy and CPU affinity does
not isolate hardware. Neither elapsed ratios nor profile-sample ratios are
portable speedup promises.

After pruning, legal-action generation is the largest of the measured paths
(41.11/33.13/30.95% inclusive CPU), with cost collection nested inside it
(10.87/10.40/3.47%). Board projection takes 10.79/11.12/23.05%. No next
optimization has been implemented or implicitly approved.

### Clone/first-submit tradeoff

Two alternating baseline/candidate runs used the probe's three snapshots
from seed-zero duel play: intents 0/100/300, turns 1/5/11, all priority
decisions. Snapshot metadata matched in both runs; the probe also checked
that each operation left the original log head unchanged.

| Position | Engine.Clone bytes/op, both versions | Clone+Submit before | Clone+Submit after |
|---|---:|---:|---:|
| Intent 0 | 52928 | 61800 | 62320 |
| Intent 100 | 54248 | 198576 | 199096 |
| Intent 300 | 55224 | 472041 | 472561 |

Both runs returned these allocation counts: Engine.Clone is unchanged;
the first Submit on a fresh clone adds 520 bytes and five allocations to
populate the new independent cache. Observed Clone+Submit timings were lower
in each pair, but this limited three-position sample does not establish MCTS
throughput. Standalone Clone timings varied in both directions despite
unchanged allocations. The turn-11 log-clone-plus-append still allocates
385416 bytes: historical log copying is not fixed by trigger pruning.

## Regression evidence and review

New tests cover the mode/event matrix across every uint8 kind, face unions,
conservative exceptions, allocation-free warm lookups, independent clone
caches, hidden-zone diagnostic order/deduplication, `Always` on bookkeeping
events, and unlocked Room alternate triggers with either face cast.

The cold irrelevant-event matcher test was run against the original code
before implementation. Compiler escape analysis identified a test-setup
allocation, which was excluded from measurement. Disabling the implemented
kind guard then failed the corrected test with five gate allocations;
restoring the guard passed with zero. The hidden-zone diagnostic, `Always`,
and Room preservation tests passed against the original scanner as well as
the candidate. Existing end-to-end granted-Ward tests cover targets without
printed triggers.

Independent read-only review found no Critical, Important or Minor findings.
The reviewer ran no competing tests during profiling. Its one missing-file
inspection was benign and corrected; no unresolved tool failures were reported.

## Final verification

All commands below completed successfully on the final candidate with the
corpus present. Ordinary checks used Go 1.25.11, `GOMAXPROCS=2` and
`GOMEMLIMIT=5GiB`; the eligibility tests also passed under the measurement
toolchain, Go 1.26.4.

```sh
go test -p=1 ./cards ./effects ./rules ./cmd/botbench ./replay ./seat ./state ./view -count=1
go test -race -p=1 ./rules \
  -run 'TestTriggerEligibility|TestGrantedWard|TestPhaseDiagnostic|TestSBABatch|TestClone|TestLifeLostAll|TestRoom|TestDamage.*Once' \
  -count=1
make sim
go vet -p=1 ./...
go run ./cmd/gentypes -check
```

The full rules suite passed (400.060 s), including its replay/golden-head
checks. The focused race run passed (93.704 s). `make sim` verified **20/20
replays**; its seed summaries and replay heads match the fresh baseline.
Formatting and `git diff --check` passed. No fixture, golden, corpus pin,
dependency, event ordinal or generated type was changed.

This is not a full-repository green claim. The previously documented
repro/feedback, host-capture, architecture and web-lint failures were not
rerun or repaired in this candidate verification. The deliberately known-red
opt-in conformance lane was not enabled. No commit, push, merge or deployment
was performed.

## Local artifacts

- Baseline: `/tmp/gorge-hotspot-baseline.PBlRp2/` (`probe`, profiles, repeated
  JSON, baseline verification logs and `REPORT.md`).
- Candidate: `/tmp/gorge-trigger-pruning.TgomPa/` (`candidate`, `measure.sh`,
  `paired-*` JSON, `candidate-*-long` CPU/heap/JSON, `cpu-*.txt`, clone JSONL,
  `focused.log`, `mutation-red.log`, compiler escape diagnostics,
  `backend.log`, `race.log`, `sim.log`, `vet.log`, `gentypes.log`,
  `go126-tests.log`).
- Reused diagnostic source/module:
  `/tmp/gorge-profile-deep.FZ9z7k/{main.go,go.mod}`.

The measurement script is run as
`taskset -c 15 bash /tmp/gorge-trigger-pruning.TgomPa/measure.sh`.
It runs three alternating pairs and one candidate long profile per workload,
then two alternating baseline/candidate clone-measurement pairs. Decks are
`death-n-taxes,dimir-tempo`; those plus `eldrazi-stompy,mono-black-aggro`;
and `mono-red-goblins,mono-green-stompy`. Seeds start at zero. Temporary
artifacts may disappear; do not overwrite the baseline binary with a rebuild
against changed source and still call it the baseline.

A final rebuild after mutation testing was byte-identical to the measured
candidate (`sha256 adc55f5a2e7841d4e6ff8bd5804778a1732509cfa8069537ff94542305842051`).
The baseline binary and IR-cache checksums still match the fresh baseline
report; no corpus rebuild or pin change entered this comparison.
