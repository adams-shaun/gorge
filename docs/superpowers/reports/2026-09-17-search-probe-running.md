# Search probe: final history-conditioned coverage calibration

The final local code completed the fixed 500-root calibration with no invariant
errors. **138/500 roots (27.6%)** met the four-positive-proposal / ESS >=4
gate; 362 roots retained ordinary baseline fallback. This is a correctness,
coverage, and cost measurement only--not strength or promotion evidence.

## Exact artifacts and commands

Run from `/tmp/gorge` on the local working tree at `HEAD`
`9ea75078a0e1c194d4440adc2c34e7db227dab5e`, with the pinned `.cards` cache:

```sh
GOMAXPROCS=5 GOMEMLIMIT=5GiB go run ./cmd/searchprobe \
  -games 500 -workers 5 -seed 10000 -sample-seed 54321 \
  -attempts 64 -worlds 4 -max-submits 5000 \
  -out /tmp/gorge-searchprobe-finalwave-500-20260917.json

GOMAXPROCS=5 GOMEMLIMIT=5GiB go run ./cmd/searchprobe \
  -games 125 -workers 1 -seed 10000 -sample-seed 54321 \
  -attempts 64 -worlds 4 -max-submits 5000 \
  -out /tmp/gorge-searchprobe-finalwave-125-20260917.json

jq -e --slurp \
  '(.[0].Results[:125] | map(del(.SampleNS, .SearchNS))) ==
   (.[1].Results | map(del(.SampleNS, .SearchNS)))' \
  /tmp/gorge-searchprobe-finalwave-500-20260917.json \
  /tmp/gorge-searchprobe-finalwave-125-20260917.json
```

Both commands exited 0. The 500-root run reported 138 covered roots and zero
errors; the serial 125-root prefix reported 28 covered roots and zero errors.
The `jq` comparison returned `true`: every matching per-game field is equal
after removing exactly `SampleNS` and `SearchNS`. Aggregate timing, worker, and
memory metadata were intentionally not compared.

The full required checks also passed:

```text
GOMAXPROCS=5 GOMEMLIMIT=5GiB go test ./... -count=1  # exit 0
GOMAXPROCS=5 GOMEMLIMIT=5GiB go vet ./...            # exit 0
git diff --check                                      # exit 0
```

No race suite or golden regeneration ran. `git status --short` contained the
local sampler/engine/tests/spec/plan work only; it contained no tracked Forge
card or token scripts. All changes remain local and unstaged. Verification is
complete; separate authorization is required and is now requested before any
commit or push.

## Protocol

- Death-n-taxes versus dimir-tempo; current/current baseline; no mulligans.
- Seeds 10000--10499; actor `seed % 2`; first eligible turn>=5
  cast/ability/pass root; baseline land/mana handling retained.
- Baseline and pass retained in a maximum-eight candidate set; 64 fixed
  reconstruction attempts, four worlds, 5,000 submit cap, and sampler seed
  54321 independent of game seed.
- A root is usable only with four positive proposals and ESS >=4. Selected
  worlds replay from their own chance tapes and intent logs; resampled
  duplicates receive independent mutable engines.

## Coverage, sampling, and cost

| Measurement | Total | Death-n-taxes actor | Dimir actor |
|---|---:|---:|---:|
| Roots retained | 500 | 250 | 250 |
| Usable four-world roots | 138 | 31 | 107 |
| Baseline fallbacks | 362 | 219 | 143 |
| Accepted proposals / attempts | 2,473 / 32,000 | 312 / 16,000 | 2,161 / 16,000 |
| Prefix rejections | 29,421 | 15,683 | 13,738 |
| Incompatible proposals | 106 | 5 | 101 |
| Submit-budget exhaustions | 0 | 0 | 0 |
| Reconstruction submissions | 1,228,363 | 419,636 | 808,727 |
| Sum of per-root ESS | 2,473 | 312 | 2,161 |
| Duplicate selected entries | 80 | 30 | 50 |
| Guided genesis shuffles | 63,936 | 32,000 | 31,936 |
| Guided later shuffles | 131 | 20 | 111 |
| Actor arrange-window uses | 3,016 | 0 | 3,016 |
| Unguided-constraint occurrences | 34,327 | 20,119 | 14,208 |

The attempt accounting closes exactly: 2,473 accepted + 29,421 prefix
rejected + 106 incompatible + 0 budget exhausted = 32,000. Covered-root ESS
ranged from 4 to 64. The four-world arm replayed 552 selected entries (four
per covered root); the one-world arm replayed 138. All 500 baseline replays
and all 2,000 paired terminal outcome replays succeeded; no outcome was
nonterminal.

Of 258 roots with a later shuffle/reorder/library-return epoch, 41 were usable:
zero of 122 Death-n-taxes roots and 41 of 136 Dimir roots. The 258 roots stayed
in the population; none were filtered to improve this result.

| Cost measurement | Value |
|---|---:|
| Corpus load / total elapsed | 0.768 / 114.186 s |
| Sampler elapsed per root, p50 / p95 / max | 574.488 / 1,210.666 / 4,507.111 ms |
| Combined search elapsed, covered-root p50 / p95 / max | 53.828 / 85.533 / 114.948 ms |
| Allocations after corpus load | 83,695,350,608 bytes |
| End HeapAlloc / HeapSys | 4,011,688 / 246,906,880 bytes |

Per-root sampler elapsed is measured under five-worker contention and sums to
326.222 worker-seconds; it is not isolated latency. Allocation and heap values
are Go allocator counters, not peak RSS.

## Deterministic rejection histogram

The 29,421 prefix rejections aggregate to 26,967 identity and 2,454 event
buckets. The leading normalized buckets were:

