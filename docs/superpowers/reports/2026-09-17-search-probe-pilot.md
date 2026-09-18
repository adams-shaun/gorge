# Search-label probe: 500-game feasibility pilot

## Outcome

The feasibility pilot ran; **the proposed action-label strength comparison did
not run**. No valid hypothetical-world sampler, static-only challenger, search
policy, trainer, or production change was implemented.

Keep the recommendation **not yet** for live search or a trainer. The next
design should address history-conditioned world reconstruction. This pilot
does not establish that search cannot improve the bot.

Two findings determine the next step:

- **258/500 selected roots (51.6%)** already followed a non-genesis shuffle,
  library reorder, or return-to-library. Sampling only an initial deal cannot
  cover these histories. Restricting evaluation to the other 242 roots would
  change the target population, not answer the original headline matchup gate.
- Four-world terminal rollout cost projects to **378–429 ms/root on average**,
  before world construction. This exceeds the proposed 250 ms target. Rolling
  to end of turn projects to **16–19 ms/root**, but its value as a teaching
  signal remains untested.

The second result is arithmetic over measured true-state branches, **not a
measurement of four sampled worlds** and not a rigorous search cost bound.

## Checkout and scope

- Branch: `perf/hotspot-optimization-2026-09-17`.
- HEAD and cached upstream: `1cfef874b83d41a1229dc067bacc8d920c82d451`.
- Merge-base with cached `origin/main`: `8bce166943e3d09cce81acad4797a6106630ad5c`.
- Origin: `git@github.com:adams-shaun/gorge.git`; no fetch performed.
- Go: `go1.25.11 linux/amd64`.
- Every run used `GOMAXPROCS=5`, `GOMEMLIMIT=5GiB`.
- Corpus pin: `95f04e8a04c8925fa97cb226fc3341cabcc90a53`.
- IR SHA256: `f6f87777b35f7270ac8caafe8c34fbe9cffd3055f1efaa16256aa77cdf636477`.

The existing edit to `2026-09-17-hotspot-resume.md` was preserved. All diagnostic
code and raw output remain throwaway files in
`/tmp/gorge-search-pilot.iNfFJg/`; this report is the only repository addition.
No engine/policy source edits, commits, remote writes, golden regeneration,
deployments, race tests, or additional dependencies.

## Protocol

Played current bot against itself using explicit decks
`death-n-taxes,dimir-tempo`, seeds **10000–10499**, ordinary constructed
configuration with no mulligans. These are disjoint from the earlier baseline
seeds. Every baseline game was also replayed from Config plus its recorded
intents, checking head, event count, and RNG draw count.

At each decision, preserve the current policy's land/mana handling. A search
surface exists only when its selected action is `cast`, `ability`, or `pass`
and there are at least two offered options in that group. Concede is excluded.
This definition includes offered abilities the current bot considers unhelpful;
it is a concrete intervention surface, not a count of tactically valuable
choices.

Select one root per game: the first such decision at engine turn >=5 for seat
`seed % 2`. Selection uses no eventual outcome. All 500 games supplied a root;
mean root turn was 5.574, median 5, p95 7, maximum 14. Engine turns here are
individual player turns, not full rounds.

Retain the baseline action and pass, then fill up to eight candidates in fixed
structured action-key order (kind, source ID, player, ability, mode,
alternative cost, SVar). There is no learned or static candidate ranking in
this calibration. Indices are used only to submit into the same cloned root;
cross-world action mapping has not been validated.

For each retained candidate, clone the **actual engine** and continue using
the current observation-based bot policy with fresh, fixed per-seat bot RNGs.
The clone retains the actual engine RNG and hidden zones: these branches are
**cost measurements only**, never action labels or strength evidence. Stop at
terminal or 5000 submitted intents, not a clock deadline. No branch hit the
cap. Timing includes clone, submission, observation construction, policy, and
normal event hashing. Sample cumulative elapsed and submit counts at next
same-seat priority, first step boundary, first turn boundary, and terminal.
Terminal states also terminate the earlier horizons.

## Results

Five-worker pilot: 19.52 s wall / 93.11 s user CPU, including corpus loading,
500 baseline games, their replays, and 1460 terminal candidate branches.
Repeat: 19.81 s wall / 94.97 s user CPU. Peak RSS was about 275 MiB in both.
This is **not** ordinary 500-game botbench throughput or retained memory/node.

Across the 500 complete baseline games:

| Quantity | Result |
|---|---:|
| Decisions/game | 479.622 |
| Forced priority decisions/game | 218.870 |
| Defined search surfaces/game | 108.406 |
| Candidates/surface, including pass | mean 2.750; median 2; p95 5; p99 7; max 13 |
| Surfaces with more than eight candidates | 145 / 54203 |
| Candidates at selected roots | mean 2.920; max 8 |

At selected roots, overlapping history requirements were:

| Requirement observed before the root | Roots |
|---|---:|
| Non-genesis shuffle | 98 |
| Library reorder | 205 |
| Return-to-library | 126 |
| Union of those three | 258 |
| Actor received a decision offering library objects | 123 |
| Public zone-to-hand move | 97 |
| Public ID-bearing Note, requiring knowledge interpretation | 116 |

The union was 122/250 for seat 0 and 136/250 for seat 1. These categories
describe reconstruction requirements; they do not claim every Note was a
hand reveal or that the remaining roots are already safe to determinize.

