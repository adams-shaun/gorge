# dirac at scale: operand algebras and vocabulary ports

Status: approved by the operator 11-19 ("od all"), OD-1 to OD-11. Implementation tickets are the `dsd-*` set in gorge's agentctl ledger. Repro scripts are kept as .txt so the module does not build them.


Status: decided design, for the operator's sign-off (OPERATOR-DECISIONS.md) and then `docs/design/2026-11-20-dirac-operand-algebras.md`.
Base: origin/main `9ce12ae3fb` (2026-11-20 03:06Z), corpus pin `418ba5215f` (FORGE_REF, `.cards/cards.lock`).
Every number below was measured at that base with a command in section 12. A number or claim marked INFERRED was not
read from code or data.

## 1. Decision

Per-class lowering stays, but it is no longer the main path. dirac takes four **operand algebras** and **ports gorge's
vocabularies whole, one family at a time**:

1. **SetExpr**, the object and player set algebra. It replaces the closed `kop.Ref` enum as the way to spell anything
   new.
2. **Filter programs** (`PredProg`). They carry every filter conjunct the fixed `kop.Filter` struct cannot hold.
3. **One duration operand** and **Effect containers** (command-zone effect records whose abilities are ordinary
   lowered statics, triggers and replacements).
4. **One object-set store**, which unifies the per-source memories (Remembered, Imprinted, ExiledWith, Noted, Chosen,
   Paired) into one cold region.

Two rules come with them. kgen lowers each rules parameter the same way under every API, instead of shape by shape.
Each family's atoms (filter words, Defined$ referents, Count$ heads, trigger, static and replacement modes) are then
ported from gorge's own tables, and each atom is proven by an **atom-level differential harness** (`atomdiff`). That
harness runs gorge's evaluator and dirac's on the same live lockstep positions. Per-card lockstep evidence stays the
support criterion, unchanged.

Three things are rejected, and section 5 says why: a generic interpreter over gorge's IR inside the kernel, a
transpiler from gorge's effect code, and continuing class-by-class waves as the main path.

The design is **additive**. A card that lowers today keeps its exact lowered IR, so its evidence key is unchanged and
the `stale_max` ratchet stays flat. The new encodings are reached only from what kgen refuses today. A table without
the new features pays one branch per feature gate, so FDN and BLB instructions per op do not move.

What it buys, over the 7,439 paper cards gorge plays and dirac does not lower (section 6):
- The nine mechanism phases D1-D9 unlock **3,499 paper cards with the algebras alone**. This is the lower bound:
  their guard and unrecognised-value classes only.
- They unlock **4,779 with the vocabulary ports**, counting every hidden blocker. This is the planning number.
- They unlock 5,594 at the visible-only upper bound.

Paper support goes from 75.5% to between 86.3% and 90.2%. What is left after D9 is about 576 API and effect-shape
classes plus 67 non-expanded keywords. They stay per-class work, on a cheaper substrate.

The singleton tail is 1,571 classes. Under this design about 350 of them stay per-class work: 132 API atoms, 177
effect-shape classes and 37 keywords. The rest are compositions or atoms of a ported vocabulary.

## 2. Where we are (measured)

### 2.1 Coverage

| set | cards | paper |
|---|---:|---:|
| corpus (`.cards/kgen/ALL.txt`) | 34,442 | 32,530 |
| gorge playable (`forgec report`: 33,463 = 97.2%) | 33,461 | 32,330 |
| dirac lowered | 25,355 | 24,891 |
| dirac supported (lowered, not refused, evidenced) | 24,984 | 24,574 (**75.5%**) |
| of which hold at any seat count (support.md `4-seat`) | 24,969 | n/a |
| unverified | 347 | 293 |
| refused by name | 24 | 24 |
| **gap A: gorge plays, dirac does not lower** | **8,106** | **7,439** |

Paper means legal in a real format, by `repro/legal.py` over Forge's editions and formats at the pin. The 11-19 plan
(`35aed6c5ca`, 15.3 h earlier) had 25,205 lowered and 24,753 supported. Lowering since then is +150, about 235 a day.

### 2.2 The refusal census

These figures come from `bin/kgen -census-std -all-blockers -cards $PWD/.cards -tsv` over ALL rows. They cover the
paper gap-A cards (7,439).

| measure | value |
|---|---:|
| distinct refusal classes | 3,061 |
| classes with exactly 1 paper carrier / 2 / 3-4 / 5-9 / 10+ | 1,571 / 501 / 404 / 346 / 239 |
| carrier-rows (card x class) | 10,558 |
| cards with one class / two / three / four or more | 5,693 / 1,701 / 458 / 254 |
| cards with params left unread after a refusal (hidden blockers) | **2,666 (36%)**, 3,216 partial rows |