| Component / shape | Count |
|---|---:|
| identities / hand_to_battlefield | 11,048 |
| identities / hand_to_stack | 10,625 |
| identities / other | 3,686 |
| events / priority_to_stack_resolve | 1,679 |
| identities / hand_to_exile | 1,583 |
| events / choose_to_choose | 378 |
| events / priority_to_priority | 168 |
| events / stack_resolve_to_priority | 140 |
| all remaining normalized buckets | 114 |

These are diagnostics of the full-prefix correctness oracle, not hidden-card
diagnostics and not a reason to weaken rejection.

## Supported and unguided history shapes

The proposal compiler uses only the actor-owned allowed history and public deck
definitions. It supports actor-visible draws as fixed positions (a known public
object stays a physical-object constraint; otherwise the card name is fixed),
actor-visible `KArrange` ordered top-card windows, and cumulative name-count
deadlines from an opponent's first public Hand-to-public-zone appearance. A
supported actor answer deterministically reorders the current top window;
matching `LibraryOrder` preserves a mapping to original shuffle positions.
Top/reorder and bottom-placement answers are supported, including subsequent
draws of cards not exposed in the arrange window. Only semantic actor actions
and the observed public library size are consumed, never private intent indices.
A later Shuffle begins a new epoch; matching cards already in the hypothetical
hand first satisfy that epoch's public-play requirement, and only the remaining
deficit constrains post-shuffle draws. Genesis and later shuffle permutations
are sampled uniformly over the exactly counted compatible physical orders.

Unmodelled, opponent-private, or invalid `library_order` (including unsupported
graveyard-destination arrangements), non-draw `library_mutation`, an arrange window after
positional reliability is lost, and an opponent hand exit after that loss are
explicitly unguided shapes. Earlier supported constraints remain usable; an
epoch with none uses the ordinary prior. All stay subject to full-prefix
rejection. The 34,327 occurrence count above is across
proposal attempts, not a count of unique games.

Nameless stack-ability identities are permitted; actual card facts still require
names. Reusing one exact object reference at different positions is a public
contradiction. The experiment now retains such typed contradictions as a distinct
baseline fallback and still runs baseline/outcome replay; invariants remain
fatal. This fixed run had no contradiction fallbacks: all 362 fallbacks were
`insufficient sampled worlds/ESS`. Rejection shapes now consider only differing
identity declarations, not earlier matching cards in the same frame.

## Comparison with the prior fixed checkpoint

| Checkpoint measurement | Prior | This run |
|---|---:|---:|
| Accepted proposals | 164 / 32,000 | 2,473 / 32,000 |
| Usable roots | 17 / 500 | 138 / 500 |
| Death-n-taxes actor usable roots | 0 / 250 | 31 / 250 |
| Dimir actor usable roots | 17 / 250 | 107 / 250 |
| Usable later-epoch roots | 6 / 258 | 41 / 258 |

This is a sampling-coverage comparison, not an evaluation of action quality or
search strength. The remaining bottleneck is the large full-prefix rejection
tail, dominated by opponent hand-to-battlefield and hand-to-stack identity
events plus unguided epochs; 362 roots still fall back after the fixed work
budget. Further work must retain the information boundary, exact proposal
likelihoods, all-root accounting, and full-prefix validation.

The preceding pre-final-review artifacts remain untouched at
`/tmp/gorge-searchprobe-task6-{500,125}-20260917.json`; that 500-root run had
135 covered roots and 2,432 accepted proposals. The new final-code run adds
three covered later-epoch roots and 41 positive proposals. This is a coverage
change, not evidence of stronger play.

---

# Land-isolation proposal follow-up

The exact 50/50 land-isolation mixture reduced the targeted opponent
hand-to-battlefield rejection bucket and increased raw accepted proposals, but
it did not improve the four-world/ESS coverage gate. **131/500 roots (26.2%)**
were usable, down from 138/500. The result is therefore retained as bounded
coverage/correctness evidence, not as promotion readiness or a strength claim.

## Artifacts and commands

The final optimized artifacts are:

```text
/tmp/gorge-searchprobe-land-isolation-opt-500-20260917.json
/tmp/gorge-searchprobe-land-isolation-opt-125-20260917.json
```

They were produced with the same fixed protocol:

```sh
GOMAXPROCS=5 GOMEMLIMIT=5GiB go run ./cmd/searchprobe \
  -games 500 -workers 5 -seed 10000 -sample-seed 54321 \
  -attempts 64 -worlds 4 -max-submits 5000 \
  -out /tmp/gorge-searchprobe-land-isolation-opt-500-20260917.json

GOMAXPROCS=5 GOMEMLIMIT=5GiB go run ./cmd/searchprobe \
  -games 125 -workers 1 -seed 10000 -sample-seed 54321 \
  -attempts 64 -worlds 4 -max-submits 5000 \
  -out /tmp/gorge-searchprobe-land-isolation-opt-125-20260917.json

jq -e --slurp \
  '(.[0].Results[:125] | map(del(.SampleNS, .SearchNS))) ==
   (.[1].Results | map(del(.SampleNS, .SearchNS)))' \
  /tmp/gorge-searchprobe-land-isolation-opt-500-20260917.json \
  /tmp/gorge-searchprobe-land-isolation-opt-125-20260917.json
```

Both runs exited 0 with zero experiment errors. The 125-root run covered 26
roots, and the `jq` comparison returned `true`. Every corresponding per-root
field matched after removing only `SampleNS` and `SearchNS`.

## Proposal and accounting

For current constrained set `C` and nonempty land-isolated subset `L`, the
implementation samples each component with probability one half and uses the
exact physical-permutation density

```text
q(x) = 1/(2|C|) + I[x in L]/(2|L|).
```

An ineligible or empty `L` samples uniformly from `C`; the empty case consumes
no component-selection draw. Opponent actions still come from
`botpolicy.Decide`, and full-prefix validation remains the acceptance oracle.

