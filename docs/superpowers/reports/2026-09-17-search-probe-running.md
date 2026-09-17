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

Cost after the counter-reuse/early-pruning refactor was 0.871 seconds corpus
load and 125.395 seconds total. Per-root sampler elapsed under five-worker
contention was 680.889 / 1,393.170 / 5,916.048 ms at p50/p95/max and summed to
384.582 worker-seconds. Covered-root search elapsed was 51.873 / 83.406 /
109.584 ms and summed to 7.302 worker-seconds. End HeapAlloc/HeapSys were
4,528,064 / 234,323,968 bytes. A pre-refactor artifact at
`/tmp/gorge-searchprobe-land-isolation-500-20260917.json` produced identical
non-timing per-root results but took 327.833 seconds and allocated
370,156,069,016 bytes; it is not the final calibration artifact.