Every class gets a **kind** (`repro/mech.py`'s GUARD regex over the class text, INFERRED at the boundary):
- **guard**: both pieces exist, and kgen refuses their combination ("X outside a Y", "X shape", "beside", "without",
  ...).
- **value**: kgen does not recognise a value of a parameter it does read (`"_"`).
- **atom**: a primitive is missing (`api:`, `kw:`, `trig:`, `stat:`, `repl:`, `count:`, `prop:`, `param:`, `cost:`).

| kind | carrier-rows | share |
|---|---:|---:|
| guard | 3,543 | 34% |
| value | 2,630 | 25% |
| atom | 4,385 | 41% |

**59% of what blocks the paper gap is composition or spelling, not missing rules.**

### 2.3 Mechanism families

Each class goes to the kernel capability that would retire it (`repro/mech.py`, regexes in order). Sole-family cards
are blocked only by that family.

| family | classes | carrier-rows | sole-family cards | guard / value / atom % |
|---|---:|---:|---:|---|
| M2 object memory (Remembered, Imprinted, ExiledWith, Noted, Chosen, Paired) | 371 | 1,627 | 562 | 35 / 16 / 47 |
| M3 reference algebra (Defined$, Targeted, Triggered*, unknown filter head) | 407 | 1,629 | 788 | 35 / 52 / 12 |
| M4 durations and Effect objects | 133 | 676 | 441 | 55 / 32 / 12 |
| M5 filter program (prop:, comparisons, ValidSA, count:Valid) | 454 | 1,275 | 722 | 18 / 25 / 55 |
| M6 value and condition expressions (count:, SVar, Condition) | 309 | 822 | 475 | 21 / 33 / 44 |
| M7 keywords | 108 | 721 | 442 | 0 / 0 / 100 |
| M8 trigger, static and replacement mode tables | 394 | 1,285 | 786 | 39 / 8 / 51 |
| M9 cost program | 180 | 576 | 397 | 51 / 30 / 18 |
| M10 mana | 55 | 168 | 128 | 79 / 16 / 3 |
| M11 choice vocabulary | 68 | 254 | 84 | 38 / 35 / 26 |
| M12 API / param atoms | 247 | 645 | 342 | 0 / 0 / 100 |
| M13 effect shapes (Clone, Animate, CopySpellAbility, ...) | 329 | 873 | 584 | 63 / 32 / 4 |
| several families | n/a | n/a | 1,684 | |

The tail is flat by class: 1,571 classes have one card each. **It is not flat by mechanism.** The 1,571 singleton
classes are 558 guard, 301 value and 712 atom. Of the atoms, 208 are filter words gorge already has, 93 are modes, 72
are Count$ heads and 57 are memory forms. Only 132 are API atoms.

### 2.4 State and speed

Measured with `make test-one`, `perf stat` and the benches. The box was loaded (about 28 of 32 cores), so wall times
are advisory.

| measure | value | source |
|---|---:|---|
| `unsafe.Sizeof(State{})` | **38,072 B** | `TestStateSize` |
| of which `stateCore` (copied whole on every clone) | 24,640 B | scratch test |
| of which `perms` (68 B x `MaxPerm` 192, copied to `permHi`) | 13,056 B | scratch test |
| largest pool, State + cold (`edh`) | **45,048 B**, against a bar of 45,056 (**8 B of headroom**) | `TestEveryPoolFitsTheStateBar` |
| pools / with cold < 1 KiB / median cold | 1,013 / 866 / 24 B | scratch test |
| `edh` cold regions | exRider 1,536, parked 1,056, ms 744, mkObj 488, srcMem 384, exw 384, entered 384, may 384, ... | scratch test |
| largest `stateCore` parts | genState 9,752 (scr 2,400, lki 2,268, fx 768), objs 3,904, pl 3,424, stack 2,784 | `dirac/testdata/layout.tsv` |
| `kop.Instr` / `kop.Filter` / `kop.Expr` | 16 B (1 pad) / 40 B / 30 B | scratch test |
| instructions:u per op, `BenchmarkTransitionFDN` | **62,021** (program base 59,647: 1.040x) | perf stat, as gate step 5 |
| instructions:u per op, `BenchmarkTransitionBLB` | **74,982** (program base 70,566: 1.063x) | perf stat |
| `BenchmarkTransitionFDN` / `BLB` (loaded box) | 5.29 / 6.24 us per op, 0 allocs | go test -bench |
| `BenchmarkCloneFDN` | 192 ns, 0 allocs | go test -bench |
| `dirac/search` `BenchmarkSearch` | 1.58 ms/op, 15.85 us/sim, 0 allocs | go test -bench |
| gorge, per decision | 10-25 us | `docs/site/performance/engine.md` |

Two of these are binding:
- **The Commander pool is at the bar.** `edh` is the largest State of the 1,013 pools. Any cold region an `edh` card
  needs breaks `TestEveryPoolFitsTheStateBar` today. Commander is the program's priority, since pods are four players
  only.
- **The instruction budget is spent on this box.** Since the 11-19 re-base (`8d4dd66bdd`), main has moved FDN +4.0%
  and BLB +6.3% past the program base, over 326 non-test dirac commits. That is past `INSTR_PB_MAX` 1.02. The 11-18
  local cache (`~/.cache/dirac-gate/a96a077569…`) read 55,385 against that day's base of 55,386, so this box measured
  the base. INFERRED: either the gate host measures lower, or the per-class approach has used up the budget, one
  hot-path branch per mechanic. Per-class lowering is not free on FDN even when no FDN card changes.

## 3. Why the tail is flat: the IR is not orthogonal

The kernel's vocabulary is closed enums and fixed struct shapes. kgen lowers parameters per API and per shape, and
refuses every other combination:

1. **Fixed filter shape.** `kop.Filter` (`dirac/kop/filters.go:97`) holds one `Pred` slot with `PredN`/`PredV`, the
   `FilterFlags` bits (uint16: "Bit 15, the LAST free FilterFlags bit", `filters.go:90`), the `Flags2` bits (uint32),
   overflow sub-kinds (`PredWord7` 1-19) and an `Or` chain. A clause with two comparisons cannot be represented: kgen
   says "two comparisons in one clause" (`dirac/kgen/filter.go:1274`).
   - About 137 kernel functions take a `FilterID` or a `Filter`, across 20 or so matchers (`chars.go:133` `matchPerm`,
     `chars.go:247` `matchCard`, `filters_gen.go:43` `matchPermGen`, `filters_gen.go:633` `matchCardGen`, ...). A new
     predicate is added matcher by matcher, and kgen refuses it in contexts whose matcher lacks it ("filter X outside
     Y").
2. **Closed references.** `kop.Ref` is a uint8 enum (`dirac/kop/refs.go:8`) with hand-placed high values (241-247).
   kgen's `defined()` (`dirac/kgen/lower_effect.go:24`) is a string switch, one case per spelling.
   - gorge's `Defined$` is an algebra (`engine/effects/context.go:684` `definedSpec`): a dotted qualifier `X.qual`, a
     `" & "` union (`context.go:219`), `Non<ref>` complement, `AttachedTo <ref>`, `Spawner>`, controller and owner
     projections and LKI forms.
   - Of the 313 `Defined$` refusal rows on paper gap cards, 219 (70%) are such compositions: 72 dotted, 50 LKI, 48
     projections, 25 unions, 24 `Valid`. The other 93 name a base referent dirac lacks (the largest is
     `TriggeredBlockerLKICopy`, 41).
3. **Durations per op.** A one-shot continuous effect is an `fxRec` (`dirac/gen.go:1916`) whose `ovRec` knows only
   until-end-of-turn or permanent (`ovEOT`). The other durations live per op (`kop/operands.go:195-210` `Dur*` for
   GainControl, MayPlay and the designations). So `Pump Duration "UntilUntaps" / "UntilEndOfCombat" /
   "AsLongAsControl"` is a refusal even though the expiry exists elsewhere.
4. **Effect objects by shape.** `api:Effect` is lowered by 18 `lower_effect*.go` shape files. Every other
   combination of `StaticAbilities$` / `Triggers$` / `ReplacementEffects$` is refused: "Effect static `_`" 28
   carriers, "Effect replacement `_`" 37, "Effect triggers without a permanent duration" 31. This holds even when each
   static or trigger lowers on a permanent.
5. **Memory by shape.** Per-source memory exists in three stores:
   - `coldSrcMem` (`dirac/zone_srcmem.go:54`, 64 records, `StallSrcMem` 173)
   - the RemPersist links (`zone_rememberpersist.go`)
   - `coldExw` (`zone_exiledwith.go:54`)

   kgen admits each per shape. `lower_rempersist.go` "accepts exactly that pair". `lower_remembered.go:459` refuses
   "reads Remembered written by another ability" (41 carriers, 0 sole), and `param:Cleanup.ClearImprinted` refuses
   109 carriers (0 sole). These are hubs: their cards carry other memory classes too.
6. **Keywords by whitelist.** `lowerKeywords` (`dirac/kgen/lower.go:1138`) is a switch that refuses any head it does
   not name. That includes heads gorge expands into ordinary IR lines (`engine/cards/keywords.go:105`
   `registerKeyword`, 81 heads, each expansion carrying `Keyword$`/`KeywordLine$`). kgen checks for the expansion
   (`hasKeywordLine`, `lower.go:2108`) only for heads it lists.
   - 396 of the 845 paper `kw:` carrier-rows (47%) are heads gorge expands.
   - Only 27 of those 81 heads are never named in gorge's non-test rules or effects code. That makes a strict
     pass-through worth 64 paper cards (K2).
7. **Hidden blockers.** A refusal stops reading the node. 36% of paper gap cards have params left unread
   (`censusstd.go:126` `partialClass`). Closing a class then surfaces the next one: the 11-19 plan measured 376 new
   classes per 450 closed.

The scale shows in the code:
- kgen has 610 non-test files, 448 of them `lower_*.go`, and 4,108 `refuse` sites in 430 files.
- `kop.CardDef` is 1,392 B of per-mechanic fields.
- A refusal class is usually not a missing rule. It is a missing *combination*.

## 4. Constraints every option is held to

- `dirac.State` holds no references beyond its views of one pointer-free buffer and the fork pool. The transition loop
  allocates nothing (gate step 3, `TestColdArraysHoldNoPointers`). Escapes and inlining are checked by gate steps 6-7.
- Mutable state fits L1d: `maxStateBytes` 44 KiB (`dirac/kernel_test.go:38`) over State plus a pool's cold arrays.
  The bar is protected.
- Instructions: `INSTR_MB_MAX` 1.05 against the merge base and `INSTR_PB_MAX` 1.02 against the program base
  (`scripts/dirac-gate.sh:47-57`). Both are protected.
- gorge leads. dirac plays identically decision by decision, with no tolerated divergence. A card dirac cannot match is
  `kgen -refuse`d. Capacity stalls are declared and counted.
- Pods are four seats; Commander is never played in two seats. No cgo, no third-party dependencies, no nondeterminism,
  no map order reaching an action.
- Evidence keys: a card's key is the sha256 of its `kgen -lir` block (`dirac/kgen/lirkey.go:16`). `stale_max=99`
  (`dirac/support/coverage-floor.txt`) only falls, so a change that rewrites the IR of evidenced cards must re-evidence
  them in the same change.

## 5. Options

| option | what it is | paper cards (D-phase horizon) | State bytes | instr / ns | hard rules | verdict |
|---|---|---:|---|---|---|---|
| **A. Per-class waves** (the 11-19 plan) | one refusal class per PR, 4-20 per stacked wave | "a" waves: 4,130 in ~193 agent-days; 95% at about day 45-50 | grows per mechanic, cold | creeps. Measured +4.0% FDN / +6.3% BLB past the program base since 11-19 | holds | **kept for API atoms only** (section 9) |
| **B. Generic IR interpreter in the kernel** | the kernel reads gorge's IR (spec strings, SVars) at run time | in principle all, but each param still needs gorge's semantics reimplemented | strings and slices in the table only, plus per-resolution parse state; State itself stays pointer-free (INFERRED) | INFERRED 3-10x per op: string dispatch per filter evaluation, which is what gorge's `filter_compiled.go` exists to avoid. That makes it gorge-speed (10-25 us per decision) against dirac's ~5 us per transition | breaks no-alloc unless everything is pre-parsed, and pre-parsed is option D | **rejected**: it converges on gorge without gorge's clone cost advantage, and lockstep still needs atom-exact semantics |
| **C. Transpile gorge's effect code** | generate kernel Go from `engine/effects` (1,045 files) | unknown | gorge's code reads `*state.Game` pointers and maps and is event-sourced (`events.Apply`) | n/a | each translated function would need a pointer-free rewrite | **rejected**: a research project with no bounded deliverable |
| **D. Operand algebras + vocabulary ports + atomdiff** (recommended) | sections 1 and 6 | lower 3,499 / planning **4,779** / upper 5,594 by D9 | about +0.6 KiB cold on `edh` after offsets, 0 in `stateCore` (section 7) | 0 on FDN and BLB, which use no new encoding (additive) and pay one gate branch per feature (INFERRED < 0.2%) | holds; atomdiff adds per-atom proof | **chosen** |
| **E. Table-driven ability shapes only** | D7 without D2-D6 | about 1,000 cards (D7 alone, conservative) | same as D7 | same | holds | subsumed by D |

Why D beats A on cost, with each agent-day count INFERRED from the 11-19 measured rates:
1. **A's unit is a class. D's unit is a mechanism plus a bounded vocabulary.**
   - D1-D9 retire 2,439 classes, including 859 of the 1,571 singletons. They do it in about 26 PRs, roughly 120
     agent-days.
   - A's "a" and "b" waves reach a comparable 6,141 paper cards with 1,495 classes in about 486 agent-days, and still
     leave the 1,572-class singleton tail at about 511 seat-days.
2. **Surfacing shrinks.** A guard class closed by A exposes the next guard on the same node: 0.84 new classes per class
   closed. D closes a family's compositions together, and with T01 the census reports hidden params by family, so the
   denominator is honest before work starts.
3. **The vocabulary is bounded by gorge, not by cards.** gorge's whole supported vocabulary is api 190, count 242, kw
   213, repl 31, stat 79 and trig 116 (`effects.Supported()`, `engine/effects/registry.go:692`). On top of that come
   gorge's filter words (`engine/effects/filter.go:78` `predicates`, 84 entries plus the `with<Keyword>` family at
   `:1022`; `filter_word.go` 79 `wordKind`s) and about 89 `definedSpec` codes. Porting a family is a finite list, and
   every gap card gorge plays uses only words in it.
4. **The proof is per atom as well as per card.**
   - atomdiff compares gorge's evaluator (`effects.MatchesObjectCtx`, `filter.go:3276`; `definedSpec`; the count heads)
     with dirac's on every object at every lockstep decision of the existing seeds.
   - A wrong atom fails at its first wrong object, named, instead of as a divergence many turns later in some game that
     happened to deal it.
   - This is what lets Sonnet seats port words safely.

## 6. The design

### 6.1 Principles

1. **Additive tier.** kgen keeps every lowering it has, byte for byte in `-lir`. A new encoding (`RefSet`, `PredProg`,
   a duration code, an effect container) is emitted only on the path that refuses today. Evidence keys do not move,
   and `stale_max` does not rise. A later PR may migrate old shapes onto an algebra only with the re-evidence in the
   same PR.
2. **One evaluator per algebra.** Each algebra has one kernel evaluator that takes one context struct. Each matcher or
   op that reads it adds one `case` to its existing dispatch, and that is the only per-site edit.
3. **Off costs one branch.** Each algebra is gated per table (`newGenUse`, `genneed.go`). FDN and BLB tables carry none
   of the new encodings, because every FDN card already lowers and keeps its IR. So FDN and BLB instructions per op
   change only by the gate tests: one predictable branch each, out of line.
4. **Vocabulary comes from gorge's tables, not from cards.** Each port PR names the gorge table it ports and lists every
   entry it ports or defers. The order within a family is by paper carriers.
5. **Atoms are proven by atomdiff, cards by evidence.** The per-card evidence rule (3 clean plays, every ability key;
   `EvidenceMinPlays`, `EvidenceAllKeys` in `dirac/oracle/evidence.go:11-14`) is unchanged. atomdiff is additional,
   and runs in `dirac/oracle`'s tests on committed seeds.
6. **State is cold and paid per table.** No `stateCore` growth. Each new store is a `newCold` region sized from
   `genPrep`, and a full store is a named stall. On `edh`, every byte added is offset in the same PR until the operator
   moves the pool bar (OD-1).
7. **No new hot op.** New behaviour is ext sub-ops (`ExtOp` manifest) and manifest codes. The free hot op band
   [158,249] is not touched.

### 6.2 Components

#### K0 Census by mechanism (T01)
- kgen tags every refusal class with a family and a kind in `dirac/kgen/classes.go`, the one place a refusal is
  classified.
- It also tags every unread-after-refusal param with the family it will need.
- `-census-std -tsv` gains two columns, and `acceptance.tsv` gains a per-family carrier pin. A family's count may only
  fall, so a phase PR proves its yield.

#### K1 atomdiff (T02)
`dirac/oracle/atomdiff` replays committed kfuzz seeds in lockstep. These are the FDN and BLB slices and the
std-blb/otj/mkm synthetic pools the gate already plays (`dirac-gate.sh` steps 4 and 9), plus one 4-seat `edh` seed.

At each priority decision it evaluates every registered atom on both sides:
- **filter words**: gorge `MatchesObjectCtx(g, "Card.<word>", o, sc)` against dirac's atom eval, for every object;
- **Defined bases and SetExprs**: gorge's `definedSpec` against dirac's set evaluation, inside a resolution context
  built from the stack top;
- **Count$ heads**: gorge's count evaluator against dirac's `Expr` eval.

It maps objects through `canon`. A mismatch fails, naming the atom, the seed, the decision index, the object and both
values. Atoms register themselves in their own files, with no shared list. Test-only, so 0 bytes and 0 ns in the
kernel.

#### K2 Keyword pass-through (T04 gorge, T05 kgen)
gorge gets `cards.KeywordExpansionOnly(head)` and a ratchet that lists the expandable heads whose name gorge's rules or
effects code also reads directly (`HasKeyword("<head>")`, combat or rules switches). The `with<Keyword>` filter words
are filter atoms, not this.

kgen accepts a keyword K when all of these hold:
- `KeywordExpansionOnly(K)` is true;
- every expansion line exists (`hasKeywordLine`, or a trigger carrying `Keyword$ K`);
- K needs no kernel bit, or a table filter that reads `withK` gets one through the KwID manifest.

The expansion lines then lower as ordinary abilities. 0 State bytes, 0 kernel code.

gorge reads some expanded heads by name outside their expansion:
- `engine/rules/keyword_heads.go` holds 76 literal heads;
- `trigger_blocks.go:191-214` reads Flanking, `trigmatch/counters.go:205` reads Vanishing, and
  `trigger_granted.go:663` reads Afflict.

So the pass-through is **strict**. Only the 27 expanders no non-test file in `engine/rules` or `engine/effects` names
qualify: Amplify, Annihilator, Backup, Battle cry, Bushido, Champion, Craft, Devour, Fabricate, Fading, For Mirrodin,
Graft, Gravestorm, Increment, Ingest, Job select, Mobilize, Provoke, Rampage, Ravenous, Reconfigure, Reinforce,
Soulshift, Transmute, Undaunted, Undying and etbCounter.

| | paper cards |
|---|---:|
| strict (Soulshift 27, Reinforce 10, Ingest 9, Annihilator 6, ...) | **64** |
| ceiling, if every expander qualified | 127 conservative, 133 visible |

T05 reviews each other expander head by hand: Flanking, Vanishing, Afflict and the 51 more. A head is admitted only
when the kernel already replicates gorge's rules-side read, with a per-head lockstep repro. Soulbond, Mentor,
Replicate and Squad also need M2/M6 atoms, and arrive with their families.

The section 6.3 rows use the 127 ceiling. With the strict list only, D1-D9's cumulative figures are at most 63 lower.

#### K3 SetExpr (T06 kernel, T07 kgen grammar, T08 base referents)

Representation:
- `kop.Table.Sets []SetExpr`. A SetExpr is a short postfix program of `SetOp{Kind, Arg}`:
  - push a base (target slot k, the target union, self, controller, trigger object or player or source, remembered,
    imprinted, exiled-with, chosen, attached, sacrificed, discarded, defined-by-filter in a zone, and so on);
  - `∩ filter` (a `FilterID`, which may be a `PredProg`);
  - `∪`, players-complement, controller-of, owner-of, LKI-of, spawner-of.
- An instruction names a set through one new manifest `Ref`, `RefSet`. The set id lives in a per-ability side table
  `Ability.SetRefs []SetRef{PC uint8, Set uint16}`.
- `kop.Instr` stays 16 B. The interpreter looks up the side table only when `in.Obj == kop.RefSet`, a branch every
  existing op's operand read already takes.

Kernel:
- `s.evalSet(k, ab, pc, ctx) setVal` returns `setVal{objs objBits, pl uint8}`. That is 64 B for `MaxObj` 488 plus the
  players, on the Go stack, so 0 State bytes. Gate step 7 holds it off the heap.
- Order is object-index order, then seat order from the controller: gorge's `AliveFrom` and its target order. Where
  gorge's order differs (the target union is in slot order), the base keeps an order array of length ≤ `maxXTgt`,
  still on the stack.
- A suspended resolution re-evaluates the set when it resumes, as gorge re-evaluates `Defined$` when the op runs.

Ops:
- Each op that takes `Obj Ref` gets one `RefSet` arm. T06 wires the 12 ops with the most `Defined$` refusal carriers:
  ChangeZone, Pump, PutCounter, DealDamage, Destroy, Sacrifice, Tap/Untap, GainLife/LoseLife, Draw, Discard,
  GainControl and Animate.
- Other ops refuse a `RefSet` in kgen until wired. That refusal is a guard class, counted under M3.

kgen (T07):
- `definedSet(v string)` ports gorge's grammar: dotted qualifier, `" & "`, `Non`, `AttachedTo `, `Spawner>`,
  `*Controller`/`*Owner`, `*LKICopy`, `Valid <filter>`, `ValidZone <filter>`.
- It is called only where `defined()` returns a refusal today.

Atoms (T08): the base referents, about 40 with ≥ 2 paper carriers, from gorge's `definedSpecCodes`. Each base is one
file (`set_base_<name>.go`) plus one atomdiff registration.

Cost: 0 State bytes and 0 FDN instructions, since no FDN card emits `RefSet`.

Unlocks:

| | paper cards |
|---|---:|
| mechanism only (T06+T07) | 682 |
| with the bases (T08) | 796 |
| conservative | 441 + the hidden M3 params of 696 cards (the sim credits them once M3 is done) |

The 18 "ChoosePlayer among the opponents answered as the one opponent" cards are a choice over the SetExpr
`Opponents`. T19 retires them, and with them most kgen two-seat marks (section 8).

#### K4 Object-set store (T09 unify, T10 lower)

**T09** (kernel only, no LIR change) replaces three regions with one cold region `objsets`: `coldSrcMem` (64 x 6 B =
384), `coldExw` (384 on `edh`) and the RemPersist links' use of `genState.links`.
- A record is `{o Obj, src Obj, sver uint8, kind uint8}`, 6 B. The kinds are Remembered, Imprinted, ImprintedAny,
  ExiledWith, Noted, NotedFor, ChosenCard, Paired, HauntedBy and ChampionedBy.
- Records are an insertion-ordered live prefix. They are live while their source incarnation is (`smLiveFor`'s rule,
  `zone_srcmem.go` header), and collected when no stack entry or pending trigger reads them (`smGC`).
- Capacity is 96 records when any per-source memory card is in the table, else 0. A full store is `StallObjSet`, which
  reuses `StallSrcMem` 173's name.
- `exwRec` (`zone_exiledwith.go:62`) and `smRec` already have the same shape, `{o, src Obj; sver, fl uint8}`.
- `edh` goes from srcMem 384 + exw 384 = 768 B to 576 B, **saving 192 B**.
- `genState.links` (`gen.go:2297`, 16 records, which also serve ExileUntilLeaves, Hideaway, Aegis and the hand exile)
  stays in `stateCore`. T09 only routes the RemPersist reads through the store's API.
- The proof is lockstep neutrality: the whole gate, the slices, the synthetic pools and every pin, with no LIR change.

**T10** (kgen) lowers memory reads as SetExpr bases (T08) and writes as `ExtRememberSet{kind, set}`:
- `RememberObjects$` on Pump, Effect and delayed triggers;
- `ImprintCards$`, `NoteCards$`, `ForgetOnMoved$`, `ExileOnMoved$` and `Cleanup` `Clear*` / `Forget*`.

It drops the per-shape acceptance in `lower_rempersist.go` and `lower_remembered.go:459` for new cards only (additive).

Unlocks:

| | paper cards |
|---|---:|
| mechanism only | 402 |
| family complete | 707 |
| conservative | 449 |

#### K5 Filter programs (T11 kernel, T12 kgen fallback, T13 word ports)

Representation:
- `PredProg`, a manifest `Pred` code. `PredN` indexes `Table.FilterProgs`, so `Filter` stays 40 B.
- A program is a conjunction of literals `{Word FilterWord(uint16), Neg bool, Arg int16, V ValueRef, R Ref}` that
  extends the struct's own conjuncts. Disjunction stays the `Or` chain, which is how gorge's comma alternatives already
  lower.
- `FilterWord` is a new manifest family. Each word is one file `dirac/fw_<word>.go` with a
  `//kmanifest:fword FwX` handler `func (s *State) fwX(k *prep, c *fctx, l *kop.FLit) bool`.
- `kmanifest gen` writes the dispatch table, a fixed array of funcs indexed by word: package data, not State.

Kernel:
- Each `switch fl.Pred` site gets `case kop.PredProg: return s.filterProg(k, fl.PredN, &fctx{...})`. There are seven:
  `chars.go:222` (`predPerm`), `chars.go:280` (`matchCard`), `filter_predicates.go:118` and `:144`,
  `filters_gen.go:486` (`predPermGen`), `:840` (`matchCardOneAt`) and `:982` (`predCardPT`).
- A matcher that never reaches a `switch fl.Pred`, such as `static_layerwalk.go:548` `matchCurOne`, is found by
  T11's audit test. That test fails if a `PredProg` filter reaches a matcher with no arm.
- `fctx` carries the zone, object, slot, card, owner, controller, source, layer view and the resolving entry.
- A word that cannot be evaluated in a context (a battlefield-only word on a card off it) returns gorge's fail-closed
  answer. atomdiff checks those cases too.

kgen:
- **T12**: when `lClause` overflows (a second comparison, a second `Pred`, a property with no flag), kgen emits the
  struct for what fits and a `PredProg` for the rest. Today these refuse.
- **T13**: ports gorge's word table in three PRs:
  - T13a: combat and history words (`attackedThisCombat`, `wasDealtDamageThisTurn`, ...);
  - T13b: relation words (`sharesCreatureTypeWith`, `sharesNameWith <ref>`, `withX`/`withoutX` for every keyword,
    `PairedWith`, `ChosenCtrl`, ...);
  - T13c: zone and provenance words (`suspended`, `TopGraveyardCreature`, `wasCastFromGraveyard`, ...).

Cost: 0 State bytes. Table bytes grow by the programs, only for cards that need them. 0 FDN instructions.

Unlocks:

| | paper cards |
|---|---:|
| mechanism only | 369 |
| family complete | 895 |
| conservative | 708 |

#### K6 Duration operand and Effect containers (T14, T15)

**T14** gives every continuous record one duration code: `ovRec` and fx, the keyword grants, `may`, control and
`exH`. It has one expiry sweep.
- The codes are gorge's durations: EOT, Permanent, UntilYourNextTurn, UntilEndOfCombat, UntilUntaps (object), the
  AsLongAs* family over control, in play and source, UntilYourNextUpkeep, UntilHostLeavesPlay and so on.
- The expiry hooks already exist per mechanic (cleanup, end of combat, untap, leave, control change).
- The new code sits in `ovRec`'s flag byte where it fits. INFERRED: `fxRec` is 12 B with a pad (`maxFx` 64 x 12 = 768
  B in `genState.fx`); if it does not fit, the code goes in a parallel cold array `fxDur [maxFx]uint8` that only
  tables with a non-EOT duration pay for, 64 B.