| Measurement | Total | Death-n-taxes actor | Dimir actor |
|---|---:|---:|---:|
| Roots retained | 500 | 250 | 250 |
| Usable four-world roots | 131 | 39 | 92 |
| Baseline fallbacks | 369 | 211 | 158 |
| Accepted proposals / attempts | 3,155 / 32,000 | 667 / 16,000 | 2,488 / 16,000 |
| Prefix rejections | 28,715 | 15,300 | 13,415 |
| Incompatible proposals | 130 | 33 | 97 |
| Submit-budget exhaustions | 0 | 0 | 0 |
| Reconstruction submissions | 1,310,803 | 488,514 | 822,289 |
| Sum of per-root ESS | 1,813.867 | 394.788 | 1,419.079 |
| Duplicate selected entries | 94 | 42 | 52 |
| Guided genesis shuffles | 63,936 | 32,000 | 31,936 |
| Guided later shuffles | 133 | 65 | 68 |
| Actor arrange-window uses | 3,012 | 0 | 3,012 |
| Unguided-constraint occurrences | 34,434 | 20,226 | 14,208 |
| Land-isolation eligible shuffles | 31,335 | 15,425 | 15,910 |
| Isolation-component selections | 15,744 | 7,785 | 7,959 |
| Empty-isolation fallbacks | 17 | 0 | 17 |
| Unsupported land-isolation shapes | 119 | 118 | 1 |

The attempt accounting closes exactly: 3,155 accepted + 28,715 prefix
rejected + 130 incompatible + 0 budget exhausted = 32,000. Covered-root ESS
ranged from 4 to 42.089. The one-world and four-world arms replayed 131 and
524 selected entries. All 500 baseline replays and all 2,000 terminal outcome
replays succeeded. Of 258 later-epoch roots, 35 were usable: zero of 122
Death-n-taxes roots and 35 of 136 Dimir roots.

## Rejections and comparison

The 28,715 prefix rejections split into 25,547 identity and 3,168 event
buckets. The leading normalized buckets were:

| Component / shape | Count |
|---|---:|
| identities / hand_to_stack | 13,034 |
| identities / hand_to_battlefield | 6,668 |
| identities / other | 3,776 |
| events / priority_to_stack_resolve | 2,281 |
| identities / hand_to_exile | 2,017 |
| events / choose_to_choose | 411 |
| events / priority_to_priority | 253 |
| events / stack_resolve_to_priority | 131 |
| all remaining normalized buckets | 144 |

| Checkpoint measurement | Previous | Land isolation | Change |
|---|---:|---:|---:|
| Accepted proposals | 2,473 | 3,155 | +682 |
| Usable roots | 138 | 131 | -7 |
| Death-n-taxes actor usable | 31 | 39 | +8 |
| Dimir actor usable | 107 | 92 | -15 |
| Usable later-epoch roots | 41 | 35 | -6 |
| `hand_to_battlefield` rejections | 11,048 | 6,668 | -4,380 |
| `hand_to_stack` rejections | 10,625 | 13,034 | +2,409 |
| Total prefix rejections | 29,421 | 28,715 | -706 |
| Total elapsed | 114.186 s | 125.395 s | +11.209 s |
| Allocations after corpus load | 83,695,350,608 B | 101,631,260,040 B | +17,935,909,432 B |

The target bucket fell 39.6%, but more proposals advanced far enough to reject
at later hand-to-stack and event transitions. More importantly, the mixture's
nonuniform exact weights reduced ESS enough to lose seven usable roots despite
682 additional accepted attempts. This is why raw acceptance alone is not the
success criterion.

## ESS-loss diagnosis and bounded weight-cap prototypes

A root-by-root comparison found 36 newly covered roots and 43 roots lost from
the prior history-conditioned proposal. Of the 43 losses, 15 had fewer than
four accepted proposals; 28 still had at least four and failed only ESS. Across
all 369 land-mixture fallbacks, 319 had fewer than four acceptances and 50 had
at least four acceptances but ESS below four.

Identity-free accepted-weight diagnostics reproduced every pre-existing
non-timing result exactly in
`/tmp/gorge-searchprobe-weightdiag-500-20260917.json`. For the 28 previously
covered roots lost only to ESS, accepted outside-isolation worlds averaged
2.07 attempts but carried 85.4% of normalized mass and 98.8% of squared-weight
mass. Every one of all 50 ESS-only fallbacks assigned more than 90% of its
squared-weight mass to outside-isolation worlds. The highest-value cause is
therefore the fixed mixture's unbounded inside/outside importance ratio, not a
shortage of accepted proposals on those roots.

The full fixed 500-root causal census is
`/tmp/gorge-searchprobe-stackcensus-500-20260917.json`; every pre-existing
non-timing result matches the land-isolation artifact after removing only the
new diagnostic fields. It closes all 13,034 `hand_to_stack` rejections:
6,175 hypothetical-extra-cast divergences where the observed action was pass,
5,738 different-name cast competitions, 1,118 missing observed casts, and only
3 observer-reference novelties. All occurred in main phase 1. Of the 6,859
rejections where the observation cast a spell, 6,686 (97.5%) already had a
supported public deadline and only 173 were in an unguided epoch. The bucket
affected 409 roots, including 303 fallbacks and 182 roots with zero accepted
proposals. This does not support adding spell isolation as the next automatic
analogue of land isolation: the dominant problem is frozen-policy action
divergence under different information-consistent hands, not reference mapping
or absent public constraints.

