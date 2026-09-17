# Search-probe profile optimization design

## Goal

Run ten measured optimization iterations against the post-land-revert search
probe, split into five CPU-led and five allocation-led iterations, then rerun
the exact 500-game CPU/heap profile and report the cumulative result.

## Baseline

The immutable comparison artifacts are:

- `/tmp/gorge-searchprobe-post-land-revert-profiled-500-20260917.json`
- `/tmp/gorge-searchprobe-post-land-revert-cpu-500-20260917.pprof`
- `/tmp/gorge-searchprobe-post-land-revert-heap-500-20260917.pprof`

The profiled baseline is 114/499 covered roots, one no-root game, zero errors,
114.862 seconds wall time, 558.42 CPU-seconds, and 89,925,803,696 bytes
allocated by `runtime.MemStats` (83.98 GiB sampled alloc_space).

## Method

There are ten total iterations, alternating CPU-led and allocation-led work.
Each iteration has one explicit hypothesis, a targeted before/after benchmark,
and deterministic correctness verification. A change that does not improve its
target, changes any non-timing result, or materially regresses the other axis
is reverted and recorded as a rejected iteration. Rejected experiments count
toward the ten because they are measured optimization iterations, not shipped
changes.

The first five iterations address exact constrained-permutation counting. The
500-game profiled protocol is rerun at the midpoint to re-rank the remaining
hotspots. The final five address the highest surviving engine/reconstruction
costs. The exact 500-game profiled protocol is rerun after iteration ten.

## Initial hotspot ladder

1. CPU: cache factorial values inside each constraint counter.
2. Allocation: make memoized `big.Int` results immutable and remove defensive
   copies on memo hits and insertion.
3. CPU: replace decimal/string-builder memo keys with compact binary keys.
4. Allocation: mutate and restore recursive count vectors instead of cloning
   them for each branch.
5. CPU: reuse equivalent compiled constraint counters across attempts.
6. Allocation: reuse `botpolicy.Board` buffers during hypothetical replay.
7. CPU: cache per-player turn counts instead of scanning the entire event log.
8. Allocation: reserve hypothetical event-log capacity from the observed
   prefix length.
9. CPU: reduce trigger prefilter work without changing trigger order or event
   semantics.
10. Allocation: remove the largest remaining observation/replay temporary
    identified by the midpoint heap profile.

Iterations 9 and 10 deliberately name behavioral boundaries rather than a
specific representation: the midpoint profile selects the implementation, but
the acceptance criteria below do not change.

## Correctness boundaries

- All non-timing JSON fields must match the baseline after deleting only
  top-level timing/memory fields and per-result `SampleNS`/`SearchNS`.
- Proposal densities, RNG draw order, accepted worlds, ESS, replay heads, and
  rejection diagnostics remain byte-identical.
- Manual mana tapping remains unchanged.
- All `state.Game` mutation continues through `events.Apply`.
- No Forge scripts, third-party dependencies, commits, pushes, race tests, or
  golden regeneration.

## Measurements and deliverables

Targeted benchmarks record wall time, allocations/op, allocated bytes/op, and
the relevant result digest. Midpoint and final runs use exactly:

```sh
GOMAXPROCS=5 GOMEMLIMIT=5GiB searchprobe \
  -games 500 -workers 5 -seed 10000 -sample-seed 54321 \
  -attempts 64 -worlds 4 -max-submits 5000 \
  -cpuprofile CPU -memprofile HEAP -out REPORT
```

The final report lists all ten iterations, including rejected changes, and
compares wall time, CPU-seconds, total allocation, live heap, coverage, and
the largest CPU/alloc_space nodes against iteration zero.