**T15** lowers `api:Effect` with any `StaticAbilities$`, `Triggers$` and `ReplacementEffects$` whose members lower on a
permanent.
- kgen builds an **effect card**: a synthetic `CardDef` row in the table, flagged `EffectCard`, whose abilities are
  those lowered abilities. This is gorge's own model, an effect object in the command zone (`engine/effects/effect.go`
  `effEffect`).
- The kernel generalises the emblem record (`static_emblemcold.go` `embRec`) into an effect record: `{card, ctrl,
  srcObj, srcVer, dur, ts, set}`, 12 B, read by the statics walk, the trigger fan-out and the replacement scan as a
  command-zone source.
- The emblem path is the existing proof that command-zone sources work in all three.
- Capacity is 8 per seat when the table has an effect card, else 0. On `edh` that is 32 x 12 = **+384 B**.
- Emblems keep their own region unchanged, so emblem pins stay byte-identical.

Unlocks:

| | paper cards |
|---|---:|
| mechanism only | 458 |
| family complete | 543 |
| conservative | 426 |

#### K7 Expressions and conditions (T16)
The `Expr` tree exists (`kop/values.go:159`; 99 `ExprKind` manifest entries). The gap is the count heads (85 `count:`
classes) and the condition forms: `Condition$`, `ConditionCompare`/`ConditionPresent`/`ConditionDefined`, and
`CheckSVar`.