Two exact count-adaptive mixtures were tested on fresh development seeds
10500--10624. Fixed 50/50 covered 37/125 with 876 accepted proposals and 10
ESS-only fallbacks. A 2:1 importance-ratio cap covered 36/125 with 733 accepted
and no ESS-only fallbacks. A 4:1 cap covered 37/125 with 708 accepted and two
ESS-only fallbacks, gaining seven roots and losing seven. Neither improved net
coverage, so both probability changes were reverted and no holdout was spent.
The exact designs and artifact paths are recorded in
`docs/superpowers/specs/2026-09-17-search-land-mixture-weight-cap-design.md`.

Cost after the counter-reuse/early-pruning refactor was 0.871 seconds corpus
load and 125.395 seconds total. Per-root sampler elapsed under five-worker
contention was 680.889 / 1,393.170 / 5,916.048 ms at p50/p95/max and summed to
384.582 worker-seconds. Covered-root search elapsed was 51.873 / 83.406 /
109.584 ms and summed to 7.302 worker-seconds. End HeapAlloc/HeapSys were
4,528,064 / 234,323,968 bytes. A pre-refactor artifact at
`/tmp/gorge-searchprobe-land-isolation-500-20260917.json` produced identical
non-timing per-root results but took 327.833 seconds and allocated
370,156,069,016 bytes; it is not the final calibration artifact.

## Post-rebase fixed census

The six branch commits were rebased onto `origin/main` `0695432` on
2026-09-17. The rebased local tip was `7c699ce` before restoring the unstaged
diagnostic work. The only rebase conflict was the engine constructor: the
resolution retains upstream's livelock watcher and uses the caller-supplied RNG
required by hypothetical construction. No commit or push followed the rebase.

The same fixed 500-game command above was rerun to a new exclusive artifact:

```text
/tmp/gorge-searchprobe-stackcensus-rebased-500-20260917.json
```

The run completed all 500 games with zero experiment errors and zero sampling
budget exhaustions. It covered 112 of 499 eligible roots; the other 387
eligible roots used explicit baseline fallback. Seed 10307 produced no eligible
turn>=5 root, so it ran no sampled or terminal-outcome arms. The run accepted
2,588 of 31,936 proposals, prefix
rejected 29,198, and found 150 incompatible proposals. Death-n-taxes actor
coverage was 39/250 and Dimir actor coverage was 73/250. It covered 20 of 263
later-epoch roots. Land isolation recorded 31,366 eligible attempt-shuffles,
15,786 isolation selections, 34 empty-isolation fallbacks, and 93 unsupported
public shapes. All 500 baseline replays, 112 one-world replays, 448 four-world
replays, and all 1,996 terminal outcome replays succeeded.

The post-rebase `hand_to_stack` total is 11,776: 5,781 hypothetical extra casts
against an observed pass, 4,931 different-name cast competitions, 1,061
missing observed casts, and 3 observer-reference novelties. All remain in main
phase 1. Of the 5,995 observed-cast mismatches, 5,973 had a supported public
deadline and 22 were unguided. The bucket affected 405 roots, including 312
non-covered roots and 199 roots with zero accepted proposals.

This is not directly comparable as a sampler-only rerun. After removing
`SampleNS`, `SearchNS`, `WeightDiagnostics`, `HandToStackCauses`, and
`StackRejectionContexts`, only 28/500 per-root records matched the pre-rebase
stack-census artifact exactly. Baseline heads changed for 459 roots, root
positions changed for 160, and pre-existing sampling fields changed for 309.
Coverage gained 16 roots and lost 35, a net change from 131 to 112. Of the 35
losses, 31 accepted fewer than four proposals and four accepted at least four
but failed only the ESS >=4 gate. The 82 upstream commits after the branch fork
changed the engine and bot policy, so the earlier artifact remains the
pre-rebase checkpoint and this artifact is the new-engine census.

The post-rebase one-worker prefix is
`/tmp/gorge-searchprobe-stackcensus-rebased-125-resume-20260917.json`. It
completed 125 games with zero errors and covered 24 roots. Every per-game field
matches the first 125 five-worker records after removing only `SampleNS` and
`SearchNS`; the `jq -e --slurp` comparison returned `true`. Worker allocation
therefore does not affect the deterministic experiment result.

Seed 10307 is a genuine no-root game under the current manual-mana protocol,
not a sampling fallback or an engine defect. The bot explicitly tapped Island
for blue, played Underground Sea, and explicitly selected blue again, leaving
`UU` in its pool. Its black spells were therefore not payable and were not
presented as cast options; it passed instead. The engine does not plan an
alternative sequence of mana activations to advertise a future cast. New
artifacts make this distinction explicit with per-game `NoRootReason` and
top-level `EligibleRoots`/`NoRootGames` accounting; the existing artifact is
reported as 112/499 eligible roots plus one no-root game.

Focused search-probe tests and `go vet ./...` passed, and `git diff --check`
was clean. `go test ./... -count=1` is red on two tests that reproduce
unchanged on a clean detached `origin/main` worktree:
`cmd/botbench.TestConstructedDefaultIsByteIdentical` measures 18/2 against its
16/4 golden, and `host.TestStallGuardSetToZeroDoesNotHalt` does not reach 200
decisions before its 30-second deadline. They are upstream baseline failures,
not introduced by the rebased search-probe changes; no unrelated golden or
host behavior was changed here.

## Land-isolation reversion checkpoint

The fixed 50/50 land-isolation proposal was removed from the active sampler
after its negative coverage result. Its design, tests, measurements, and
artifacts remain documented above as experiment evidence. The active proposal
again uses only the history-conditioned position and cumulative lower-deadline
constraints. The independent weight-distribution and `hand_to_stack` cause
diagnostics remain enabled.

Fresh post-rebase artifacts for the reverted proposal are:

```text
/tmp/gorge-searchprobe-post-land-revert-500-20260917.json
/tmp/gorge-searchprobe-post-land-revert-125-20260917.json
```

