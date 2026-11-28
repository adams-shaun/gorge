# Where a four-seat transition's instructions go (2026-11-28)

Profile of `BenchmarkTransitionEDH4` (dirac/benchedh_test.go) at main's head, made before any change
aims at the 400,000-instruction target for a four-seat transition. It replaces the 2026-11-24 profile
(70% in `priorityOptions`, 33% in the mana planner), which predates the static-source cache memo
(`manaPCGenMemo`, docs/design/dirac-static-source-cache.md). No code in `dirac/` changed for this.

## What was measured

| | |
|---|---|
| commit | `2872018f7b140ebc862df96e27518529dfdea19e` (main, "Merge pull request #5731") |
| corpus pin | `FORGE_REF` 5bc5851a54a0fbbbf7f1a38b48d8ff9b7714ddb2; `.cards/cards.lock` digest `3c5d840e8efc9b41` (the 16-hex key `scripts/dirac-gate.sh` caches under), 34482 card files, 864 token files |
| edh slice | `dirac/oracle/testdata/slices/edh/decks.tar.gz` as committed at that commit |
| toolchain | go1.26.3 linux/amd64, `perf` from `/usr/bin/perf` |
| host | AMD Ryzen 9 9950X (8 cores visible), core 7 (the gate's `INSTR_CORE=20` does not exist on this host) |
| environment | `GOGC=off GOMEMLIMIT=3GiB GOMAXPROCS=1 taskset -c 7`, run from `dirac/` |

**The binary.** The ticket asks for the gate's `head.test` (`go test -c -o head.test ./dirac/`). The
implementer seat that made this profile refuses `go test` in every form, `-c` included, so the
profile ran on a `go build` of a small driver that is a line-for-line copy of the bench loop over
the same `dirac` package. The driver used the same 16 pods (candidate c = sorted decks c, c+7, c+14,
c+21, with the same refusal skips), one untimed warm-up game per pod, game g = pod g%16 under seed
g+1 with `NewRandomPolicy(g+1)`, the same per-game panic recovery, and N transitions in the measured
loop. Only the `testing.B` harness is missing, and the differencing below cancels its fixed cost.
None of the 325 games in the N=100000 run ended in a kernel panic. Re-measuring on a real
`dirac.test` is the first item under "Not measured".

## Instructions per transition (the gate's step 5 method)

`perf stat -x, -e instructions:u` over the whole process at N=20000 and N=100000, differenced:
per op = (I(100000) − I(20000)) / 80000. The gate itself uses 500k/100k. This uses the ticket's
20k/100k pair because a four-seat run of 500k transitions is long.

| run | I(20000) | I(100000) | per op |
|---|---:|---:|---:|
| 1 | 21,792,256,826 | 102,979,588,668 | **1,014,841** |
| 2 | 21,792,804,544 | 102,980,875,607 | **1,014,850** |

The two runs agree to 9 instructions per op. The rest of this note uses **1,014,846** (their mean).
That is 2.54× the 400,000 target: a four-seat transition has to shed about 615k instructions, 61%.

## Transitions per decision kind

The driver's `-kinds` mode read an `instructions:u` counter (`perf_event_open`, this thread, user
space only) before `StepInto` and after `Apply`. It charged each interval to the kind of the decision
`StepInto` produced, over the first 100,000 measured transitions. An interval includes the
`StepInto` that built the decision (so the option walk, `priorityOptions` for a priority) and the
`Apply` of the policy's pick. A game restart (`InitN`, 325 of them) is charged to the transition
after it.

| kind | transitions | share | instr / transition | share of instr | mean options |
|---|---:|---:|---:|---:|---:|
| Priority | 58,543 | 58.5% | 1,242,059 | 74.5% | 3.5 |
| Attackers | 13,092 | 13.1% | 827,307 | 11.1% | 6.1 |
| Choose | 12,647 | 12.6% | 620,679 | 8.0% | 13.8 |
| Target | 6,324 | 6.3% | 226,231 | 1.5% | 7.5 |
| Blockers | 4,939 | 4.9% | 635,735 | 3.2% | 4.4 |
| Mulligan | 1,941 | 1.9% | 207,310 | 0.4% | 2.0 |
| TriggerOrder | 1,852 | 1.9% | 654,225 | 1.2% | 3.1 |
| Modes | 662 | 0.7% | 100,145 | 0.1% | 2.4 |
| all | 100,000 | | 976,456 | | |

