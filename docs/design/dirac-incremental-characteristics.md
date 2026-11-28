# dirac: incremental derived characteristics (census and measured ceiling)

Status: measured, not implemented. A types-only prototype was built and shadow-checked.
It is sound on what it covers, but it saves 3.6% of `BenchmarkTransitionEDH4`, and the
ship bar was 5%. This note records the census, the ceilings, and what blocked the
prototype, for the 12-11 decision.
Base: `e62aa28e17`. Every count is on one locked core, and the instruction counts use
the `instr.sh` differencing method (N=20000 and N=100000; FDN 100000 and 500000).

## Problem

dirac derives a permanent's types, P/T, keywords and granted abilities through the
CR 613 layer walk on every ask (`typesGen`, `ptGen`, `kwGen`, `grantedAbilitiesOwn` in
`dirac/chars_gen.go`; the walk is in `dirac/static_layerwalk.go`). On the root State of
`BenchmarkTransitionEDH4`, over 100,000 posed transitions:

| ask | asks | answer differed from the slot's previous answer |
|---|--:|--:|
| `typesGen` | 18.65M | 1,155 (plus 31,192 first asks of a new object) |
| `ptGen` | 1.54M | 9,958 |
| `kwGen` | 924k | 5,270 |
| `grantedAbilitiesOwn` | 19.9M | not counted |

The 18.65M type asks are 180 per transition: 161 from the State that owns the pool,
and 19 that a cache must pass up (in-place probe holds, 6.8 per transition; fork
asks, 12.5 per transition). Of the 161, 161.03 return the slot's previous answer.
Running every `typesGen` ask twice adds 136k instructions per transition. That is the
cost of all type asks: 13.2% of EDH4's 1,027,439.

A per-priority-walk memo was tried earlier (perf-charsmemo, shelved). It had a ceiling
of 5.1% and came out at +1.9% net, because most asks fall outside one walk. So any
cache must live across walks, state-based passes and applies.

## 1. The inputs a derived characteristic can read

This list combines the shelved memo's scratch audit, `sbaFingerprint` /
`sbaBoardObjectOnly` (`dirac/static_sbaskip.go`), the layer walk, and this lane's
reading of the in-place writers.

1. **The slot's own object and permanent:** card, copy-of, override flags and types,
   `perm.gty` (face-down and provenance), counters, attachment, tapped and attacking
   flags, controller, chosen colour or type, and timestamp. Every write to these
   bumps `touchN` and the object's stamp (`marks.go`).
2. **Effect records on the slot:** override records (`fxM` / `ovRec`) and copy
   exceptions. Adding or dropping one touches the slot.
3. **The statics in force** (`statM`) and each source's own fields: its card, its
   controller, what it is attached to, its lost abilities, and its chosen type or
   colour (`AddChosen`).
4. **Filter reads of the candidate's derived characteristics:** types, colours and
   subtypes recurse into the same sources. P/T and keyword predicates reach `ptGen`
   and `kwGen`, and therefore their Dynamic statics.
5. **Conditions** (`Static.Cond2`, `OvGate`) and dynamic values. These can read
   anything: board counts (`CondPresent`), expressions (`VExpr`), life, turn tallies,
   hand and graveyard sizes, and turn, step and active seat.
6. **Emblems** (`g.nemb`, `lfxCollectEmblems`) and the Ring (`ringBearerAt`), both
   outside the objects.
7. **Scratch a walk or probe writes in place, without a touch:**
   - `g.availX`, `g.probe`, `g.tgtSelf`, `g.mayPlayAny`, `g.atkDef`, `sp` and `line`;
   - `ctrlM` withheld in place (`availSansObj`, `cast_tgtkeep.go`, `mana_window.go`);
   - `pfTapped` set in place (`filtBoundPaysWalk`);
   - `perm.chosen` set in place (`askColValue`);
   - the `walkCond` re-entry bit (`lfxCond`);
   - `perm.ovr.walk`, which `refreshTy` rewrites without a touch when the flags and
     `gty` are unchanged;
   - `g.hist` (`hideInflightCast`) and `g.turnF` (`withRootExhaust`).
8. **Non-object game state**, which is what `sbaFingerprint` hashes: turn, step,
   active seat, layer clock, each seat's life and tallies, the floating pool,
   `spent`, `hist`, and the cold banks (contract, atkLast, experience, dungeon, rad).