The 500-game run completed with zero errors and zero budget exhaustions. It
covered 114/499 eligible roots, with one explicit no-root game, and accepted
1,999 of 31,936 attempted proposals. It recorded 29,823 prefix rejections and
114 incompatible proposals. Actor coverage was 29 Death-n-taxes and 85 Dimir;
26/263 later-epoch roots were covered. All 500 baseline replays, 114 one-world
replays, 456 four-world replays, and 1,996 terminal outcomes succeeded.

Against the post-rebase land-isolation artifact, removing the mixture changed
coverage from 112/499 to 114/499: 40 roots were gained and 38 lost. Raw
acceptance fell from 2,588 to 1,999. This supports removing the fixed mixture,
but the root churn and two-root net change are coverage evidence only, not a
strength claim.

The one-worker 125-game prefix covered 26 roots with zero errors and matched
the first 125 five-worker records exactly after deleting only `SampleNS` and
`SearchNS`. The comparison returned `true`.

## Ten-iteration CPU/allocation optimization

The post-land-reversion probe was profiled and optimized in ten alternating
CPU/allocation iterations. Manual mana activation and every sampler semantic
boundary stayed unchanged. The final JSON matches the baseline in every field
after deleting only the five top-level timing/memory fields and per-result
`SampleNS`/`SearchNS`; the exact `jq -e` comparison returned `true`.

| Measurement | Baseline | After iteration 5 | Final | Baseline to final |
|---|---:|---:|---:|---:|
| Wall time | 114.862 s | 93.098 s | 86.275 s | -24.9% |
| Profiled CPU | 558.42 CPU-s | 448.65 CPU-s | 405.30 CPU-s | -27.4% |
| Runtime allocated bytes | 89,925,803,696 | 62,731,002,760 | 58,309,831,952 | -35.2% |
| Sampled `alloc_space` | 83.98 GiB | 58.70 GiB | 54.65 GiB | -34.9% |
| Sampled allocated objects | 1,118,836,403 | 507,567,642 | 502,265,036 | -55.1% |
| End `HeapAlloc` | 16,226,304 B | 22,881,584 B | 18,336,000 B | +13.0% |
| End `HeapSys` | 238,551,040 B | 246,906,880 B | 255,328,256 B | +7.0% |

The end-of-run live-heap figures are not peak RSS and are GC-timing-sensitive.
The final sampled `inuse_space` was only 2 MiB: 1 MiB runtime thread storage,
0.5 MiB the JSON type cache, and 0.5 MiB runtime scavenger state. The material
heap problem remains allocation churn, not retained application objects.

The ten iteration outcomes were:

1. CPU: precomputed factorials in each constraint counter; retained.
2. Allocation: immutable memoized `big.Int` values removed defensive copies;
   retained. After the first two changes the 60-card counter moved from about
   106.9 ms / 30.74 MB / 726,875 allocations to 79.5 ms / 19.68 MB / 490,548.
3. CPU: compact unambiguous varint memo keys; retained, reaching about 70.4 ms /
   17.91 MB / 380,121 allocations.
4. Allocation: recursive count vectors now decrement/restore in place;
   retained, reaching about 59 ms / 13.18 MB / 281,478 allocations. A discovered
   deadline-baseline alias was fixed by copying once at the `total` boundary.
5. CPU: equivalent public constraint plans are compiled once per sample and
   reused across attempts; retained. A fresh steady-state benchmark measured a
   median 0.188 ms / about 8.5 KB / 574 allocations per cached unranking. The
   exact midpoint run was already -18.9% wall, -19.7% CPU, and -30.2% runtime
   allocation versus baseline.
6. Allocation: opponent bot boards reuse per-player maps; retained. Midpoint to
   final profile subtraction removes 3.60 GiB and about 5.65 million objects
   from `botpolicy.NewBoard`.
7. CPU: `TurnsTaken` uses an engine-local, epoch-validated derived cache that is
   rebuilt for external history and independently cloned; retained.
   `TurnsTaken` fell by 30.48 cumulative CPU-seconds from the midpoint profile.
8. Allocation: hypothetical logs reserve the exact observed-prefix event
   count; retained as a mixed trade. It moved about 2.00 GiB out of
   `events.growEvents` into 2.13 GiB of one-shot `Log.Reserve` allocation, so it
   is allocation-neutral/slightly negative at heap-profile sampling precision,
   while the second-half profile also removed 31.79 CPU-seconds of
   `runtime.duffcopy`. It is not counted as a heap win.
9. CPU: unrelated events bypass granted Ward/Dethrone checks; retained.
   Midpoint subtraction shows 3.10 CPU-seconds removed from Ward checks and
   1.91 CPU-seconds from Dethrone checks; trigger traversal order and LKI/Room
   behavior remain covered.
10. Allocation: `Collector.Capture` reuses its redacted-event scratch while
    returning independently owned frames; retained. The focused benchmark went
    from 44 allocations / about 20.38 KB to 43 allocations / about 17.30 KB per
    capture, and final-profile subtraction removed 0.51 GiB flat allocation
    from `Capture`.

The largest remaining final CPU costs are engine traversal rather than the
constraint solver: `checkFaceTriggers.func1` is 22.24 CPU-s flat / 73.08 s
cumulative, `Object.Face` is 13.45 s flat, `faceMayTrigger` is 7.57 s flat /
14.19 s cumulative, and `forEachObject` is 86.86 s cumulative. At broader
boundaries, `Engine.Submit` is 271.61 s cumulative, `Advance` 176.32 s,
`Sample` 147.71 s, `emit` 119.57 s, `legalActions` 114.64 s, and
`verifyActual` 106.15 s. `runtime.duffcopy` remains the largest individual flat
runtime node at 23.67 s.

