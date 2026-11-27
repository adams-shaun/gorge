# dirac: per-decision manaPCGen memo (static-source cache)

Status: implemented with its shadow check (`dirac/mana_pcmemo.go`).
Citations are `path:line` at base `ca64c6c095` unless marked "head".

## Problem

`BenchmarkTransitionEDH4` (`dirac/benchedh_test.go`) plays 16 four-seat 100-card
Commander pods. A CPU profile at base (200000x, GOMAXPROCS=1) puts 70% of the time
in `priorityOptions` (`dirac/legal.go:65`). The largest single cost under it is
`manaPCGen` (`dirac/gen.go:2937`) at 33% cumulative. That function gives the planner's
view of one battlefield slot as a mana source. It re-derives the view from the static
sources on every call: granted abilities (`grantIdx`, 1.0 s of 12.4 s), land-type
grants (`blgView`, `dirac/static_landtype.go:343`), reflected colours (`manaReflView`,
`dirac/mana_reflected.go:293`), gates, and `CantBeActivated` locks.

One priority walk asks for the same slot many times. The callers are:

- the census walks (`availForMode`), under each `availFor` mode;
- the nested plain re-walks (`plainOptions`, `dirac/script_gen.go:212`);
- the filter checks (`filtOtherMana`);
- the tap probes (`canTapForMana`);
- the planner reads (`manaPCAt`, `dirac/cost_disjointmana.go:191`).

None of these change the inputs of the view.

## What is cached

The memo caches `manaPCGen`'s answer for each slot. That answer is a `*pcard` into
the table's prep, never into State. The memo stores it as a place, not a pointer:
`(pcTab, idx)`, which names one of `k.cards`, `varPC`, `grantPC`, `selfPC`, `blgPC`,
`altPC`, `gatePC` or `setPC`, or `&noManaPC` (`dirac/gen.go:665`).

`pcPlace` finds that place by comparing integer addresses. No integer is ever
converted back to a pointer. A view that lies in none of those slices is not
remembered (`pcNone`). A future view arm with a slice of its own is therefore
recomputed, never answered wrongly.

This one memo covers the granted-ability walk and the layer and type matching that
`manaPCGen` does. The brief's separate per-slot static-source bitset and grant index
list are not needed for this path. The per-player `scriptFilt` result is out of
scope: see "Not done".

## Where it lives

The memo lives in `ForkPool.pcm` (head `dirac/statecopy.go`). It is not in State. This
follows `filtMemo` (`dirac/mana_filtmemo.go:22`, `dirac/statecopy.go:96`), the existing
per-transition memo held in the pool. Because of this:

- `TestNoNewFixedStateFields`, `TestStateSize` and `TestColdArraysHoldNoPointers`
  do not move.
- The State stays free of references.

Each entry is 16 bytes and holds no pointers: one `key` word (write count, opening,
mode bits), `idx int32`, `tab uint8`. There is one entry per slot (`MaxPerm` = 192,
`dirac/state.go:16`), so the memo is 3 KB and fits L1.

The memo is allocated with the pool, once per State. `pcMemoOpen` makes the pool if a
State has none, as `fork` does (`dirac/statecopy.go:124`). The transition loop
allocates nothing: the bench's 0 allocs/op holds.

The owner is kept as a `uintptr` identity. A stored `*State` or `*pcard` makes the
prep's and the State's contents escape, for escape analysis. That moved `cost` at
`dirac/cast.go:927` to the heap and cost 6 B/op, so neither is stored.

## Which tables use it

The memo is per table: `prep.pcMemo` (`prepPCMemo`). It is set when some view can be
more than the card's own `pcard`: a granted mana ability, a land-type grant or
setter, a `CantBeActivated` lock, or a variant view (`gfManaVar`).

On other tables a call is cheaper than the memo's bookkeeping. On those, `manaPCGen`
(now an inlined gate in front of `manaPCGenCalc`) costs one flag test, and
`pcMemoOpen` leaves the memo shut. All 16 EDH bench pods use the memo; 6 of the 64
two-seat FDN bench tables do.

A board-size gate (`permHi`) was measured and dropped. It lost most of the EDH win,
because early Commander boards are small, and saved under 0.3% on FDN.

## When an entry answers (invalidation)

An entry answers only when all of the following hold.

1. **The memo is open for this State.** The memo keeps the opener's address as an
   identity and never dereferences it. `stepRaw`'s priority arm (`dirac/engine.go:324`) opens the
   memo around `priorityOptions`, and closes it right after.
   - The transition boundaries close it as well. These are `filtMemoReset` at the top
     of `stepRaw` (`dirac/engine.go:259`) and of `Apply` (`dirac/engine.go:409`), and
     `InitNStart` after `resetCold` (`dirac/engine.go:64`).
   - So a walk that a kernel panic left open never answers past the panic.
   - Opening starts a new generation (`gen`). A wrap clears the table.