T16 does three things:
- ports gorge's remaining count heads, ordered by paper carriers, each one file plus an atomdiff registration;
- lowers conditions as `ExCompare` over SetExpr counts;
- makes `PlayerCount*` heads iterate seats, which retires several pod-sensitive SVars.

Cost: 0 State bytes. Unlocks:

| | paper cards |
|---|---:|
| mechanism only | 333 |
| family complete | 576 |
| conservative | 778 (hidden Condition params are common) |

#### K8 Mode tables (T17a triggers, T17b statics, T17c replacements)
Trigger, static and replacement params are read orthogonally:
- `ValidCard`, `ValidSource`, `ValidTarget`, `ValidBlocker` and `ValidSA` through `PredProg`;
- `Defined`-like params through SetExpr;
- `TriggerZones$` generically;
- the effect's duration through T14.

Then the remaining modes in gorge's tables are ported: trig 48, stat 40, repl 64 classes. Each mode is one file through
the existing `TrigEvent`, `StaticKind` and `ReplKind` manifest families.

Cost: per mode, a cold array only where a mode needs per-turn history. INFERRED ≤ 64 B each, paid per table.

Unlocks:

| | paper cards |
|---|---:|
| mechanism only | 472 |
| family complete | 1,057 |
| conservative | 1,015 |