The largest final allocation sites are `livelockWatcher.observe` 4.97 GiB,
`Game.AddObject` 4.75 GiB, `events.growEvents` 4.59 GiB, `json.Marshal`
4.03 GiB, `view.cardViews` 3.00 GiB, `Collector.cards` 2.95 GiB,
`legalActions.func1` 2.71 GiB, `strings.Replacer.build` 2.31 GiB,
`Collector.Capture` 2.21 GiB flat / 15.14 GiB cumulative, and `Log.Reserve`
2.13 GiB. By object count the leading sites are `utf8.AppendRune` 72.7 million,
`splitCostTokens` 54.6 million cumulative 91.5 million, `derivedWith` 37.4
million, `strings.genSplit` 34.1 million, and `strings.Fields` 19.0 million.

Artifacts:

```text
baseline JSON: /tmp/gorge-searchprobe-post-land-revert-profiled-500-20260917.json
baseline CPU:  /tmp/gorge-searchprobe-post-land-revert-cpu-500-20260917.pprof
baseline heap: /tmp/gorge-searchprobe-post-land-revert-heap-500-20260917.pprof
midpoint JSON: /tmp/gorge-searchprobe-profile-midpoint-500-20260917.json
midpoint CPU:  /tmp/gorge-searchprobe-profile-midpoint-cpu-500-20260917.pprof
midpoint heap: /tmp/gorge-searchprobe-profile-midpoint-heap-500-20260917.pprof
final JSON:    /tmp/gorge-searchprobe-profile-final-500-20260917.json
final binary:  /tmp/gorge-searchprobe-profile-final-bin-20260917
final CPU:     /tmp/gorge-searchprobe-profile-final-cpu-500-20260917.pprof
final heap:    /tmp/gorge-searchprobe-profile-final-heap-500-20260917.pprof
CPU SVG:       /tmp/gorge-searchprobe-final-cpu.svg
CPU diff SVG:  /tmp/gorge-searchprobe-cpu-diff.svg
alloc SVG:     /tmp/gorge-searchprobe-final-alloc-space.svg
alloc diff:    /tmp/gorge-searchprobe-alloc-space-diff.svg
live heap SVG: /tmp/gorge-searchprobe-final-inuse-space.svg
```

Focused package tests, full `go vet ./...`, and `git diff --check` passed after
iteration ten. No race suite, golden regeneration, commit, or push was run.

## Engine-only profile optimization pass

This second pass starts from the immutable final artifacts above and changes
production code only under `rules/`, `effects/`, `events/`, and `state/`.
The baseline is 86.275419953 seconds wall, 405.30 CPU-seconds,
58,309,831,952 runtime-allocated bytes, 54.65 GiB sampled `alloc_space`, and
502,265,036 sampled allocated objects. Every retained iteration must preserve
the exact non-timing JSON comparison described in the engine resume report.