2. **The asker is the State that opened it.** In addition:
   - no fork of it is out (pool depth 0);
   - no filter search (`inFilt`) and no census window is running;
   - no in-place probe holds it (`pcMemoHold`).

   A fork never reads or writes the memo, so a fork starts with an empty cache.
3. **The object-write count matches.** The entry's `touch` equals `State.touchN`.
   Every object or slot write bumps this count (`dirac/marks.go:32`;
   `TestMarksCoverEveryMutation` holds every mutation site to it). A walk that
   really changes the game therefore misses.
4. **The `availFor` mode bits match.** The entry's `avx` equals `State.g.availX`
   (`dirac/gen.go:2207`). A view can read the mode, for example
   `mana_condsplit.go:110`.

The kernel has three writes that bypass the object-write count, and the memo handles
each one:

- The filter-bound probe taps the filter in place and restores it without marking
  (`dirac/mana_filter.go:281`). It holds the memo for that span.
- `CopyFrom` into the owner copies `touchN` together with the game. It starts a new
  generation (`pcMemoDrop`) because the count alone cannot tell two games apart.
- In-place probes that restore with `CopyFrom(saved)` take their fork first, so the
  depth check covers them.

The brief planned "one generation counter bumped in the events.Apply paths". dirac has
no `events.Apply`: kernel writes are spread over the package. `touchN` is the existing
counter that every write already bumps, and the memo keys on it. The memo's own `gen`
only scopes it to one walk.

Side effects inside a view are unchanged. A stall latch (`capFull`,
`dirac/gen.go:2897`) is idempotent and set by the first, computed call. Diagnostic
counters such as `BlgStats` are read only by tests, and those tests run with the
shadow check on, which recomputes every view.

## Shadow check

- `DebugPCMemoShadow`: on every hit, the memo recomputes the view with
  `manaPCGenCalc` and panics if the two pointers differ. Pointer identity into prep is
  exact equality.
- `dirac/oracle`'s `TestMain` turns the shadow check on for the whole package. Every
  lockstep game against gorge therefore also proves that the cached path equals the
  uncached one. CI runs this through the merge-queue `oracle-sentinels` set and the
  two-hourly whole `dirac/oracle` run.
- `TestPCMemoShadowEDH` plays the bench pods with the shadow check on, and requires
  that the memo answers.
- `TestPCMemoOffMatchesOn` plays the pods with the memo off and on, and requires the
  same option streams.
- `TestStaticCacheShadowMutation` drops the invalidation (`debugPCMemoNoInval`: no
  generation bump and no write count in the key). The shadow check must then panic.
- `DebugPCMemoOff` turns the memo off.

## Win

The method is the workload bench's A1 row, the same as the dirac gate's step 5
(`scripts/dirac-gate.sh:288`): instructions:u at
500000x minus 100000x, divided by 400000, with GOGC=off and GOMAXPROCS=1 on one
fenced core.

| bench | base `ca64c6c095` | head | change |
|---|---|---|---|
| BenchmarkTransitionEDH4 | 1217197 | 1006674 | -17.3% |
| BenchmarkTransitionFDN | 64480 | 65083 | +0.9% |
| BenchmarkTransitionBLB | 74325 | 74736 | +0.6% |
| BenchmarkTransitionFDNPool | 109080 | 111417 | +2.1% |
| BenchmarkCloneFDN | 366 | 373 | +1.9% |

The two-seat benches pay the inlined gate, the open and close calls at each priority
decision, and the memo on the few FDN tables that hold it. FDN stays within the
gate's limits (`scripts/dirac-gate.sh:47-48`):

- 1.05 times the merge base;
- 1.02 times the program base `INSTR_PB_FDN=64068`, where head is at 1.016.

## Not done

- (Done in the stacked change, `dirac/mana_sfmemo.go`.) The per-player `scriptFilt` mask
  is memoised per seat in `ForkPool.sfm`, under the same scope as `pcm` (same State, no
  fork, no `inFilt`/census, no `pcMemoHold`). Its key is `filtBoundPays`' own key
  (write count, stack depth, the seat's pool, probe, self target, play-anything, line)
  plus the `availFor` bits and the seat's life. Shadow: `DebugSFMemoShadow`; mutation:
  `TestSFMemoShadowMutation`.
- A `typesGen` per-slot memo (11%).
- Relaxing the `inFilt` and census guards.

Each of these would reuse this memo's scope and keys.