#### K9 Cost program and mana riders (T18)
Non-mana cost parts become a sequence of cost atoms (exists in part as `kop.Cost`):
- OptionalAttackCost, RaiseCost, AlternativeCost, ExiledMoveToGrave, Unattach, Draw and the reveal and exile costs;
- the mana-ability rider program;
- typed artifact mana.

The mana planner is the hot path (`mana_*.go`, 126 files), so T18 is gated per table and measured on FDN and BLB.

Unlocks:

| | paper cards |
|---|---:|
| mechanism only | 525 |
| family complete | 686 |
| conservative | 648 |

#### K10 Choice vocabulary (T19)
These choices become choices over SetExpr or filter pools:
- ChooseSource;
- the Vote params;
- ChooseCard `Defined$`, `Amount$` and `ControlledByPlayer$`;
- GenericChoice;
- ChoosePlayer among a set, which retires the "among the opponents" two-seat marks.

Unlocks:

| | paper cards |
|---|---:|
| mechanism only | 125 |
| family complete | 201 |
| conservative | 187 |

### 6.3 What the phases unlock

Simulated over the 7,439 paper gap-A cards (`repro/sim3.py`). A card counts as unlocked when every one of its classes
is retired.
- **lower**: only the guard and value classes of the families done so far.
- **upper**: every class of those families.
- **conservative**: upper, and the families of every param the card left unread must also be done.