| Iteration | Axis | Hypothesis and focused result | Decision | Correctness / cross-axis |
|---:|---|---|---|---|
| 1 | CPU | Replace the pointer-hash lookup for immutable trigger eligibility inside the deterministic object walk with an engine-owned dense ObjID/two-face cache. `BenchmarkFaceTriggerScanDistinctFaces` (240 distinct faces, five runs) improved from median 5,836 ns/op to 3,481 ns/op (-40.4%). | Retain | Both sides were 0 B/op and 0 allocs/op. The new test failed first because `objectFaceMayTrigger` did not exist, then passed; focused trigger eligibility, granted Ward, Room/face, phase diagnostic, traversal, and clone-independence tests pass. Files: `rules/trigger_eligibility.go`, `rules/trigger_match.go`, `rules/engine.go`, `rules/clone.go`, and focused tests. |
| 2 | Allocation | `livelockWatcher.observe` repeatedly lost slice capacity after trimming its 128-event diagnostic window, and interface-based FNV hashing escaped. Bounded rings plus a stack-local byte-identical FNV-1a accumulator changed `BenchmarkLivelockWatcherSteadyAperiodic` from median 543.5 ns/op, 257 B/op, 2 allocs/op to 496.1 ns/op (-8.7%), 0 B/op, 0 allocs/op. | Retain | The allocation test first failed at 2.00 allocations/event and now passes. Existing exact-cycle, drifting-payload, runaway, progress-reset, diagnostic-order, and clone-independence tests pass. Ring storage is bounded by `2*MaxPeriod` / `MaxPeriod`; no live-heap growth. File: `rules/livelock.go` and focused tests. |
| 3 | CPU | `effects.ColorsOf`, 23.95 cumulative CPU-seconds under `derivedWith`, rebuilt a map and strings for a five-value domain on every read. A local WUBRG bitmask and immutable 32-entry result table changed `BenchmarkColorsOf` (mana, hybrid, explicit-color, Devoid, colorless) from median 1,243 ns/op to 143.4 ns/op (-88.5%). | Retain | Allocation also fell from 56 B/op and 4 allocs/op to zero. The allocation regression failed first at 2 allocations for an explicit-color face; exact color cases and the full `effects` suite pass. No parsed card metadata or process-global mutable cache was introduced. File: `effects/colors.go` and focused tests. |
| 4 | Allocation | Prototype substring tokenization changed a six-token microbenchmark from median 1,492 ns/op, 416 B/op, 14 allocs/op to 491.5 ns/op, 128 B/op, 1 alloc/op. The first midpoint profile exposed an unrepresentative capacity choice: common one-token costs paid for an eight-entry result slice, and sampled `splitCostTokens` space rose from 1.24 GiB to 4.23 GiB. | Revert | Focused semantics passed and object count fell, but the real workload materially regressed the stated allocation-space target. Production code and its implementation-specific ceiling were removed before iteration 6; the benchmark remains. |
| 5 | CPU | Full layer evaluation converted `ColorsOf` back into booleans and rebuilt its result by string concatenation. Carrying an exported typed five-bit `effects.ColorMask` through the layer pass and rendering through immutable mask strings changed `BenchmarkDerivedWithContinuousEffects` from median 1,050 ns/op to 918 ns/op (-12.6%). | Retain | Warm allocation fell from 12 B/op and 3 allocs/op to zero. The allocation regression failed first at 3 allocations; exact color, derived-characteristic, continuous-effect invalidation, and focused `effects`/`rules` tests pass. Files: `effects/colors.go`, `rules/layers.go`, and focused tests. |
| 6 | Allocation | The retained midpoint restored the original tokenizer after iteration 4's rejection; it was again the largest allocation-object source (52.57 million flat, 88.63 million cumulative). A stack-local UTF-8 iterator now feeds `ParseCost`, strict unless-cost parsing, and static cost raises without materializing token strings or a result slice. `BenchmarkParseCostHotShapes` changed from median 650.3 ns/op, 83 B/op, 4 allocs/op to 433.6 ns/op (-33.3%), 27 B/op (-67.5%), and 0 reported allocs/op. | Retain | The plain-mana allocation test failed first at 4 allocations and now passes at zero. Focused parse, token-family, modifier, and raise tests pass. Unlike iteration 4, production callers allocate no fixed-capacity token slice; `splitCostTokens` remains only as a compatibility/test helper. Files: `rules/mana.go`, `rules/statics.go`, and focused tests. |
| 7 | CPU | The midpoint attributed 4.77 flat CPU-seconds inside `objectFaceMayTrigger` to checking `len(f.Triggers)` before the dense-cache hit. Caching the zero mask for triggerless faces moves that branch to the cold miss. A five-run, two-second, single-worker comparison changed the 240-face scan median from 3,509 ns/op to 3,266 ns/op (-6.9%). | Retain | Both sides remain 0 B/op and 0 allocs/op. The existing cache test was written before this representation, and focused face-change, Room, granted-keyword, diagnostic-order, traversal, and clone-independence tests pass. File: `rules/trigger_eligibility.go`. |
| 8 | Allocation | The strict unless-cost parser rebuilt the same brace `strings.Replacer` per call, matching the midpoint's 2.45 GiB `strings.Replacer.build` site. Reusing `ParseCost`'s immutable package-level normalizer changed `BenchmarkParseUnlessCostBraced` from median 898.7 ns/op, 416 B/op, 5 allocs/op to 356.3 ns/op (-60.4%), 32 B/op (-92.3%), 2 allocs/op (-60%). | Retain | The allocation ceiling failed first at 5 and now passes at the two allocations required by normalized output ownership. Strict parsing and unless-payment focused tests pass; `strings.Replacer` is concurrency-safe after construction. File: `rules/mana.go` and focused tests. |
| 9 | CPU | `derivedWith` resolved the same object/face and entered the same active-effect cache once through `derivedScalar`, then again for the full keyword/type/color pass. Sharing one resolved object, face, and active slice changed the five-run, two-second benchmark median from 736.8 ns/op to 685.0 ns/op (-7.0%). | Retain | Both sides remain 0 B/op and 0 allocs/op. Derived characteristics, continuous-effect invalidation, end-of-turn cleanup, and source-departure tests pass. File: `rules/layers.go` and focused tests. |
| 10 | Allocation | Every engine genesis knows its exact configured deck-card count, but `Game.Objs` grew geometrically through 240 `AddObject` calls. Supplying that capacity at construction changed the four-seat/240-card `BenchmarkNewEngineObjectArena` median from 493,633 ns/op, 338,588 B/op, 143 allocs/op to 427,114 ns/op (-13.5%), 191,708 B/op (-43.4%), 135 allocs/op (-5.6%). | Retain | The exact-capacity test failed first at cap 292 and now passes at len/cap 240 while checking every dense ID. Focused constructor, clone, state, and chain-head tests pass. Capacity affects backing storage only; authoritative game fields still change through `events.Apply`, and later token creation grows normally. Files: `state/game.go`, `rules/engine.go`, and focused tests. |

The retained iteration-5 midpoint artifacts are
`/tmp/gorge-searchprobe-engine-midpoint-retained-{500-20260917.json,bin-20260917,cpu-500-20260917.pprof,heap-500-20260917.pprof}`.
The exact non-timing comparison returned `true`: 500 games, 499 eligible
roots, one no-root game, 114 covered roots, and zero errors. It measured
73.382243556 seconds wall, 356.68 CPU-seconds, 53,115,788,624 runtime-allocated
bytes, 49.85 GiB sampled `alloc_space`, 409,718,709 sampled allocated objects,
19,998,624 B end `HeapAlloc`, and 238,518,272 B end `HeapSys`. The earlier
`engine-midpoint` artifact includes rejected iteration 4 and is diagnostic
evidence only.

### Engine-pass final result

All ten experiments are complete. Nine changes were retained and iteration 4's
fixed-capacity tokenizer was rejected and reverted after the real workload
showed that it traded fewer objects for substantially more allocation space.
The final artifact remains byte-for-byte equivalent in every non-timing field:
the prescribed `jq -e --slurp` comparison against the immutable baseline,
deleting only the five top-level timing/memory fields and each result's
`SampleNS`/`SearchNS`, returned `true`.

| Measurement | Immutable baseline | Retained midpoint | Final | Baseline to final |
|---|---:|---:|---:|---:|
| Wall time | 86.275419953 s | 73.382243556 s | 69.463003930 s | -19.5% |
| Profiled CPU | 405.30 CPU-s | 356.68 CPU-s | 338.66 CPU-s | -16.4% |
| Runtime allocated bytes | 58,309,831,952 | 53,115,788,624 | 49,603,764,312 | -14.9% |
| Sampled `alloc_space` | 54.65 GiB | 49.85 GiB | 46.36 GiB | -15.2% |
| Sampled allocated objects | 502,265,036 | 409,718,709 | 317,072,050 | -36.9% |
| End `HeapAlloc` | 18,336,000 B | 19,998,624 B | 17,349,376 B | -5.4% |
| End `HeapSys` | 255,328,256 B | 238,518,272 B | 242,712,576 B | -4.9% |