First five-worker run, 1460 candidate branches:

| Horizon | Mean elapsed/candidate | Mean submitted intents |
|---|---:|---:|
| Next same-seat priority | 0.239 ms | 1.681 |
| End of step | 0.589 ms | 6.550 |
| End of turn | 1.582 ms | 21.390 |
| Terminal | 35.880 ms | 377.150 |

Mean clone cost was 44.0 us; mean first Submit cost was 134.8 us. The stack
was still nonempty at the next-priority horizon in **985/1460** branches.
Returning priority is therefore not a resolved-consequence horizon.

For each root, sum its measured candidate costs and multiply by four:

| Horizon | Four-world projection, 500 roots / 5 workers | Confirmation, first 125 roots / 1 worker |
|---|---:|---:|
| Next same-seat priority | mean 2.79 ms; p95 5.03 ms | mean 2.35 ms; p95 3.88 ms |
| End of step | mean 6.88 ms; p95 15.20 ms | mean 5.66 ms; p95 12.73 ms |
| End of turn | mean 18.48 ms; p95 39.43 ms | mean 15.79 ms; p95 28.95 ms |
| Terminal | mean 419.08 ms; p95 1020.18 ms | mean 377.72 ms; p95 787.06 ms |

The repeated 500-root run projected 18.69 ms to turn end and 428.59 ms to
terminal. The one-worker confirmation still used `GOMAXPROCS=5`; it is not
a CPU-affinity-pinned single-core benchmark. Parallel-run per-branch wall
times include scheduling and GC effects. Hypothetical worlds may produce
different branching and game lengths; construction, aggregation, scoring,
inference, and retained search memory remain unmeasured. Do not extrapolate
these early-root costs to every decision of a live game.

## Why world construction needs a separate design

`state/determinize.go` documents that it forgets prior reveals. It directly
redistributes true hidden cards and is not an event-backed engine constructor.
`rules/clone.go` and `rules/rng.go` preserve actual hidden zones and the exact
RNG position. Neither is the required sampler.

Appending artificial hand/library MoveZone events is not a substitute:
`events.Move` records `EnteredThisTurn`, `EnteredFrom`, and `Game.Entered`,
among other zone-transition effects. A hypothetical previous deal must not
create new current-turn movement history. Replay-time rules caches and
continuations also cannot be reconstructed by swapping only `Engine.G`.

Raw seat event redaction is not itself a safe search knowledge representation:
it passes the owner's secret Shuffle and complete LibraryOrder payloads.
Shuffle order is unknown; LibraryOrder can contain an unseen suffix. The audit
strips both payloads and uses only their occurrence. A sampler must recover
legitimate known order from the private look/choice history, not those whole
library payloads. Bursts were redacted at their own end state, not retroactively
against the final snapshot.

The follow-up design should specify an event-backed constrained replay
constructor taking public deck lists and the acting seat's public/private
observation history, with independent hypothetical RNG. Opponent-private
knowledge must be hypothetical, not imported from the actual recording. The
constructor must cover reveals, returns,
library looks/reorders/searches, and subsequent shuffles; validate the entire
observable prefix; reject impossible histories; and measure sampling/rejection
cost. It must not feed true opposing hands, original RNG, hidden-inclusive
hashes, or opponent-private intent details into root decisions.

Only after that correctness boundary is tested should the original frozen
static/current/shallow/aggregated comparison proceed. End-of-turn search is
the affordable calibration candidate, not a proven good evaluator. Preserve
the original paired +3pp gate and independent-seed breadth confirmation;
these 500 feasibility games do not replace them. No teacher promotion or
training-generation economics are established here.

## Verification and reproduction

The throwaway tests were exercised red-to-green for observation filtering,
history classification, candidate retention, and consequence horizons. They
also check fixed submit caps, deterministic branch replay, and source-log
independence. `go test ./... -count=1` and `go vet ./...` passed in the pilot
module. Every one of the 500 baseline replays matched in both runs. Every
sampled root, candidate identity, branch head, and horizon submit count matched
between the runs, and also matched the 125-game one-worker subset.

Fresh repository verification passed, without race instrumentation:

```sh
GOMAXPROCS=5 GOMEMLIMIT=5GiB go test ./rules ./seat ./cmd/botbench \
  -run '^(TestCloneStaysIndependentAndReplaysInLockstep|TestCloneSharesNoMutableStateWithTheOriginal|TestBotAdaptersAgreeOverWholeGame|TestPlayMatchUsesBoardSeatWithViewParity)$' \
  -count=1
```

Rules: 0.025 s; seat: 5.402 s; botbench: 1.026 s. No broad-suite claim.

To reproduce while the temporary directory remains available:

```sh
cd /tmp/gorge-search-pilot.iNfFJg
GOMAXPROCS=5 GOMEMLIMIT=5GiB go test ./... -count=1
GOMAXPROCS=5 GOMEMLIMIT=5GiB go build -o pilot .
GOMAXPROCS=5 GOMEMLIMIT=5GiB ./pilot \
  -games 500 -workers 5 -seed 10000 -out rerun.json
```

Raw artifacts: `pilot500.json`, `pilot500-repeat.json`, `serial125.json`, and
their corresponding `.time` files in that directory. The pilot deliberately
does not serialize Forge script text.