| phase | tickets | classes (upper) | lower: cumulative paper | conservative: cumulative paper | upper: cumulative paper | supported paper % (conservative) |
|---|---|---:|---:|---:|---:|---:|
| D1 keyword pass-through (strict list: 64) | T04 T05 | 41 | 133 | 127 | 133 | 75.9 |
| D2 SetExpr + bases | T06 T07 T08 | 407 | 815 | 568 | 929 | 77.3 |
| D3 object-set store | T09 T10 | 371 | 1,217 | 1,017 | 1,636 | 78.7 |
| D4 filter programs + words | T11 T12 T13a-c | 454 | 1,586 | 1,725 | 2,531 | 80.8 |
| D5 durations + effect containers | T14 T15 | 133 | 2,044 | 2,151 | 3,074 | 82.2 |
| D6 expressions and conditions | T16 | 309 | 2,377 | 2,929 | 3,650 | 84.5 |
| D7 mode tables | T17a-c | 394 | 2,849 | 3,944 | 4,707 | 87.7 |
| D8 costs + mana | T18 | 235 | 3,374 | 4,592 | 5,393 | 89.7 |
| D9 choices | T19 | 68 | 3,499 | 4,779 | 5,594 | 90.2 |
| W keywords, non-expanded (Banding, Mutate, Splice, Protection, Disturb, ...) | T20 | 67 | n/a | 5,197 | 6,031 | 91.5 |
| W API atoms and effect shapes | T21 waves | 576 | n/a | 7,433 | 7,433 | 98.4 |