This mean (976k) covers the first 100k transitions and their game restarts. The marginal figure
above (1.015M) covers transitions 20k to 100k, so the two differ.

A priority transition costs 1.24M. The other 41,457 transitions average **601k**, which is itself over
the target. A non-priority transition still runs `stepRaw`'s turn flow: SBA passes (`sba` is 14.8% of
all instructions, none of it under `priorityOptions`), `stepGen`/`refreshTy` and the combat option
walks. So the 400k target cannot be reached through `priorityOptions` alone. Removing all of its
665k per op would leave 349k per op, just under the target.

## priorityOptions and its direct callees

`priorityOptions` (dirac/legal.go) takes **65.58%** of `instructions:u` inclusive (665.5k per op) and
1.34% self. Almost all of it is reached from `stepRaw` (64.98%; perf names the inlined
`pcMemoClose` frame as the caller because it shares the call site's line range). The other 0.59%
is the nested call through `plainOptions`, described next. Direct callees, for the outermost
`priorityOptions` frame on each stack:

| callee | share of all instr | instr / op |
|---|---:|---:|
| `scriptWalk` → `scriptOffers` (17.72%), `slotMask.any` (3.06%) | 20.96% | 212.7k |
| `activateOffers` (inlined; `canActivate` 20.97%, `availForMode` 11.18% overall) | 17.74% | 180.0k |
| `priorityGen` (`availForMode` 6.46%, `slotMask.first`→`grantedAbilities` 5.80%, `priorityMayPlay` 2.23%) | 10.61% | 107.7k |
| `grantCap` (`plainOptions` 7.04%, `grantSearch` 2.92%, `grantRelaxCensus` 2.22%, `grantGapLive` 1.99%) | 8.90% | 90.3k |
| `availOnce` | 2.60% | 26.4k |
| self | 0.98% | 9.9k |
| `offerRetraceCasts`, `Decision.add`, `castPaysUnderPain`, `adjustFit`, `tapPays`, `spellTargetsFeasibleFrom`, `offerTurnFaceUp` | 0.25–0.45% each | 2.5k–4.6k each |

`activateOffers` is 32.59% inclusive, but only 17.74% sits directly under the outermost
`priorityOptions`. The rest runs inside a **nested** `priorityOptions`: `plainOptions`
(dirac/script_gen.go) re-runs the whole priority walk under another `availFor` mode. Stacks with
two or more `priorityOptions` frames hold **24.64%** of all instructions (`plainOptions` 25.09%
inclusive). They are reached from `plainGapsOptions` (11.61%, once per subset of net-negative
filters, under `scriptOffers`), `grantCap` (5.89%), `restrOptions` (4.04%, at the prefix search's
nodes), `mergeBareGaps` (3.05%) and `scriptPrefix` (0.42%).

## Top 25 by self share (`perf report --no-children`)

`perf record -e instructions:u -g` (frame-pointer call chains) at N=100000: 36,435 samples, about
2.8M instructions per sample, so one sample is 0.003%. The process total includes setup (pod
build, warm-up): I(20000) − 20000 × 1,014,846 ≈ 1.50G instructions, 1.5% of the process at
N=100000. Shares are of the whole process.
Instructions per op are share × 1,014,846, an approximation that charges setup to transitions.

| # | function (`dirac.`) | self | ≈ instr / op |
|---:|---|---:|---:|
| 1 | `(*State).grantedAbilitiesOwn` | 5.06% | 51,351 |
| 2 | `(*State).charsPrinted` | 4.27% | 43,333 |
| 3 | `(*State).eachStaticOf` | 3.17% | 32,170 |
| 4 | `(*State).grantSrcScan` | 3.11% | 31,561 |
| 5 | `(*State).chars` | 3.07% | 31,155 |
| 6 | `(*State).charsFxN` | 2.87% | 29,126 |
| 7 | `(*State).matchPermOne` | 2.46% | 24,965 |
| 8 | `(*State).contGrantsIn` | 2.42% | 24,559 |
| 9 | `(*State).loss6` | 2.09% | 21,210 |
| 10 | `(*State).charsFxMay` | 2.06% | 20,905 |
| 11 | `(*State).matchPermGenCw` | 1.90% | 19,282 |
| 12 | `(*State).lfxCollectOne` | 1.83% | 18,571 |
| 13 | `(*State).manaPCGenCalc` | 1.79% | 18,165 |
| 14 | `(*State).lfxCollectStatics` | 1.66% | 16,846 |
| 15 | `(*State).leaveGame` | 1.58% | 16,034 |
| 16 | `(*State).manaGrantMay` | 1.55% | 15,730 |
| 17 | `(*State).typesGen` | 1.48% | 15,019 |
| 18 | `(*State).genLost` | 1.37% | 13,903 |
| 19 | `(*State).priorityOptions` | 1.34% | 13,598 |
| 20 | `(*State).eachCantActStatic` | 1.25% | 12,685 |
| 21 | `(*State).manaPCGenMemo` | 1.24% | 12,584 |
| 22 | `(*State).priorityMayPlay.func1` | 1.22% | 12,381 |
| 23 | `(*State).availForMode` | 1.15% | 11,670 |
| 24 | `(*State).ovrLoses` | 1.13% | 11,467 |
| 25 | `(*State).grantIdxs` | 1.12% | 11,366 |

The self profile is flat: the top 25 hold 52% and no function holds more than 5.1%. Making a
leaf faster will not reach the target. The cost is in how often the walks above re-derive the same
characteristics, grants and mana views.

## Top 25 by inclusive share (`perf report --children`)

`dirac` frames only. The `runtime.main`/`main.main` frames above `StepInto` (99.3–99.98%) are left
out. "(inlined)" frames are perf's inline expansion, so a frame's inclusive share can equal its caller's.

| # | function (`dirac.`) | children | self | ≈ instr / op |
|---:|---|---:|---:|---:|
| 1 | `(*State).StepInto` | 95.55% | 0.03% | 969,685 |
| 2 | `(*State).stepRaw` | 92.99% | 0.14% | 943,705 |
| 3 | `(*State).priorityOptions` | 65.58% | 1.34% | 665,536 |
| 4 | `(*State).pcMemoClose` (inlined) | 62.93% | 0.00% | 638,642 |
| 5 | `(*State).activateOffers` | 32.59% | 0.39% | 330,738 |
| 6 | `(*State).availForMode` | 29.68% | 1.15% | 301,206 |
| 7 | `(*State).manaPCGen` | 24.30% | 0.36% | 246,607 |
| 8 | `(*State).manaPCGenMemo` | 23.83% | 1.24% | 241,837 |
| 9 | `(*State).manaPCGenCalc` | 22.33% | 1.79% | 226,615 |
| 10 | `(*State).grantedAbilities` | 21.18% | 0.35% | 214,944 |
| 11 | `(*State).canActivate` | 21.01% | 0.15% | 213,219 |
| 12 | `(*State).scriptWalk` | 20.97% | 0.01% | 212,813 |
| 13 | `(*State).grantedAbilitiesOwn` | 20.75% | 5.06% | 210,580 |
| 14 | `(*State).filtScript` | 20.19% | 0.01% | 204,897 |
| 15 | `(*State).actSourcesAfford` | 19.86% | 0.04% | 201,548 |
| 16 | `(*State).actScriptPaysCost` | 19.50% | 0.01% | 197,894 |
| 17 | `(*State).actScriptPays` (inlined) | 19.50% | 0.00% | 197,894 |
| 18 | `(*filtSearch).visit` | 19.07% | 0.11% | 193,531 |
| 19 | `(*filtSearch).try` | 18.83% | 0.02% | 191,095 |
| 20 | `(*State).scriptOffers` | 17.73% | 0.23% | 179,932 |
| 21 | `(*filtSearch).otherGap` | 16.62% | 0.01% | 168,667 |
| 22 | `(*filtSearch).otherGapTwin` | 16.62% | 0.00% | 168,667 |
| 23 | `(*filtSearch).otherGap.func1` | 16.58% | 0.00% | 168,261 |
| 24 | `(*State).priorityGen` | 16.55% | 0.74% | 167,957 |
| 25 | `(*State).grantIdxs` | 16.48% | 1.12% | 167,246 |

Just past 25: `chars` 16.31%, `filtSearch.terminal`/`offers`/`offersOnPath` 16.3%, `matchPermGenCw`
15.00%, `sba` 14.84%, `typesGen` 14.18% (14.55% counting every frame), `matchPermOne` 13.07%,
`plainGapsOptions` 11.62%, `scriptFilt` 10.01%. The mana planner as the 11-24 profile counted it
(`manaPCGen` and its memo) is now 24.3% inclusive, against 33% then. The two profiles split the
planner differently, so that comparison is approximate.

## Memoisation candidates (ranked, at most five)

"Saved if free" is the candidate's inclusive share × 1,014,846. It is an upper bound: a memo costs
its lookups and its misses, and the candidates overlap (table below). The union of all five
subtrees is 66.84% (678k per op), so making all five free would save at most 678k, not their sum.

1. **The nested priority walk: `plainOptions` → `priorityOptions`** (script_gen.go). It
   recomputes the whole option list for the same player and the same board under a different
   `availX` mode (`availPlain|availGaps`, `availPlain|0`, `availPlain|prefixAnyProbe`) and
   `filtSkip` subset, inside one outer priority walk. `plainGapsOptions` runs it once per
   subset of the net-negative filters. `restrOptions`/`restrOptionsProbe` run it at each node of
   the prefix search, and `grantCap` and `mergeBareGaps` run it once each. **Invalidated by:** any
   object write (`State.touchN`), the `availX` bits and `filtSkip` subset it runs under, the root
   Exhaust bits `withRootExhaust` swaps into `g.turnF`, the player's floating mana pool (it differs
   between prefix-search nodes), and the stack. A per-outer-walk table keyed by those would answer
   repeats. **Saved if free: 25.09%, about 254.6k per op.** How many of the nested calls repeat a
   key was not measured, so the achievable share is unknown.
