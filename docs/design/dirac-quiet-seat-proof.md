# dirac: the quiet-seat proof (skip the priority walk when a seat can only pass)

Status: implemented and consulted (`dirac/cast_quietseat.go`), with a verify arm
(`DebugQuietVerify`) and a shadow arm (`DebugQuietShadow`, on in every
`dirac/oracle` test). Base `e62aa28e17`.

gorge's engine solved the same problem the same way:
[`2026-10-08-quiet-seat-walk-skip-design.md`](2026-10-08-quiet-seat-walk-skip-design.md)
(`engine/rules/quiet_facts.go`, `quiet_grantscope.go`). This note is dirac's
version.

## Problem

In a four-seat pod `stepRaw`'s priority arm (`dirac/engine.go`) runs
`priorityOptions` (`dirac/legal.go`) about 7 times per posed decision. About 91% of
those walks list only pass, and `StepInto` elides them, yet each runs every section:
the hand loop, the graveyard and exile walkers, `activateOffers`, `priorityGen` and
the script walk (`cappedScriptWalk`), whose nested `plainOptions` walks re-run the
whole walk under other mana censuses. Together they were about half of
`BenchmarkTransitionEDH4`'s instructions. Three memo designs over the walk did not pay.

## The proof

`seatQuiet(k, p)` (`quietBlocker == qbNone`) is an over-approximating proof that the
seat holding priority has no option but pass. It never computes an option: it shows
that every route that could add one is ruled out by a cheap necessary condition.
Anything it cannot rule out returns a blocker, and the walk runs exactly as before.
A proved seat's Decision is `[pass]`, the walk's own answer for it. Nothing is added
to State; the facts are dense per-card records on `prep` (`quietCard`), built once
per table by `prepQuiet`; the proof allocates nothing and reads no map or string.

### Obligations