How to read this:
- The lower bound assumes the algebra retires every guard and value class of its family, and nothing else.
- About 30% of the Defined$ value rows are base referents, which are atoms, so lower is optimistic within value and
  pessimistic overall. It ports no vocabulary at all.
- Conservative is the planning column.
- Every newly lowered card still needs evidence before it counts as supported (section 7). The table counts lowered
  cards.

### 6.4 Costs

| phase | State (stateCore) | cold on `edh` | cold elsewhere | FDN/BLB instr | table bytes |
|---|---:|---:|---|---|---|
| T01-T03 | 0 | 0 | 0 | 0 | 0 |
| D1 | 0 | 0 | 0 | 0 | 0 |
| D2 | 0 | 0 | 0 | 0 (no `RefSet` in FDN/BLB); one `in.Obj == RefSet` test where an op reads its object, already a switch arm | `Sets`, `SetRefs` |
| D3 | 0 | **−192** | only pools with memory cards | 0 (lockstep-neutral refactor; the gate's 1.05 merge-base bar) | 0 |
| D4 | 0 | 0 | 0 | 0 (no `PredProg` in FDN/BLB; one switch arm) | `FilterProgs` |
| D5 | 0 | +384 (effects); +64 (`fxDur`) | effect-card tables only | one sweep test, gated | effect-card rows |
| D6 | 0 | 0 | 0 | 0 | `Exprs` |
| D7 | 0 | INFERRED ≤ +256 | per mode | gated per mode | modes |
| D8 | 0 | INFERRED ≤ +128 | per atom | **watch: mana planner hot path**, measured on FDN/BLB and the `edh` bench (T03) | cost atoms |
| D9 | 0 | 0 | 0 | 0 | 0 |
| **total** | **0** | **INFERRED +450 to +650 B** (−192 + 448 + ≤ 256 + ≤ 128) | pay-per-table | **≤ 0.5% FDN/BLB, INFERRED** | |

Because `edh` has 8 B of headroom, every phase that adds cold bytes to `edh` must offset them in the same PR, or the
operator moves the pool bar (OD-1).

## 7. Evidence throughput

evloop measured at 20:14 (the evidence loop's status and per-card files, kept outside this repository):

| measure | value |
|---|---:|
| landed / credited per hour (24 h) | 12.0 / 11.7 |
| games per 24 h | 1,977,287 |
| core-minutes per credited card | 16.0 |
| box card table: has / need / stuck / blocked | 24,746 / 142 / 163 / 3 |

**Evidence is need-starved, not capacity-bound, at today's lowering rate.** About 235 cards a day are lowered. evloop
credits 285 a day and plays most of its games as coverage re-deals of cards already supported (13,000 `cov` retries
in 24 h).

The design changes three things:
1. **Burst.** D2-D7 each land 400-1,000 newly lowered cards at once. INFERRED: evloop clears 1,000 in about 1.5-3.5
   days at 12-30 credits an hour. T22 measures the real ceiling on the first D-phase burst and raises `EVL_*` caps
   (in the evidence loop's own configuration) if need waits more than 24 h.
2. **Key stability.** The additive rule keeps existing keys, so no burst of stale re-evidence. A PR that breaks this
   fails the `stale_max` ratchet.
3. **Pods.** support.md says "no 4-seat game is in the evidence runs". So no supported card has pod evidence, and
   Commander pods are the mandated format. kfuzz already plays 3- and 4-seat pods on `-synth` pools (`-seats`,
   `dirac/oracle/cmd/kfuzz/main.go:230`).
   - T22 adds 4-seat focus pools from `edh` and `cmd5` to the evidence corpus, with a per-card `pod` column in
     `evidence-games.tsv`.
   - Requiring pod evidence for Commander-legal cards is OD-4, since the evidence constants are protected.

atomdiff (T02) also lowers the blocked rate. An atom wrong for some object fails in a 30-second test, before evloop
deals the card and burns a try.

## 8. Two-seat cards and 100-card pods

Measured: ALL supported 24,984, and **24,969 hold at any seat count**. 15 supported cards are pod-limited
(`kop.CardPodLimited`, `dirac/kop/twoseat.go:43`).

kgen marks 26 cards two-seat:
- 5 supported, 1 unverified, 20 not lowered;
- 18 of them are "ChoosePlayer among the opponents answered as the one opponent", 3 "ControlledBy TriggeredTarget read
  as OppCtrl", and 1 each for TwoSeatInstr (Vulcan Mind-Meld), a per-opponent graveyard pick (The Weaver King), a
  LosesGame Effect trigger (Miss Highwater), TriggeredDefendingPlayer (Ravenloft Adventurer) and CantAttack OppOwn
  (Xantcha).

Table-level pod limits (`prepTwoSeat`, `dirac/stallclass.go:339`) also stop a pod holding any of:
- speed, packed two seats x 3 bits (`kw_speed.go`);
- the skip-untap marks;
- a battle's protector;
- a granted static goad.

The brief's "42" is this union at an earlier base. Today the measured figure is 15 supported plus 21 marked but not
supported. T23 retires the rest:
- the ChoosePlayer marks through T19's choice over `Opponents`;
- `ControlledBy TriggeredTarget` through SetExpr's controller-of;
- speed widened to four seats x 3 bits (12 bits in the same word);
- skip-untap marks per seat;
- battle protector as a chosen opponent (CR 310.10), which gorge asks;
- the goader seat for granted goad.

Each is a declared two-seat limit today, so retiring one is a capacity change with a lockstep pod repro (`kfuzz -synth
cmd5 -seats 4`).

100-card pods: `MaxObj` 488 and `MaxPerm` 192 were raised on 11-19 for four-seat Commander. `edh` is now the largest
State, which is why OD-1 exists.

## 9. What stays per class

After D1-D9 the remainder is the wave work, done on the new substrate:

| class group | conservative cards | how | who |
|---|---:|---|---|
| 576 API-atom and effect-shape classes (Clone, CopySpellAbility, Animate, Dig, Play, RingTemptsYou, Venture, Learn, Detain, ...) | about 2,236 | | T21: per-class waves; a new effect is an ext sub-op plus a lowering that now composes SetExpr, PredProg and durations instead of shape guards; Sonnet seats where atomdiff covers its operands, Opus for the stateful ones (Ring, Venture, Mutate) |
| 67 non-expanded keyword classes (Banding 38, Mutate 34, Splice 30, Protection 26, Disturb 25, Strive 20, Blitz 17, Riot 15, ...) | 418 | | T20 |
| non-paper (Alchemy, Planechase, Archenemy, stickers, ...) | | parked by name (the 11-19 plan's D1) | |
| `unmodelled-textchange` (2 cards) | | refused, as before | |

## 10. Risks

| risk | where | mitigation |
|---|---|---|
| A generic set or filter evaluator orders objects differently from gorge, and the option order differs | D2, D4 | the order is part of each base's contract; atomdiff compares ordered lists for SetExpr; lockstep compares options |
| gorge's own per-API quirks (an op that reads `Defined$` with LKI where another does not) | D2 | `RefSet` is wired per op; each op's arm states gorge's read; a quirk is a per-op flag on `SetRef`, not a new base |
| `PredProg` evaluated in a matcher whose context lacks a field | D4 | a word declares the contexts it supports; kgen refuses it elsewhere (guard, counted); atomdiff covers every declared context |
| Effect cards break emblem behaviour | D5 | T15 keeps `embRec` readers byte-identical for emblems (an effect record with `card` = emblem); emblem pins and repros in gate step 8 |
| The object-set unification changes behaviour | D3 | kernel-only and lockstep-neutral: full gate, every pin, the synthetic pools; mutation switches `DebugSrcMemNo*` keep working |
| The mana planner slows | D8 | gated per table; FDN, BLB and the `edh` bench measured; refuse rather than slow the hot path |
| `edh` bar breach | D3, D5, D7, D8 | each PR offsets; OD-1 |
| atomdiff misses an atom no seed reaches | all | atomdiff records per-atom coverage (true/false counts); an atom with no true and false hit is listed as unproven and its cards still need evidence |
| Concurrent per-class waves conflict on `filter.go` / `lower_effect.go` | all | lanes (section 11); the wave-4 tickets the design supersedes are paused (section 11) |

## 11. Lanes and order

Each lane is file-disjoint; the lanes run at once. Within a lane, PRs are serial. At most three D-phase PRs sit in the
Mergify queue at once.

| lane | tickets | shared files |
|---|---|---|
| F foundations | T01, T02, T03 | `dirac/kgen/classes*.go`, `dirac/oracle/atomdiff/`, `dirac/*_test.go` |
| K keywords | T04 → T05 → T20 | `engine/cards/keywords.go`, `dirac/kgen/lower.go` (`lowerKeywords`) |
| S sets and memory | T06 → T07 → T08 → T09 → T10 → T19 → T23 | `dirac/kop/refs.go`, new `dirac/set_*.go`, `dirac/zone_srcmem.go`, `zone_exiledwith.go`, `dirac/kgen/lower_effect.go` (`defined`), `lower_remembered.go`, `lower_rempersist.go` |
| P filters and expressions | T11 → T12 → T13a → T13b → T13c → T16 | `dirac/kop/filters.go`, `dirac/chars.go`, `filters_gen.go`, new `dirac/fw_*.go`, `dirac/kgen/filter.go`, `value*.go` |
| E effects and modes | T14 → T15 → T17a → T17b → T17c | `dirac/gen.go` (fx), `static_emblemcold.go`, `dirac/kgen/lower_effect*.go`, `trig_*`, `static_*`, `repl_*` |
| M costs | T18 (after T11) | `dirac/mana_*.go`, `cost_*.go`, `dirac/kgen/cost*.go` |
| V evidence | T22 | `dirac/oracle/cmd/kfuzz`, `dirac/support/`, the evidence loop |
| W tail | T21 waves (after T13, T17) | per class |

Cross-lane dependencies:
- T08, T13 and T16 need T02.
- T10 needs T07.
- T15 needs T07 and T09.
- T17 needs T11 and T14.
- T18 needs T11.
- T19 needs T07.
- T23 needs T19.

Calendar, INFERRED: with four Opus agents on lanes S, P, E and M plus one on F/K, D1-D9 take about 25-30 working days.
Ticket estimates sum to about 122 Opus agent-days, with T13 and T16 on Sonnet seats after T02. W runs on seats in
parallel from about day 15.

## 12. Reproduce

Measured in a gorge worktree detached at `9ce12ae3fb` with `.cards` linked. Every tool is in `repro/`. Heavy runs go in `systemd-run --user --scope -p MemoryMax=8G -p CPUQuota=400%`.

```sh
go build -trimpath -o bin/kgen ./dirac/cmd/kgen
bin/kgen -census-std -all-blockers -cards $PWD/.cards -tsv census-all.tsv     # 5.7 s
bin/kgen -census-std -cards $PWD/.cards -tsv census-first.tsv
# gorge playability: repro/forgecx-zlist.go dropped into a copy of cmd/forgec  -> gorge-play.tsv
# dirac status: repro/supportx-main.go (kgen.SupportFor over ALL.txt)          -> dirac-status.tsv
# paper: repro/legal.py over forge-oracle editions+formats at 418ba5215f      -> legal.tsv
python3 -I repro/gap.py <dir> <dir>/out class      # gapA-class.tsv
python3 -I repro/mech.py <dir>                     # family table, mech-classes.tsv
python3 -I repro/partial.py <dir>                  # hidden params by family
python3 -I repro/sim3.py <dir>                     # section 6.3
make test-one PKG=./dirac RUN='^(TestStateSize|TestEveryPoolFitsTheStateBar)$' V=1
# repro/zz_scratch_measure_test.go.txt as dirac/zz_scratch_measure_test.go: core / perms / per-pool cold
go test -c -o bin/dirac.test ./dirac; cd dirac
GOGC=off GOMAXPROCS=1 taskset -c 20 perf stat -x, -e instructions:u ../bin/dirac.test -test.run XXX \
  -test.bench '^BenchmarkTransitionFDN$' -test.benchtime 500000x   # and 100000x; (b-a)/400000
python3 -I repro/proven.py <farm>/ability-ledger.csv dirac-status.tsv legal.tsv
```

Notes: `NOTES.md` beside this file holds the timestamped measurements.