2. **`manaPCGenCalc` misses of the existing memo** (mana_pcmemo.go). It recomputes slot sl's
   mana-source view from granted abilities (`grantIdx` 8.89%), reflected colours
   (`manaReflView` 3.74%), land-type grants (`blgView` 2.66%), `actForbidden`, gates and losses.
   The memo answers only at pool depth 0, outside filter search (`inFilt`), census windows and
   in-place probes (`pcMemoHold`), and under the same `availX`. So every call from a nested
   `plainOptions` walk (different `availX`) and from the filter search misses. 22.19% of the 22.33%
   reaches `manaPCGenCalc` through `manaPCGenMemo`. **Invalidated by:** `touchN`, the memo
   generation (`pcMemoOpen`/`pcMemoDrop`), and only those `availX` bits the view reads. Keying
   on fewer bits, or keeping entries live under `inFilt`, would turn misses into hits.
   **Saved if free: 22.33%, about 226.6k per op.**
3. **`grantedAbilities` per slot** (chars_gen.go `grantedAbilitiesOwn`, the top self entry). For each
   ask about a slot it rescans every static source (`grantSrcScan` over `statM`, `contGrantsIn`,
   `eachStaticOf`, `globalGrantsGenIn`). Each source's match runs `matchPermGenCw` →
   `matchPermOne` → `typesGen`. It is asked through `grantIdxs`/`grantIdx` (15.3%, mostly from
   `manaPCGenCalc` and `manaGranted`) and from `priorityGen`'s slot loop (5.7%). **Invalidated
   by:** `touchN` (any object write moves a grant's source, its target or the target's
   characteristics), layer timestamps (`layerTs`) and ability-loss effects. A per-slot,
   per-decision list of (card, ability, count) keyed by `touchN`, opened and closed like the
   pcMemo, would answer repeats. **Saved if free: 21.18%, about 214.9k per op.**