P/T adds live override records (`livePT`) and Dynamic statics (`VA`/`VB`). Granted
abilities add `g.grantSrc` (the shelved memo's audit).

## 2. Writes by class, per posed transition (EDH4, Mulligans 2, root State)

These counts come from a temporary build that classified every `touch` / `touchSl`
caller. There are 6.88 root touches per transition, plus 0.93 on forks. The "zone
move" row includes `InitNStart`'s deal (1.28 per transition at game starts).

| write class | per transition | on a battlefield object | which static filters in the bench tables read it |
|---|--:|--:|---|
| tap / untap | 1.479 | 1.479 | `FTapped`/`FUntapped` exist in kop. Type statics: none in 16 EDH tables, none in 64 FDN tables. Keyword statics: 1/16 EDH (Paradise Druid). |
| damage | 0.195 | 0.195 | `F2WasDamaged`/`F2DamagedBy` exist. No characteristic static in either set reads them. |
| counters | 0.057 | 0.055 | Type statics in 11/16 EDH tables (Station: `PredCountersGE` on `FSelf`) and Kazandu Tuskcaller (a level count). Keyword statics (filter programs) in 14/16. |
| zone move (leave, draw, stack) | 2.909 | 0.174 | Through conditions only: graveyard and hand counts, imprint (Duplicant). |
| zone enter | 0.899 | 0.295 | Every class: a new permanent or a new source. |
| effect record | 0.259 | 0.233 | Every class. |
| combat (attack, block) | 0.402 | 0.402 | Type statics: none. Keyword statics: 2/16 EDH (Crossway Troublemakers, Tromokratis). P/T statics: 1/16 (Crescendo of War). |
| activation / turn mark | 0.231 | 0.230 | None. |
| control | 0.004 | 0.004 | Every class (`Ctrl` filters). |
| attach | 0.002 | 0.002 | Attachment filters: types 2/16 EDH and 18/64 FDN; P/T 16/16; keywords 15/16; grants 7/16. |
| face / transform | 0.003 | 0.002 | Every class. |
| other (`exec`, `execGen`, payment sites) | 0.446 | 0.308 | Unclassified, so treated as affecting everything. |

The per-table census of what each kind of static's filters and conditions read:

| | EDH pods (16) | FDN bench tables (64) |
|---|---|---|
| type statics, any | 13 | 25 |
| type statics with a condition | 6 (Duplicant's `CondValue` over an expression; Kazandu's `CondPresent`) | 0 |
| type statics with a counter predicate | 11 (Station spacecraft) | 0 |
| keyword statics with a condition | 15 | 25 |
| P/T statics with a Dynamic value | 16 | 1 |
| P/T statics with a condition | 9 | 21 |

`sbaBoardObjectOnly`'s rule marks every override static as non-object. Under that
rule, 14 of the 16 EDH tables would need the full non-object fingerprint. Ten of those
14 read only the source's own counters (Station) or a constant count of battlefield
objects (Kazandu). Of the conditions seen, only Duplicant's (6/16 EDH tables) and
Goddric, Cloaked Reveler's celebration condition genuinely read non-object state. Both
are on the first `dpod` heuristic-bot pod table, the one for games 0-3; the other four
pod tables were not checked.

## 3. Candidate designs and their measured ceilings (`typesGen`)

The probe ran every design side by side on the owner's 161 asks per transition (20,000
transitions, the bench's pods and seeds). For each design it counted the hits and the
hits whose answer differed from a recomputation ("wrong"). The instruction figures
come from a shadow build: the difference between a run that recomputes every hit and
a run that answers it.

| design | key | hits per transition (EDH4) | hits per transition (EDH4 auto-pay) | wrong |
|---|---|--:|--:|--:|
| K0 | `touchN` + full non-object context (scratch, `ctrlM`, pools, turn facts, `sbaFingerprint`) | 102.9 (63.8%) | 117.2 (66.7%) | 0 |
| (a) | characteristics epoch: every class but damage and marks, plus the full context | 102.9 (63.8%) | 117.3 (66.7%) | 0 |
| (b) | per-table epoch: the classes the table's type statics read, plus the full context | 104.0 (64.5%) | 118.6 (67.4%) | 0 |
| (b'), conditions classified by read set | the same, with the turn/life/pool context only under a non-object condition | 114.8 (71.1%) | 127.8 (72.7%) | 0 |
| (c) dependency | global epoch on writes to a type source (or any write, under a board-count condition); otherwise the slot's own object stamp (`ChangedSince`); context = control masks, emblems, line | 133.1 (82.5%) | 147.2 (83.7%) | 0 |
| oracle | the slot's previous answer | 161.0 (99.8%) | 175.5 (99.8%) | n/a |

What the asks cost:

- **K0 hits:** 65.3k instructions per transition (6.4% of EDH4), from the shadow
  difference of a real K0 cache. But K0's per-ask check costs about 155k: building
  and comparing the context, with `sbaFingerprint` hashing the cold banks on every ask.
  K0 measured +8.8% (EDH4 1,117,802).
- **Write-class epochs (a) and (b) do not pay on their own.** Fewer than one write per
  transition that (a) ignores reaches the asks, and the misses come from context
  changes, not object writes. About 3.4 context changes per transition each make the
  next ~17 asks miss: `touch` 1.48, `sbaFingerprint` 0.78, `availX` 0.65, `probe`
  0.34, `sp` 0.07. A cache of the last eight contexts, four entries per slot, gained
  only 3 hits per transition. The contexts rarely repeat.
- **Eager maintenance (c'):** recomputing the affected slots on each write never beats
  the lazy dependency design (c). It recomputes the same slots whether or not they are
  asked again. Its write volume is the "global" bumps (1.86 per transition, about 50
  slots each), plus one slot per other characteristic-affecting write.
- **Design (c)'s ceiling:** about 126k on the probe (an overcount: the probe's
  per-design bookkeeping is skipped on hits). The prototype realized 60k of hit value.

## 4. The prototype (types only, design (c))

The prototype was a per-slot entry in the ForkPool, holding the types, the touch count
and the object, keyed by a context number. Nothing was added to State, and nothing was
allocated in the loop. Its rules:

- It runs only on a table whose type statics read objects alone (`tyObjTable`: no
  condition but a constant `CondPresent` over a plain filter, no gate, plain filters
  plus counter thresholds, no Ring). Every other table computes every ask.
- A hit needs four things:
  1. the asker owns the pool's cache;
  2. no hold, filter search or census window is running;
  3. the slot holds the same object, and that object is unwritten since the entry
     (`ChangedSince`);
  4. the context number is unchanged.
- The context number moves on a new line, emblem count, control mask or set of statics
  in force. It also moves on a write to a type source, on any write under a board-count
  condition, or after 256 writes.
- `CopyFrom` and `InitNStart` drop the entries. So does `refreshTy` when it moves a
  slot's walk flag. The in-place writers listed in section 1 hold the cache (no read,
  no write): the `pcMemoHold` sites, `lfxCond`, and `askColValue`.

Shadow hook (recompute every hit, panic on a difference): zero mismatches on all of
these:

| run | transitions | hits | misses | hit rate |
|---|--:|--:|--:|--:|
| EDH pods 0-7, `Config{Mulligans: 2}` | 12,000 | 1,360,122 | 88,517 | 93.9% of the asks it takes |
| EDH pods 8-15, `Config{Mulligans: 2}` | 12,000 | 943,248 | 61,235 | 93.9% |
| EDH pods, `LockstepConfig(SurfaceAutopay)` | 12,000 | 1,099,308 | 61,252 | 94.7% |

The FDN soak loop (40 pairings on the bench decks) gave 11,570 hits, none stale.

A mutation that drops the invalidation is caught by the shadow check. The prototype
answers 79 to 113 asks per transition of the 180.

| bench | base `e62aa28e17` | prototype | change |
|---|--:|--:|--:|
| BenchmarkTransitionEDH4 | 1,027,439 | 990,336 | -3.6% |
| BenchmarkTransitionFDN | 67,099 | 67,209 | +0.16% |

On the `dpod -policy hbot` pods the prototype answered nothing in games 0-3: that
pod's table carries Duplicant and Goddric, Cloaked Reveler, so it is not object-only. A K0-style full
context there costs more than it saves. The hbot pods' ceilings by design were not
measured.

## What blocked it, and what would move it

1. **The non-object check is the cost, not the write classes.** Any key that has to
   cover conditions over life, tallies or expressions pays `sbaFingerprint` (or an
   equivalent compare) on every ask, and that costs more than a hit saves. Only a
   per-field epoch would fix this: a counter bumped by the few writers of life,
   tallies and turn facts. It cannot live in State, and putting it in the pool would
   need a pool dereference at every writer.
2. **Table coverage.** A prep-time read-set classifier for conditions and expressions
   would bring the Duplicant- and Goddric-style tables in, with only the fields each
   condition reads in its key. Those fields: the exiled-with set (Duplicant), the
   celebration tally (Goddric). Without one, at least 6 of the 16 EDH pods, and the
   hbot pod that was checked, compute every ask.
3. **Types alone are small.** All type asks cost 13.2% of EDH4. The prototype keeps
   3.6%, and with every table covered, (c)'s hit rate suggests about 6 to 8%. The same
   key applied to `grantedAbilitiesOwn` (the largest by inclusive share, 23%) and to
   `ptGen`/`kwGen` on the search workload is where a cross-walk cache could clear 5%.
   That needs grant lists in a pooled arena (the shelved memo has one) and the
   read-set classifier from point 2 for P/T's Dynamic and conditional statics.

## Not verified

- The hbot-pod ceilings per design. The probe ran only on the bench pods, and only the
  first hbot pod table (games 0-3) was checked for whether it is object-only.
- Kazandu Tuskcaller's pod (EDH pod 14) may or may not pass `tyObjTable`: its
  `CondPresent` count was not inspected.
- The `ptGen`, `kwGen` and grant ceilings under these keys. The ask and change counts
  above come from the earlier probe; their hit value was not measured.
- The kpod search workload.
