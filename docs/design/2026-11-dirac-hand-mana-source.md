# A hand-zone mana source (Spirit Guides) in the kernel's payment window

Date: 2026-11-12.
Status: **design only.** No kernel, kgen or gorge code changes; the probes this
note quotes are in `dirac/oracle/spiritguide_probe_test.go`. The implementation
ticket is the last section. This is Branch B of the Spirit Guide scoping: the
guide is a source in the kernel's manual payment window **only**, never an
"activate" at priority (`dirac/oracle/doc.go` "Autopay priority vocabulary"
stays as it is).

## Problem

kgen refuses Simian and Elvish Spirit Guide
(`AB$ Mana | Cost$ ExileFromHand<1/CARDNAME> | Produced$ R|G | ActivationZone$ Hand`):
`handManaRefusal` in `dirac/kgen/lower_handmana.go`, called from the `case "AB":`
arm of `dirac/kgen/lower.go` before `costHand`. The refusal is right while the
kernel has no hand source: gorge lists the guide in a payment window and the
oracle's answer takes it once nothing on the battlefield is left, and a kernel
that ignored hand cards would then diverge.

## 1. What gorge does (Q1)

### Which engine sites range hand sources

`pay.ManaSourceIDs` (`engine/rules/pay/board.go`) is the payer's battlefield
sources (`BattlefieldManaSourceIDs`: own permanents first, then the other seats'),
then the payer's `ZHand`, then the payer's `ZGraveyard`. Its callers decide which
windows can list a hand card (grep `ManaSourceIDs(` in `engine`, test files
excluded):

| Site | Window | Ranges |
|---|---|---|
| `manaWindowAsk` (`engine/rules/cast_paymentplan.go`) | a cast's CR 601.2g window (prompt "Activate mana abilities to pay for <spell>") | `ManaSourceIDs`, hand included |
| `announcedManaWindowAsk` (`engine/rules/announce_pay.go`) | an announced {X} window | `ManaSourceIDs`, hand included |
| `unlessManaWindowNeeded` via `hasUntappedManaSourceFor` (`engine/rules/unless_mana.go`, `cast_paymentplan.go`) | whether an unless-cost mana window opens at all | `ManaSourceIDs`, hand included |
| `paymentManaAskClass` (`engine/rules/cumulative.go`) | a trigger's own cost window | `ZBattlefield` of the payer only |
| `castWindowProbeUnits`, `castWindowCensusComplete` (`engine/rules/cast_window.go`), the attack-tax window (`engine/rules/attack_cost.go`) | censuses and the declare-attackers tax window | `BattlefieldManaSourceIDs` only |

So a hand card can be listed by cast and announced-X windows (measured below),
and may be by an unless-cost window (the code opens one for a payer whose only
source is a hand card; **not measured**, see "Unmeasured edges"). A trigger-cost
window never lists one.

### Option order and the answer rule

A window that lists a guide lists, in order: every battlefield source (an
"activate" option each), then one "activate" option per guide in the hand (in
hand order), then the non-source rows (done, cancel). The oracle answers through
`payexec.WindowChoices` / `WindowAnswer` (`bots/spellbench/payexec/payexec.go`),
called from `internal` in `dirac/oracle/gorge.go`. `WindowAnswer` checks, in
order: (1) an auto-fill option; (2) the first "mana"/"activate" option whose
`ManaSymbol` is an owed colour; (3) the lowest WUBRGC single-symbol "mana"
option, else the first "activate" option; (4) done/cancel. A manual window's
options are "activate" rows with no `ManaSymbol`, so (2) never matches and (3)
takes the first "activate" row: **the first battlefield source if one is listed,
else the first hand guide.** A window is one activation: a payment that needs two
mana is two windows.

Probe `TestSpiritGuideWindowOrder` (120 seeds, a 600-decision cap per game; B a
battlefield source's option, H a hand guide's, x a non-source row; the key is the
option letters, then the letter of the option the answer took) asserts exactly
this and logs, for the forced state (`tapAll`: every untapped caster permanent
tapped at the target ask, after the cast was announced):

    tapAll Elvish Spirit Guide: windows=600 lists=600 picks=600
      shapes=map[HHHHHHx->H:1 HHHHHx->H:16 HHHHx->H:53 HHHx->H:109 HHx->H:184 Hx->H:237]
    tapAll Simian Spirit Guide: windows=499 lists=499 picks=499
      shapes=map[HHHHHHx->H:1 HHHHHx->H:6 HHHHx->H:32 HHHx->H:81 HHx->H:146 Hx->H:233]

and for `tapOne` (the caster's first untapped permanent tapped, so some windows
keep a battlefield source), for example:

    tapOne Elvish Spirit Guide: windows=452 lists=452 picks=236
      shapes=map[BHx->B:33 BHHx->B:34 BHHHx->B:48 ... HHx->H:76 Hx->H:84 ...]

Every shape with a B ahead of an H is answered B (216 windows); every shape with
no B is answered with the first H (236 windows); no shape lists a B after an H.
**The guide is answered iff no battlefield source is listed.**

### A correction to the earlier numbers

The earlier report counted "windows with a caster source left" (50/266 Elvish,
9/85 Simian answered with the guide). `casterSources` in the probe file counts
every untapped permanent of the caster, not only mana sources: a Grizzly Bears
or a Spirit Guide cast as a creature (its mana ability works from the hand only)
counts. `TestSpiritGuideWindowOrder` tallies, for windows that listed the guide
but no B, the untapped permanents they held:

    tapOne Elvish Spirit Guide: unlisted=map[Elvish Spirit Guide ...:41 Grizzly Bears ...:31]
    tapOne Simian Spirit Guide: unlisted=map[Simian Spirit Guide ...:17]

and asserts that none is a Forest or a Mountain. So the guide was never answered
while a real mana source was listed.

### The colour clamp applies to the guide

Gorge withholds a source that cannot add a colour the payment still absorbs
(`paymentSourceUseful`); `TestSpiritGuideOffColour` (tapAll, 120 seeds) shows it
does the same to a hand guide:

    Simian Spirit Guide with Giant Growth: windows=3929 windowsHoldGuide=3929 lists=0 picks=0
    Elvish Spirit Guide with Lightning Bolt: windows=5033 windowsHoldGuide=5033 lists=0 picks=0
    Elvish Spirit Guide with Arc Lightning: windows=2247 windowsHoldGuide=1541 lists=1317 picks=1317
      byPool=map[listed=false pool=2:224 listed=true pool=0:925 listed=true pool=1:392]

A red guide is never listed for {G}, a green one never for {R}. For {2}{R} the
green guide is listed while the pool holds 0 or 1 mana (the generic {2} is open)
and withheld at 2 (only {R} is owed). That is the kernel's own clamp:
`windowUseful` in `dirac/mana_window.go` returns `want` (the pip colours the pool
still lacks) and `decided` (the pool's surplus covers the generic), and
`windowPick` skips a source whose mask misses `want` when `decided`.

### Natural play

`TestSpiritGuideGorgeWindow` (existing) measures natural play: 0 windows of any
kind in 120 games per guide, no cast is ever offered on a hand guide alone
(`castNoLandOneGuid` 0, with `noSourceGuide` 3190 / 2975 priorities that could
have shown one), and gorge's raw priority offers the guide's "activate" every
time (`rawActivateOffer` equals `guideInHand`: 44555 and 57521) while the
oracle's canonical list never does.

`TestSpiritGuideXWindow` (200 seeds, Banefire's announced {X}, the one natural
window shape the decks reached):

    Simian Spirit Guide with Banefire: windows=5 holdGuide=2 lists=2 picks=0 shapes=map[BHHx->B:2]
    Elvish Spirit Guide with Banefire: windows=0 holdGuide=0 lists=0 picks=0

A natural window can list the guide, and never takes it (a battlefield source is
ahead of it and the plan covers what is owed). The guide is picked only where the
battlefield is dry, which natural play did not reach in these decks.

## 2. Where a hand source sits in the kernel's window walk (Q2)

`windowTapWalk` (`dirac/mana_window.go`) loops until `windowSpellSettled`: each
pass builds `order` from `s.battlefieldOrder(p, s.ctrlM[p].and(s.occ), ...)`, asks
`windowPick(k, p, order, ...)` for the next source and, on `NoSlot`, returns
`false` (the payment cannot be paid). A hand card is an `Obj` in
`pl.hand[MaxHand]` / `pl.handN` (`dirac/state.go`), not a `Slot`, so it cannot
enter `order`, and does not need to: gorge lists hand options after every
battlefield option and answers the first, which is the walk's order with the hand
as a tail.

**Decision: the hand source is a tail of the walk, taken only where `windowPick`
returns `NoSlot`.** `windowPick`'s signature and body do not change.

- In `windowTapWalk`, the `sl == NoSlot` branch becomes: when
  `k.gen != nil && k.gen.anyHandMana` and the walk is one gorge lists hand
  sources for (below), call `s.windowHandPick(k, p, useful, clamp, anyColor)`; if
  it returns a hand `Obj`, pay it (`s.windowHandTap`) and `continue` the loop;
  else `return false, held` as today.
- `windowHandPick` scans `pl.hand[0..handN)` in hand order (gorge lists
  `Zone(ZHand)` order; `removeFrom` in `dirac/zones.go` keeps the order when a card
  leaves). For each card whose table row has a hand mana ability
  (`k.handManaAb[card] >= 0`) it applies the same clamp as `windowPick`: when
  `clamp && !anyColor`, `want, decided := s.windowUseful(k, p, useful)` once, and
  skip the card when `decided` and `colourMask(ability) & want == 0`. The first
  card that passes is the pick. A guide's mask is one colour bit
  (`ManaColors`, R or G) and `ManaN` is 1.
- `windowHandTap` adds the mana as a fixed-colour source does:
  `s.poolAdd(p, col, 1)` with `col = bits.TrailingZeros8(mask)`, then pays the cost
  with the shared primitive `s.exileFromGrave(o)` (`dirac/cost_exilefromgrave.go`:
  `detach` from the hand, `placeFrom` to exile; no discard, so no discard trigger
  or madness reads it; Q4). It does none of the slot-only steps (`pfTapped`,
  `manaTapped`, `artTag`, restricted-mana rides, sacrifice parks): a Spirit Guide
  has none of them.
- Walks the hand tail joins: the cast window (`payCastWindow`) and the announced
  {X} window (`loweredWindowOK` in `dirac/mana_pain.go`, `xAnnounced`), the two
  windows measured above; the unless/ward walk (`payWindow` in `dirac/mana.go`)
  only after step 0 of the implementation confirms gorge lists a guide there
  (section 6). Walks it never joins:
  `potential` walks (`windowTapNoOwed`, the trigger-cost window, battlefield-only
  in gorge: `paymentManaAskClass`), and the attack-tax window (not this walk).
- `windowDryRestr` (the dry-run census) reaches the same `sl == NoSlot` break:
  when `windowHandPick` would find a card there it returns `ok=false, known=false`
  (the contract for "the caller must probe another way": `tapForHyb` and the
  cast's dry-run caller then run the real walk on a fork, `windowTapRider`),
  instead of `ok=false, known=true`.
- **Not changed:** `windowAffords*` / `windowPays` (the unless/ward *offer* gate)
  and every `avail` census (`availFor`, `availPotentialFor`, `lowerWitness`):
  gorge's planner prices no hand source (`TestSpiritGuideGorgeWindow`:
  `castNoLandOneGuid` 0), so the kernel must not offer a cast, a pay election or
  an X on a guide either. The guide is spent only in a window that is already
  open.

## 3. State budget (Q3)

The change adds **0 bytes** to `State`. A hand source has no state of its own: it
leaves the hand when paid, and the hand array and `handN` already exist. What it
needs lives in `prep`, the read-only per-table side that forks do not copy:

- `prep.handManaAb []int8`: per card, the index of its hand mana ability in
  `cd.Abilities`, -1 none (the shape of `prep.tapXAb`, filled in the same loop in
  `dirac/prep.go`).
- `genPrep.anyHandMana bool` (`dirac/gen.go`, beside `anyHandAct`): any card in the
  table has one. Every walk's hook is behind it, so a table with no Spirit Guide
  (every pool but the ones that contain one) takes one false branch in cold code.

Measured at this base with `make test-one PKG=./dirac RUN='^TestStateSize$$|^TestStateLayoutPinned$$|^TestTransitionAllocatesNothing$$|^TestColdArraysHoldNoPointers$'`:
`State size: 20064 bytes` (the pin in `TestStateLayoutPinned` is `want = 20064`;
the bar in `TestStateSize` is `maxStateBytes = 24 * 1024` = 24576, so 4512 bytes of
headroom this change does not touch), and all four pass. `TestStateIsPointerFree`
does not exist at this base; `TestColdArraysHoldNoPointers` is the pointer check.

Code placement follows the header of `dirac/mana_window.go`: the linker lays out
a package's functions in file order, so the new functions (`windowHandPick`,
`windowHandTap`) go at the **end** of `mana_window.go`, `//go:noinline`, and the two
hooks in `windowTapWalk` and `windowDryRestr` are one `if` each in the existing
cold branch. `windowTapWalk` is not on the transition benchmark's path for a
table without a guide, and `execPay` / `prepareGen` keep their sizes. Check:
`TestTransitionAllocatesNothing` plus the benchmark note below.

## 4. The kop representation of the self-exile (Q4)

What exists: a trigger's own `ExileFromHand<N/Card>` (`dirac/cost_exilefromhand.go`)
rides `PayExileGrave` with `B.N = kop.PayBSubExileHand` (= 9), and `execPay`
(`dirac/interp_ask.go`) widens it to `payKindExileHand` (= 19), posing a pick over
the payer's hand (`handExileOptions`). That path is an `Instr` of an
unless/if-paid cost, for N cards the payer chooses. A self-exile names one card
(the source), asks nothing and is the cost of a mana ability the window walk
pays, not an `Instr`. **It reuses none of that path.** The one thing it shares is
the move primitive the file's comment names: `exileFromGrave`, which detaches from
any zone.

**Decision:**

- Cost: `kop.Cost{Flags: kop.CostExileSelf}`, nothing else (no mana, no tap).
  `CostExileSelf` exists (`dirac/kop/costs.go`, "exile this permanent",
  `1 << 20`) and its kgen lowering (`lCost.ExileSelf`, `build.go`) already sets it.
  No new cost field and no new constant.
- Kind: a **new `kop.AbHandMana`** (`AbilityKind`, appended after `AbAltCost` = 6,
  so 7; `dirac/kop/testdata/ordinals.tsv` takes the row via `KOP_ORDINALS_REGEN=1`,
  and `dirac/kgen/names.go` lists it beside `kop.AbAltCost`). It carries the mana
  facts the way `AbMana` does (`ManaColors`, `ManaN`) and `ActZone = kop.ZoneHand`.
  kgen lowers the card as an `akMana` with `Zone = zHand` and emits `AbHandMana`
  at the one place that maps kgen kinds to kop kinds (the `abKinds` table use in
  `dirac/kgen/build_ab.go`).

Why not `AbMana` + `ActZone` hand: `kop.AbMana` is read in 30 non-test files of
`dirac` at 65 sites (`grep -c 'kop.AbMana'`), and `manaAb` 114 times. The
battlefield is where those readers look, and a Spirit Guide *does* reach the
battlefield (the probes show guides cast as creatures: they are among the untapped
permanents above). A guide on the battlefield with an `AbMana` row would be read by
every planner census, window gap test and mana-pain/grant scan that ranges a
slot's abilities, each needing its own `ActZone` check; a new kind is invisible to
all of them. `prep.go`'s `case kop.AbMana:` would also have to be guarded so
`pc.manaAb` is not set from a hand ability. Only readers that `switch` on kind
with a `default` that treats it as a spell or activated ability need a look: the
one in `dirac/feat/mzxgraph.go` (`mzxAbilityName` has a `default` that names a
cost; `ab.Kind == kop.AbMana` tests there decide which abilities draw as mana) is
the only one found outside kgen.

Priority: `priorityHandAct` (`dirac/cost_handactivate.go`) offers hand abilities
only for `Kind == kop.AbActivated`, so an `AbHandMana` is never an option at
priority, which is what `doc.go` requires; no code change and a test pins it
(`TestSpiritGuideNoPriorityOffer`, below).

## 5. kgen and lockstep (Q5)

### kgen

- `dirac/kgen/lower_handmana.go`: `handManaRefusal` becomes `handManaCost`, which
  *accepts* exactly the shape `kind == akMana`, `ab.Zone == zHand` and a cost of
  exactly `ExileFromHand<1/CARDNAME>` (or NICKNAME), and refuses every other
  mana ability that functions from the hand with its own reason (the same
  `strings.Contains` on the cost, now whole-string).
- `costHand` (`dirac/kgen/lower.go`, the `case "ExileFromHand":` arm) reads
  `ExileFromHand<1/CARDNAME>` into `lCost.ExileSelf` only when the caller passed
  `handManaCost`'s approval (a field on `lowerCtx`, as `lc.grantExile` is), else
  keeps today's refusal "exile cost ... Card" so a non-mana hand ability
  (Masked Bandits) still refuses with its older reason.
- The two gates after the cost is read widen by exactly this shape:
  "exile-self cost outside a battlefield ability" (`cost.ExileSelf && ab.Zone != 0 &&
  ab.Zone != zBattlefield`) and "hand ability without discarding itself" (the
  `putSelfAbility` call in `lower_ninjutsu.go`).
- `dirac/kgen/build_ab.go`: after `ka.Kind` is set from `abKinds`, a lowered
  `akMana` with `Zone == zHand` becomes `kop.AbHandMana` (and `ka.ActZone` is
  already `ZoneHand` from `zoneMap`).
- `TestSpiritGuideRefusal` and `TestSpiritGuideHandManaRefusal` (kgen) pin the
  refusal and are replaced by tests that the two cards lower (below); this ticket
  leaves them standing (kgen unchanged).

### Lockstep rows

There is no `TestReproSpiritGuide` at this base. Two halves, because the forced
state cannot be recorded: a repro (`Repro`, `dirac/oracle/repro.go`) holds picks
and replays them on both sides; `TestSpiritGuideWindowOrder`'s tap is an
out-of-band `events.Tap` on gorge alone, and the oracle has no paired injection
(no side-by-side hook in `Lockstep`/`LockstepLed`).

1. `TestReproSpiritGuide` in a new `dirac/oracle/spiritguide_repro_test.go`, in the
   style of `TestReproBanefire` (`dirac/oracle/banefire_test.go`: an
   `effectLongCase` table, `replayEffectLongCase`, recorded games under
   `dirac/oracle/testdata/repro/`, a `SPIRITGUIDE_RECORD=1` recorder, pools from
   `GORGE_POOLS` in the machine's `test-targets.env`), with the case
   `spirit_guide_listed_never_taken_in_x_window`: Simian guides beside Banefire and
   Mountains (the deck `TestSpiritGuideXWindow` measured), `surface: "full"`,
   `reached` = a Banefire resolved and a guide is still in the caster's hand. This
   pins the *inert* side: the kernel must not tap a hand source ahead of the
   battlefield. (Measured reach: 2 listing windows in 200 games; the recorder needs
   a policy that prefers Banefire and a large X, as `banefirePolicy` does.)
2. The *taken* side (the window-level row where the guide's mana pays the cast) has
   a kernel half and a gorge half. Kernel: `TestWindowHandSource` in a new
   `dirac/mana_handsource_test.go`, built like the other `mana_window_*_test.go`
   files: a table with a Simian guide and a Lightning Bolt, the Bolt cast with the
   battlefield empty and the guide in hand; assert the walk pays, the pool/stack
   hold R spent, the guide is in exile and not in the graveyard, `handN` fell by one
   and no discard trigger fired. Gorge: `TestSpiritGuideWindowOrder` /
   `TestSpiritGuideGorgeWindow` already measure the same state (tapAll). If
   step 0 finds a window shape natural play reaches with the battlefield dry, a
   recorded row `spirit_guide_pays_when_battlefield_is_dry` joins `TestReproSpiritGuide`;
   without one, the two halves are the evidence and this note says so.

## 6. Unmeasured edges

- **Unless-cost and activated-ability windows.** `unlessManaWindowNeeded` opens an
  unless-cost window for a payer whose only source is a hand card, so gorge may
  list the guide there; natural play opened none: a scratch probe (not committed, it had
  nothing to assert) over 200 seeds of Simian guides, Mana Leak, Lightning Bolt,
  Islands and Mountains opened 0 payment windows of any kind. Step 0 forces one (a tap at the pay-unless ask)
  before the hand tail is allowed into the unless/ward walk; the default stays
  battlefield-only there.
- **Gorge's colour clamp when `decided` is false** (a hybrid pip, an unsettled
  generic): measured only for fixed-cost casts. The kernel's clamp is shared, so
  the tail inherits whatever `windowPick` does for battlefield sources.
- **Hand order**: kernel `hand` order equals gorge's `Zone(ZHand)` order as long as
  both keep draw order and close gaps; the Simian-plus-Elvish case in step 4 pins it.

## Implementation ticket brief

**Goal.** kgen lowers Simian and Elvish Spirit Guide, and the kernel's cast and
announced-X payment windows take a hand guide, after every battlefield source, in
hand order, behind the colour clamp, exiling it as the cost; the oracle's priority
vocabulary is unchanged. The seat needs a writable `.cards/kernel-tables` (the
tables are rebuilt by `make dirac-tables` and keyed by the kgen sources), and
`GORGE_POOLS` from `test-targets.env` for the lockstep rows.

**Files.**

- `dirac/oracle/spiritguide_probe_test.go`: step 0 probes only.
- `dirac/kop/abilities.go`, `dirac/kop/testdata/ordinals.tsv`: `AbHandMana`.
- `dirac/kgen/lower_handmana.go`, `lower.go`, `lower_ninjutsu.go`, `build_ab.go`,
  `names.go`: Q5.
- `dirac/prep.go`, `dirac/gen.go`: `handManaAb`, `anyHandMana`.
- `dirac/mana_window.go` (end of file for new functions; one `if` each in
  `windowTapWalk` and `windowDryRestr`). Three other live tickets edit this file:
  keep the hunks small and at the ends.
- New tests: `dirac/mana_handsource_test.go`, `dirac/oracle/spiritguide_repro_test.go`,
  `dirac/kgen/spiritguide_lower_test.go`. Replace the two kgen refusal tests.
- `dirac/docs/support.md` and the kgen acceptance pins: the two cards move from
  refused to supported (re-pin below). Not `dirac/oracle/doc.go`.

**Steps (one commit each).**

0. Probe the two edges in section 6 (forced unless window; an activated-ability
   window) in the probe file; decide the hand tail's walk set from the results and
   record them in the commit. Natural-reach search for a recorded row.
1. kop: `AbHandMana` appended; `KOP_ORDINALS_REGEN=1`; `names.go`.
2. kgen lowering and emit (Q5); the new `TestSpiritGuideLowers` asserts both cards
   lower to one `AbHandMana` with `Cost.Flags == CostExileSelf`, `ActZone ==
   ZoneHand`, `ManaN == 1` and the right colour, and that Masked Bandits keeps its
   refusal.
3. Kernel: prep/gen fields; `windowHandPick`, `windowHandTap`; the two hooks.
4. Tests: `TestWindowHandSource` (kernel; with the hand tail off the same state
   must fail the payment, so the test fails without the hook);
   `TestWindowHandSourceClamp` (a red guide never pays {G}; a green guide pays
   {2}{R}'s generic while the pool holds 0-1 and not at 2); `TestWindowHandSourceOrder`
   (Simian then Elvish in hand, a {R}{G} cast: Simian first; a battlefield source is
   always ahead of both); `TestSpiritGuideNoPriorityOffer` (no `AkActivate` for a
   hand guide in any priority list); `TestReproSpiritGuide`.
5. Re-pin: `make dirac-tables`, then
   `make test-one PKG=./dirac/kgen RUN='^TestCorpusAcceptance$$|^TestSetAcceptance$$|^TestSupportDocIsCurrent$' TESTENV='KGEN_REPIN=1'`
   (a rise only) and `DIRAC_UPDATE_SUPPORT=1` for the support doc rows.

**Commands.**

    make dirac-tables
    make test-one PKG=./dirac RUN='^TestWindowHandSource$$|^TestWindowHandSourceClamp$$|^TestWindowHandSourceOrder$$|^TestSpiritGuideNoPriorityOffer$$|^TestStateSize$$|^TestStateLayoutPinned$$|^TestTransitionAllocatesNothing$$|^TestColdArraysHoldNoPointers$'
    make test-one PKG=./dirac/kgen RUN='^TestSpiritGuideLowers$'
    make test-one PKG=./dirac/oracle RUN='^TestSpiritGuideGorgeWindow$$|^TestSpiritGuideWindowOrder$$|^TestSpiritGuideOffColour$$|^TestSpiritGuideXWindow$$|^TestReproSpiritGuide$'
    make test-one PKG=./internal/archtest RUN='.'
    make test-one PKG=./internal/codeshape RUN='.'
    make precheck BASE=land

**Done means.**

- [ ] The five tests of step 4 exist, run and pass; each of the first three fails
      with the hand tail's hook removed (paste the failure).
- [ ] `TestStateSize`, `TestStateLayoutPinned` (`want` unchanged at 20064),
      `TestTransitionAllocatesNothing` and `TestColdArraysHoldNoPointers` pass: no
      State growth.
- [ ] A benchmark note for the hook in `windowTapWalk`: `BenchmarkTransitionFDN`
      before and after on a table with no guide, inside the noise the file header
      names (2-3 %).
- [ ] The oracle probes of this note still pass (their forced-state claims become
      the equivalence the kernel half asserts).
- [ ] `go run ./cmd/gentypes -check` is clean (no wire type moved).
- [ ] `make precheck BASE=land` ends `precheck: ok <sha>`.

**Out of scope.** Other hand or graveyard mana abilities (a different cost each),
"activate" at priority, gorge, `dirac/oracle/doc.go`.