The final result is still 500 games, 499 eligible roots, one no-root game, 114
covered roots, and zero errors. Midpoint to final improved wall time another
5.3%, CPU 5.1%, runtime allocation 6.6%, sampled allocation space 7.0%, and
sampled object count 22.6%. End-of-run heap values remain GC-timing-sensitive;
the final sampled live heap was 1 MiB, entirely `runtime.allocm`, so the
measured improvement is allocation churn rather than evidence about peak RSS.

The largest remaining final CPU nodes are
`checkFaceTriggers.func1` at 22.06 CPU-s flat / 59.32 s cumulative,
`runtime.duffcopy` at 21.87 s flat, `Object.Face` at 14.52 s flat,
small-string map lookup at 6.46 s flat / 13.29 s cumulative,
`objectFaceMayTrigger` at 5.88 s flat / 6.88 s cumulative, `Game.Obj` at
5.60 s flat, and `forEachObject` at 4.60 s flat / 72.73 s cumulative.
Broader overlapping paths are `Engine.Submit` at 226.23 s cumulative,
`Advance` at 141.05 s, `emit` at 101.79 s, `legalActions` at 89.87 s,
`checkTriggers` at 65.84 s, and `derivedWith` at 26.65 s. These cumulative
figures overlap and must not be added.

The largest final allocation-space sites are `events.growEvents` 4.52 GiB,
`json.Marshal` 4.04 GiB, `view.cardViews` 3.05 GiB, `Collector.cards` 2.92 GiB,
`legalActions.func1` 2.68 GiB, `strings.Replacer.build` 2.38 GiB,
`Collector.Capture` 2.18 GiB flat / 14.80 GiB cumulative, `Log.Reserve`
2.07 GiB, `state.NewGameLife` 1.93 GiB, `Game.AddObject` 1.26 GiB, and
`livelockWatcher.observe` 1.11 GiB. Against the immutable baseline, the
largest attributable removals were 3.96 GiB flat from livelock observation,
3.57 GiB from geometric object-arena growth, 1.27 GiB flat / 1.85 GiB
cumulative from `splitCostTokens`, 1.12 GiB from `utf8.AppendRune`, and
0.57 GiB flat / 1.13 GiB cumulative from `derivedWith`; the exact initial
arena instead appears as a 1.93 GiB one-shot `NewGameLife` allocation.

By allocated objects the remaining leaders are `strings.genSplit` 32.86
million, `strings.Fields` 17.94 million, `view.cardView` 16.55 million flat,
`Cost.costPips` 16.35 million, `paymentConv` 13.55 million,
`strings.Builder.grow` 12.80 million, `cards.SplitKeywordList` 12.76 million
flat / 18.27 million cumulative, `attachmentSBAs` 10.81 million,
`checkSagas` 8.98 million, and `activeStatics` 7.60 million. The remaining
string/map work and the dominant trigger/object traversal both point at the
same next architectural target: immutable compiled `cards.Face` metadata.

Final artifacts:

```text
/tmp/gorge-searchprobe-engine-final-500-20260917.json
/tmp/gorge-searchprobe-engine-final-bin-20260917
/tmp/gorge-searchprobe-engine-final-cpu-500-20260917.pprof
/tmp/gorge-searchprobe-engine-final-heap-500-20260917.pprof
```

Focused package tests, `go vet ./...`, and `git diff --check` passed before the
final profile. The required full suite reproduced the two known clean-main
failures: `cmd/botbench.TestConstructedDefaultIsByteIdentical` remains 18/2
versus its 16/4 golden, and `host.TestStallGuardSetToZeroDoesNotHalt` again
missed 200 decisions in 30 seconds. It also hit one asynchronous
`host.TestUndoStreamReceivesRewindThenConsistentFrames` assertion once: both
the drained pre-undo and rewind heads were 36. The unchanged host test passed
20/20 focused reruns, consistent with its subscriber-drain timing race; no host
or golden behavior was changed.

### Next task: compiled face metadata and an accelerator boundary

The next pass should move immutable, repeatedly parsed `cards.Face` syntax into
compiled metadata. This is not a blanket replacement of strings. Closed
domains should become typed integer-backed values: face-local stable IDs,
card/supertype masks, common-keyword masks, SA-kind/API opcodes, trigger-mode
and replacement-event opcodes, colour masks, boolean flag masks, and compiled
cost/filter programs. Open-ended Forge values, names, text, unknown parameters,
and diagnostics must retain a deterministic string fallback so new corpus
syntax fails closed or remains reportable rather than aliasing an enum zero.

The CPU representation should already be accelerator-shaped: immutable flat
tables with integer IDs plus offset/count spans for abilities, triggers,
statics, replacements, parameters, and bytecode operands. A versioned schema
and deterministic compiler order are required; runtime-assigned indexes must
never enter events or replays without the corpus/schema identity that gives
them meaning. Engine-local mutable state continues to reference immutable face
rows but remains clone-independent.

That layout is useful for a future CUDA tier because it can be copied as
pointer-free structure-of-arrays buffers and evaluated in batches without Go
maps, strings, or pointer chasing. Good first accelerator candidates are pure
candidate predicates, trigger preselection, and cost/mana feasibility over a
snapshot. Authoritative event mutation, ordering, resolution continuations,
RNG, and validation stay on the CPU. Because the pure-Go core forbids cgo and
third-party dependencies, any CUDA runtime belongs behind an optional external
adapter/process or later plugin boundary; the core should expose versioned
plain-data snapshots and retain the CPU evaluator as the correctness oracle.