Every non-pass `d.add` reachable from `priorityOptions` (53 routes, the script
walk's nested walks included) is one of these. The proof refuses a sorcery-speed
window outright (`sorcerySpeed`), which removes every sorcery-only route (land plays,
plot, unlocks, embalm and unearth grants, adventure land plays). At instant speed:

| route | necessary condition the proof reads |
|---|---|
| hand: face, face-down, foretell, sneak, offering, may-flash, teamwork routes (ahead of the timing gate) | the card prints none of them (`qHand`) |
| hand: every cast behind the timing gate (`!instant && !sorc && !castFlash`) | not `qInst` (instant or flash on any face), and no live flash grant (`castFlash`, read only when a flash static, self-flash card or the seat's flash emblem exists) |
| hand: the plain cast and its variants | the cast's floor (`quietFloor`: X at 0, Phyrexian 0, hybrid 1) is above the seat's mana bound, unless the card has an open price (`qOpen`: alternative, credited or self-reduced cost) or a cost modifier is live; a card with one cast route (`qPlain`: no kicker, optional or extra cost) is also quiet when `spellTargetsFeasibleFrom` fails, the gate every route of such a card passes |
| hand: activated abilities (cycling) | the least non-sorcery hand-ability floor is above the bound |
| command zone (Commander) | not instant-capable, or the floor is above the bound |
| graveyard casts (flashback, aftermath, disturb, harmonize, beam, graveyard warp, the card's own graveyard alternative cost) | not instant-capable, or no route: no printed route (`qGyCast`), no stamped or static flashback grant, no beam grant |
| escape, mayhem, retrace; graveyard activated abilities; the card's own graveyard may-play static | the card prints none (`qGrave`) |
| exile casts (adventure, prepared copy, paradigm, foretell, warp, airbend) | not instant-capable, or no record / mark |
| exile activated abilities; own exile may-play static | the card prints none (`qExile`) |
| play permission records | no record controlled by the seat |
| play-from-zone statics (`priorityMayPlay`) | `mayPlayZones(p) == 0` |
| battlefield activated abilities (`activateOffers`) | each own non-sorcery ability's floor is above the bound; a `{T}` one only while untapped (`CostSorcery` without an equip-flash static and `CostLoyalty` without a loyalty-flash static are sorcery) |
| granted activated abilities | the least floor over the latent non-sorcery bodies of the live grant sources (battlefield cards, override records' cards; any emblem grant reads 0) is above the bound for every slot `grantMay` admits |
| turn face up | no face-down permanent of the seat |
| unless-source priority activation (`priManaAskOffers`) | not armed |
| script walk (`grantCap`, `scriptOffers`) | it adds only options a nested walk lists; every condition above holds for every census (they read no census, or the all-census bound) |
| filters (`dropNoManaCostCasts`, `filterSplitSecond`) | remove only |

A table with a route the proof does not model runs the walk for every seat
(`quietOff`): an any-player activation, an exile grant, gained abilities, piles,
conditional split mana, escape or retrace grants, cycling grants.

### The mana bound

`quietBound` over-approximates the mana the seat could spend on one payment in any
census the walk prices (the plain census, the script walk's gap and any-colour
censuses): the floating pool; per controlled permanent, the largest amount one `{T}`
of a printed mana ability makes (untapped only) plus what its non-`{T}` abilities make
(a sacrifice once, a net-zero filter nothing, anything repeatable unbounded); a Spirit
Guide in hand and a graveyard mana source; at least one mana per untapped permanent
under a basic land type grant, and at least the largest granted `{T}` amount under a
live mana grant. Restricted floating mana, a tap-extra static, a dynamic amount or a
granted mana ability without `{T}` makes it unbounded. Summoning sickness, colour and
the source's own tap are ignored: each only overcounts.

A cost modifier is live (`quietModLive`) when any emblem exists or a battlefield card
prints a static of a kind `quietHarmless` does not clear (cost reductions, alternative
and set costs, stack keyword grants, warp and other cast grants, tap-extra, a may-play
`{0}` alternative); every floor then reads 0. A static kind added to `kop` later is not
harmless until it is classified (`TestQuietHarmlessKinds`).

### Round 2: exact reads where round 1 zeroed or overcounted

Each read below replaces a coarser one; each is still a necessary condition.

- **ReduceCost read as an amount.** A `ReduceCost` static whose `Zone` is
  `ZoneCommand` works only on its own card (`adjustCostGenB` applies it to the card
  being cast; `eachStaticC` leaves it out of every other spell's price), so it opens
  that card's cast (`qOpen`) and lowers nothing else: Ghalta, Spire Golem, Memory
  Guardian. A battlefield one lowers a spell's generic part by its constant `A`
  (`adjustCostGenB` subtracts `A`, or a dynamic or emblem-numbered amount, which
  reads unbounded). The proof subtracts the sum of every live battlefield
  `ReduceCost` amount, any controller's, from the cast floor's generic part; pips
  are untouched, as `ReduceCost` takes generic only. A colour-reducing one
  (`StReduceColor`) stays a cost modifier that zeroes every floor. `ReduceCost`
  prices spells only, so it no longer zeroes activation floors (an ability's
  reduction is `StatReduceAbility`, still a modifier).
- **Colours.** `quietBound` also bounds the mana of each colour: a source counts
  for every colour its printed mana abilities name (every colour for a chosen,
  reflected, each-colour, identity, dynamic or any-colour ability, colourless with
  any colour), the pool per kind, a granted or land-type unit for every colour. A
  filter (no `{T}`, nets nothing) turns any mana into its colours, so those colours
  read the whole bound; an any-colour or any-type static (`StatAnyColorAll`,
  `StatAnyColorSpells`, `StatAnyTypeCreatures`) turns the colour read off. A cast or
  activation with more pips of one colour than that colour's bound is ruled out.
  Hybrid pips are not read (an over-count).
- **canActivate's census-free gates, ability by ability.** An own battlefield
  ability is offered only through `canActivate` (`activateOffers`, and the same call
  in the script walk's nested walks), so each early `return false` that reads no
  census is a necessary condition: `{T}` on a tapped or summoning-sick source, a
  counter removal the source cannot pay, a sacrifice with too few candidates
  (`countCostSac`), a once-per-turn ability already used, an activation condition
  (`activateCondHolds`), the ability's gate (`gateOK`), and the final target check
  (`abilityTargetsFeasible`, or none for an announced-X sacrifice). The proof reads
  them only for a permanent whose least floor is within the bound.
- **A `{T}` ability's own source.** `abilityAffords` keeps the source's own tap out
  of the census that pays its `{T}` ability, so the proof compares that ability's
  floor with the bound less the source's own `{T}` amount.

### State the walk writes

A proved window must leave State as the walk would. The verify arm hashes State
(`gameHash`) before and after every proved walk. The only write seen is
`genState.fbp`, the `filtBoundPays` memo: its key holds every input of its answer
(slot, seat, stack depth, pool, SBA fingerprint, object-write count, probe, line), so
its content is never an input to an answer, only a cache; a proved window leaves the
older entry, and the next lookup either matches a key computed from the same inputs
or recomputes. The verify arm compares State with `fbp` restored and counts these
windows (`MemoOnly`). The capacity latches the walk can set (`capFull` in the
zero-alternative, emblem-alternative, retrace-grant, pay-life and grant-search
routes) are all behind routes the proof rules out.

## Verify numbers (base e62aa28e17, every walk run beside the proof)

| run | walks | pass-only | proved / all | proved / pass-only | mismatches | State diffs (beyond fbp) |
|---|--:|--:|--:|--:|--:|--:|
| edh pods, `Config{Mulligans: 2}`, random, 100k transitions | 702,331 | 642,597 | 63.5% | 69.4% | 0 | 0 |
| edh pods, `LockstepConfig(SurfaceAutopay)`, random, 100k | 719,323 | 660,159 | 60.4% | 65.8% | 0 | 0 |
| fdc pods (dpod), auto-pay, heuristic bot, 100k decisions | 653,719 | 595,031 | 49.8% | 54.7% | 0 | 0 |
| fdc pods (dpod), auto-pay, random, 100k decisions | 666,026 | 605,782 | 55.6% | 61.2% | 0 | 0 |
| FDN bench pairings, random, 200k (half posed) | 535,273 | 466,324 | 65.9% | 75.7% | 0 | 0 |

The committed tests run these with the verify panicking on a mismatch:
`TestQuietProofEDH`, `TestQuietProofEDHAutopay`, `TestQuietProofFDN`
(`dirac/cast_quietseat_test.go`) and `TestPodQuietProofHBot` (`dirac/cmd/dpod`). Every
`dirac/oracle` test runs with `DebugQuietShadow` (no tally, parallel-safe).

The proof costs about 2,200 instructions per call on the edh pods (one extra call per
walk adds 15,178 instr/op at 7.0 walks per transition), against about 43,000 saved per
proved walk.

## Measured (instructions, one locked core, base e62aa28e17)

| bench | base | proof | change |
|---|--:|--:|--:|
| BenchmarkTransitionEDH4 | 1,027,438 | 837,302 | -18.5% |
| BenchmarkTransitionFDN | 67,100 | 60,884 | -9.3% |

kpod cmdr4 (104 four-seat pods, four searched seats, sims 50): every chunk log and
corpus file byte-identical to the base run; 282.6 CPU-s against 320.1.

## Round 2 numbers (base main 56af2f5cc2 with #5986's branch merged)

Every walk run beside the proof: 0 mismatches, 0 State differences beyond fbp.

| run | round 1 proved / pass-only | round 2 |
|---|--:|--:|
| fdc pods (dpod), auto-pay, heuristic bot, 100k decisions | 54.7% | 62.9% |
| fdc pods (dpod), auto-pay, random, 100k | 61.2% | 71.2% |
| edh pods, `Config{Mulligans: 2}`, random, 100k | 69.4% | 76.8% |
| edh pods, auto-pay, random, 100k | 65.8% | 73.4% |
| FDN bench pairings, random, 200k | 75.7% | 83.8% |

| bench (instr/op, core 13) | main | round 1 | round 2 |
|---|--:|--:|--:|
| BenchmarkTransitionEDH4 | 1,027,411 | 837,911 | 818,555 |
| BenchmarkTransitionEDH4Autopay | 942,920 | 763,447 | 742,088 |
| BenchmarkTransitionFDN | 67,099 | 61,214 | 61,592 |

kpod cmdr4 (qs4-2 against runs/m4-1): byte-identical; 271.1 CPU-s (round 1 282.6,
base 320.1).

Unproved pass-only walks on the hbot fdc pods after round 2, by first blocker: hand
14.0% (10.8% an instant the walk drops though its floor, colours and targets fit:
X spells on the auto-pay surface, restricted sources, identity sources overcounted
as every colour), battlefield 11.5% (8.5% within the bound, 3.0% free abilities),
sorcery-speed windows 4.8%, grants 2.9%, others under 1.5% each.

## Not covered

- Sorcery-speed windows (about 3% of pass-only walks).
- Unproved pass-only windows, by first blocker (edh, random): battlefield abilities
  within the bound 13%, hand instants within the bound 7%, hand routes 2%, graveyard
  2%. Most of these are affordable abilities the walk drops for a reason the proof
  does not read (targets, activation conditions, sacrifice fodder). A per-ability
  read of `canActivate`'s census-free gates (targets, counters, sickness) is the next
  widening.
- Tables with a route the proof does not model (`quietOff`).
- The proof is read only at stepRaw's posed priority; nested and potential walks run
  as before.