4. **The SBA pass on an unchanged state** (`sba` in turn.go, `sbaBoard` in static_sbaskip.go). It re-checks every
   state-based action on every `stepRaw` loop: `sbaBoard`→`sbaScan` 7.25%, `sbaClean` 1.82%,
   `initSBA` 1.53%, `legendRule` 1.17%. `sbaBoard` already skips its scan when `touchN` and
   `sbaFingerprint` match the last clean pass, but 6.88% still reaches `sbaScan` (a miss or
   `anyEmb`), and the rest of the pass is never skipped. **Invalidated by:** `touchN` plus the
   fingerprint's non-object fields (life totals, turn/step, the per-seat tallies). Skipping the
   whole pass, not just the board scan, on a fingerprint hit is the extension. Its subtree overlaps
   none of candidates 1–3. **Saved if free: 14.84%, about 150.6k per op.** It is the largest
   saving open to non-priority transitions.
5. **`typesGen` → `walkTypes` on walked slots** (chars_gen.go). It recomputes sl's types through
   the CR 613 layer walk (`chars`, `charsPrinted`, `charsFxN`, `lfxCollect`) on every call. It is
   called 9.22% from `matchPermOne` (grant and static matching) and 5.09% from `refreshTy`, which
   runs before every step and resolution op. `refreshTy` stores results into `perm.gty` only for
   override slots. **Invalidated by:** `touchN`, layer timestamps and the turn-dependent effect
   bits a CDA or conditional static reads. **Saved if free: 14.55%, about 147.7k per op.**

Overlap between the candidates' subtrees, as a share of all instructions (diagonal = inclusive):

| | 1 plainOptions | 2 manaPCGenCalc | 3 grantedAbilities | 4 sba | 5 typesGen |
|---|---:|---:|---:|---:|---:|
| 1 | 25.1 | 9.4 | 11.1 | 0.0 | 3.4 |
| 2 | 9.4 | 22.3 | 7.2 | 0.0 | 4.9 |
| 3 | 11.1 | 7.2 | 21.2 | 0.0 | 3.3 |
| 4 | 0.0 | 0.0 | 0.0 | 14.8 | 1.3 |
| 5 | 3.4 | 4.9 | 3.3 | 1.3 | 14.6 |

Left off the list: `scriptFiltCalc` (9.73%). It already has a memo (mana_sfmemo.go) with a wide key,
and 3.7 of its 9.7 points are under candidate 1.

## Not measured

- **The gate's own `head.test`.** The seat refused `go test -c`, so these numbers come from a
  `go build` driver that copies the bench loop. The differenced per-op count should match
  `dirac.test` to within the harness's per-iteration cost, but nobody ran `dirac.test` here. The
  first follow-up is `perf stat` on `go test -c -o head.test ./dirac/`, run exactly as above.
- The gate's 500k/100k differencing for EDH4, the FDN/BLB benches, and the gate's runner host and
  `INSTR_CORE=20`. Core 7 of an 8-core host under a load average of about 68 was used instead.
  `instructions:u` counts do not depend on load; wall time was not recorded.
- Memo hit rates: `pcMemo.hits`, the scriptFilt memo's hits and the SBA skip's hits are
  unexported. They were not read, so how much of each candidate is repeats (and can be saved)
  versus first computations is unknown. Equally, nobody counted how many nested `plainOptions`
  walks each priority walk runs, or how many share a key.
- Per-kind profiles: the call graph is for the whole run, not split by decision kind. The kind
  table's per-kind figures include the two counter reads per transition, which add a few hundred
  instructions of user-side syscall stub.
- The cost of setup inside the profile: shares are of the whole process, about 1.5% of which at
  N=100000 is pod building and warm-up games. The "≈ instr / op" columns are therefore slightly high.
- How exact perf's inline attribution is. With Go, perf names inlined frames by line ranges, so
  the frame named as a caller can be wrong (`pcMemoClose` as `priorityOptions`' caller, and
  `slotMask.first`/`slotMask.any` as callers of `grantedAbilities`/`scriptFilt`).
