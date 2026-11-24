# feat(cardc): CC-16b front end, costs, statics, CDAs, cast effects and replacements

Kind: feature

Target branch: `cardc`. **Complexity: high** (25 enums, about 110 variants). Seat: Sonnet.
Design of record: `docs/design/2026-11-23-typed-script-values/PLAN.md` on `cardc`, section 1, and CC-15a's brief
(`cc-15a-fe-actions-al.md`), which fixes every IR shape used here. Runs after CC-15a (`cc-15a-fe-actions-al`).

## Goal

Register:
- the `Cost` enum and its parts;
- the static, CDA, cast-effect and replacement `Rule` variants;
- the effect enums they hold: `StaticLayerEffect`, `PermanentRule`, `PlayerEffect`, `CastEffect`, `SpellEffect`;
- the replacement event and action enums.

Cover every variant the proof slice uses except the deferred list.

## Behaviour

- `c.Cost(x)` (CC-15a's helper) returns the `*ir.Cost` below. Every cost in an activated ability, keyword, `MayCost`
  or cast effect goes through it.
- A static ability is `{AbStatic, Body: Program{[{VerbApplyEffect, Obj, Effects, Dur: DurWhileActive}]}}`: the same
  step an action creates, with the static's lifetime. There is no separate static shape.
- A replacement or prevention is `{AbReplacement, Trigger: <the replaced event>, Body: <the replacement steps>}`.
  Inside its body, `{RefEventObject}` is the object the event affects.
- No edit to `engine/cardc/ir`, `lower.go` or any CC-15a file.

## Design

Decided. `const n` is `{VConst, N: n}`, `refConst1()` is `fe_ref.go:23`, and `static(obj, effs)` below is the
static step above.

### 1. Costs (`fe_cost.go`, new)

`Cost.<V>` → `*ir.Cost`. A part is `*ir.CostPart`.

| variant (schema args) | `ir.Cost` |
|---|---|
| `And(list)` | the merge, in order: Mana concatenated, `HasMana` ORed, Parts concatenated |
| `PayMana(syms)` | `{Mana, HasMana: true}`; each symbol through `c.lower` gives `*ir.ManaSym` (CC-10) |
| `PayManaX(syms, n)` | as `PayMana`; n must lower to `{VX}`, else the refusal `Cost.PayManaX:value` |
| `TapPermanent(p)` | `{RefSelf}` → part `{CostTapSelf}`; any other ref → `{CostTapUntapped, Obj: ref}` |
| `TapNumberPermanents(n, ps)` | part `{CostTapUntapped, Obj: {RefChoose, Filter: c.Filter(ps), N: c.Value(n)}}` |
| `SacrificePermanent(p)` | part `{CostSacrifice, Obj: c.Ref(p)}` |
| `SacrificeAPermanent(ps)` | part `{CostSacrifice, Obj: {RefChoose, Filter, N: const 1}}` |
| `DiscardACard` | part `{CostDiscard, Obj: {RefChoose, Filter: anyCardIn(ZoneHand), N: const 1}}` |
| `DiscardHand` | part `{CostDiscard, Obj: {RefAll, Filter: anyCardIn(ZoneHand)}}` |
| `PayLife(n)` | part `{CostPayLife, N: c.Value(n)}` |
| `Loyalty(n)` | part `{CostLoyalty, N: const n}` (n is signed: `-8`) |
| `Exile(xs, flags)` | one part per `ExilableCost`; a non-empty flag list is the refusal `Cost.Exile:flag` |
| `PutAPermanentIntoItsOwnersHand(ps)` | part `{CostReturnToHand, Obj: {RefChoose, Filter, N: const 1}}` |
| `RemoveCounters(list)` | one part per `RemoveCountersCost` |
| `RevealTopCardOfLibrary` | part `{CostReveal, Obj: {RefTopOfLibrary, N: const 1}}` |

Satellites:
- `ExilableCost` → `*ir.CostPart`:
  - `Permanent(p)` → `{CostExile, Obj: c.Ref(p)}`;
  - `ACardOfTypeFromPlayersHand(hs, pl)` → `{CostExile, Obj: {RefChoose, Of: c.Ref(pl), Filter: c.Filter(hs),
    N: const 1}}`.
- `RemoveCountersCost` → `*ir.CostPart`:
  - `ACounterOfTypeFromPermanent(ct, p)` → `{CostRemoveCounters, Obj: c.Ref(p), N: const 1, Word: counter}`;
  - `NumberCountersOfTypeFromPermanent(n, ct, p)` → `{…, N: c.Value(n), Word}`;
  - `NumberCountersFromAmongPermanents(n, ps)` → `{CostRemoveCounters, Obj: {RefAll, Filter}, N}`, empty Word:
    any kind.
- `ReplacementActionWouldEnterCost.PayLife(n)` → `{Parts: [{CostPayLife, N}]}`.
- `CostReductionSymbol.CostReduceGeneric(n)` → `*ir.ManaSym{ManaGeneric, N: n}`.
- `CostReductionSymbolX.CostReduceX` → `*ir.ManaSym{ManaX}`.
- The helper `reduction(c, syms) (int32, error)` sums a `CostReduceGeneric` list. Any other symbol is the refusal
  `CostReductionSymbol:shape`.

### 2. Static, CDA and cast rules (`fe_static.go`, new)

| `Rule.` variant | `*ir.Ability` |
|---|---|
| `PermanentLayerEffect(p, effs)` | `{AbStatic, Body: static(c.Ref(p), c.Effects(effs))}` |
| `EachPermanentLayerEffect(ps, effs)` | `{AbStatic, Body: static({RefAll, Filter: c.Filter(ps)}, effs)}` |
| `PermanentRuleEffect(p, rules)` | `{AbStatic, Body: static(c.Ref(p), c.Effects(rules))}` |
| `PlayerEffect(pl, effs)` | `{AbStatic, Body: static(c.Ref(pl), effs)}` |
| `EachPlayerEffect(pls, effs)` | `{AbStatic, Body: static({RefAll, Filter}, effs)}` |
| `CastEffect(ce)` | `{AbStatic, Body: static({RefSelf}, [c.Effect(ce)])}` |
| `ThisSpellEffect(effs)` | `{AbStatic, Zones: ir.MakeZoneSet(ZoneStack), Body: static({RefSelf}, effs)}` |
| `CDA_Power(n)` | `{AbStatic, Zones: allZones, Body: static({RefSelf}, [{EffCDAPower, P: c.Value(n)}])}` |
| `CDA_Toughness(n)` | the same with `{EffCDAToughness, T}` |
| `CDA_Color(sc)` | the same with `{EffCDAColor, Colors}` |

`allZones` is `ir.MakeZoneSet` of every zone from `ZoneBattlefield` to `ZoneCommand`: a characteristic-defining
ability functions everywhere (CR 604.3).

`StaticLayerEffect` → `*ir.Effect`, where `k(n)` is `const n`:

| variant | Effect |
|---|---|
| `AddAbility(rules)` | `{EffAddAbility, Abilities: c.NestedAbilities(rules)}` (CC-15a; each a separate ability in its own target frame) |
| `AddCreatureType(w)` | `{EffAddSubtype, Words: [w]}` |
| `AdjustPT(p, t)` | `{EffAdjustPT, P: k(p), T: k(t)}` |
| `AdjustPTForEach(p, t, n)` | `{EffAdjustPT, P: {VMul, [k(p), v]}, T: {VMul, [k(t), v]}}`, v = `c.Value(n)` |
| `LosesAllAbilities` | `{EffLoseAllAbilities}` |
| `SetCardtype(t)` | `{EffSetCardType, Words: [t]}` |
| `SetColor(sc)` | `{EffSetColor, Colors}` |
| `SetCreatureType(w)` | `{EffSetSubtype, Words: [w]}` |
| `SetName(s)` | `{EffSetName, Words: [s]}` |
| `SetPT(pt)` | `{EffSetPT, P, T}` from CC-15a's PT pair |

`SettableColor` → `ir.ColorSet` by value: `SimpleColorList(list)` is the union through `charsColorNames`
(`fe_chars.go:14`); `Colorless` is 0.

`PermanentRule` → `*ir.Effect`:
- `CantBeBlocked` → `{EffCantBeBlocked}`;
- `CantBeBlockedByDefenders(ps)` → `{EffCantBeBlockedBy, Filter}`;
- `CantBeTheTargetOfSpellsOrAbilities(sa)` → `{EffCantBeTargeted, Filter: <SpellsAndAbilities>}`;
- `CantBlock` → `{EffCantBlock}`;
- `DoesntUntapDuringControllersUntap` → `{EffNoUntap}`;
- `MustAttack` → `{EffMustAttack}`.

`SpellsAndAbilities.ControlledByAPlayer(pls)` → `{SubjObject, FPred, PControlledBy, Ref: {RefAll, Filter:
c.Filter(pls)}}`.

`PlayerEffect` → `*ir.Effect`:

| variant | Effect |
|---|---|
| `CantCastSpells(ss)` | `{EffCantCast, Filter}` |
| `CantLoseTheGame`, `CantWinTheGame` | `{EffCantLose}`, `{EffCantWin}` |
| `DecreaseSpellCost(ss, syms)` | `{EffSpellsCostLess, Filter, N: const reduction(syms)}` |
| `IncreaseSpellCost(ss, cost)` | `{EffSpellsCostMore, Filter, Cost: c.Cost(cost)}` |
| `HasNoMaximumHandSize` | `{EffNoMaxHandSize}` |
| `Hexproof` | `{EffHexproof}` |
| `MayCastGraveyardCard(cg)` | `{EffMayCast, Ref: c.Ref(cg)}` |
| `MayCastCardFromGraveyardByPayingAddedCost(cg, cost)` | `{EffMayCast, Ref, Cost}` (Cost is the added cost) |
| `MayPlayExiledCard(ce)` | `{EffMayPlay, Ref}` |
| `MayCastSpellsAsThoughTheyHadFlash(ss)` | `{EffFlash, Filter}` |
| `MayCastSpellsFromHandWithoutPaying(ss)` | `{EffCastWithoutPaying, Filter}` |
| `MayPlayAnAdditionalLand` | `{EffExtraLand, N: const 1}` |

`CastEffect` → `*ir.Effect`. The `Self` kinds are about the card itself; `Filter` on `EffSelfCostLess` is "if it
targets a permanent matching".

| variant | Effect |
|---|---|
| `AdditionalCastingCost(cost)` | `{EffAdditionalCost, Cost}` |
| `AlternateCastingCost(cost)` | `{EffAlternativeCost, Cost}` |
| `AlternateCastingCostIf(cost, cond)` | `{EffAlternativeCost, Cost, Cond: c.Cond(cond)}` |
| `CantBeCastIf(cond)` | `{EffCantCast, Cond}` |
| `ReduceCastingCostForEach(syms, n)` | `{EffSelfCostLess, N: {VMul, [const reduction(syms), c.Value(n)]}}` |
| `ReduceCastingCostIf(syms, cond)` | `{EffSelfCostLess, N: const reduction(syms), Cond}` |
| `ReduceCastingCostIfItTargetsAPermanent(syms, ps)` | `{EffSelfCostLess, N: const reduction(syms), Filter}` |
| `ReduceCastingCostX(symsX, n)` | `{EffSelfCostLess, N: c.Value(n)}`; symsX must be exactly `[CostReduceX]`, else the refusal `CastEffect.ReduceCastingCostX:shape` |

`SpellEffect.CantBeCountered` → `{EffCantBeCountered}`.

### 3. Replacements and prevention (`fe_replace.go`, new)

| `Rule.` variant | `*ir.Ability` |
|---|---|
| `AsPermanentEnters(p, acts)` | `{AbReplacement, Trigger: {EvWouldEnter, Ref: c.Ref(p)}, Body: c.StepList(acts)}` |
| `ReplaceWouldEnter(ev, acts)` | `{AbReplacement, Trigger: c.Trigger(ev), Body: c.StepList(acts)}` |
| `ReplaceWouldPutCounters(ev, acts)` | as above |
| `ReplaceWouldDealDamage(ev, acts)` | as above |
| `ReplaceAnEffectWouldCreateAnyNumberOfTokens(ev, acts)` | as above |
| `PreventDamage(ev, acts)` | as above |

Events, each → `*ir.Trigger`:
- `ReplacableEventWouldEnter.APermanentWouldEnterTheBattlefield(ps)` → `{EvWouldEnter, Subject}`.
- `ReplacableEventWouldPutCounters.AnEffectWouldPutCountersOnAPermanent(ps)` → `{EvWouldPutCounters, Object}`.
- `ReplacableEventWouldDealDamage.DamageWouldBeDealtByASourceToARecipient(ds, drl)` → `{EvWouldDealDamage,
  Subject: c.Filter(ds), Object: c.Filter(drl)}`. `DamageSources` is CC-16a's; stub it until CC-16a lands.
- `ReplacableEventAnEffectWouldCreateAnyNumberOfTokens.AnEffectWouldCreateAnyNumberOfTokensUnderAPlayersControl(ps,
  pls)` → `{EvWouldCreateTokens, Subject: c.Filter(ps), Object: c.Filter(pls)}`.
- `ReplacableEventWouldPutIntoGraveyard.APermanentWouldDie(ps)` → `{EvWouldDie, Subject}`.
- `EventPreventDamage.CombatDamageWouldBeDealtToRecipient(sdr)` → `{EvWouldDealDamage, Ref: c.Ref(sdr), Word:
  "combat"}`.
- `EventPreventDamage.NoncombatDamageWouldBeDealtToARecipient(drl)` → `{EvWouldDealDamage, Object: c.Filter(drl),
  Word: "noncombat"}`.

`DamageRecipientsList` → `*ir.Filter`: `APermanent(ps)` → `c.Filter(ps)`; `APlayerOrAPermanent(pls, ps)` →
`{SubjObject, FAny, Args: [c.Filter(pls), c.Filter(ps)]}`.

Replacement actions, each → `*ir.Step`:

| variant | Step |
|---|---|
| `ReplacementActionWouldEnter.EntersTapped` | `{VerbModifyEnter, Enter: {Tapped: true}}` |
| `ReplacementActionWouldEnter.EntersWithACounter(ct)` | `{VerbModifyEnter, Enter: {Counter, N: const 1}}` |
| `ReplacementActionWouldEnter.EntersWithNumberCounters(n, ct)` | `{VerbModifyEnter, Enter: {Counter, N: c.Value(n)}}` |
| `ReplacementActionWouldEnter.ChooseAColor(cc)` | `{VerbChooseColor, Colors}` |
| `ReplacementActionWouldEnter.ChooseACreatureType` | `{VerbChooseCreatureType}` |
| `ReplacementActionWouldEnter.ChooseANumberGreaterThanNumber(n)` | `{VerbChooseNumber, N: const n}` (N is the exclusive minimum) |
| `ReplacementActionWouldEnter.If(cond, acts)` | `{VerbIf, Cond, Then: c.StepList(acts)}` |
| `ReplacementActionWouldEnter.Unless(cond, acts)` | `{VerbIf, Cond: {CNot, [cond]}, Then}` |
| `ReplacementActionWouldEnter.IfPassesFilter(ps, acts)` | `{VerbIf, Cond: {CRefMatches, Ref: {RefEventObject}, Filter: c.Filter(ps)}, Then}` |
| `ReplacementActionWouldEnter.MayCost(rc)` | `{VerbPayCost, Optional, Cost: c.Cost(rc)}` |
| `ReplacementActionWouldPutCounters.PutNewAmount(n)` | `{VerbSetAmount, N}` |
| `ReplacementActionWouldDealDamage.DealDamageInstead(n)` | `{VerbSetAmount, N}` |
| `ReplacementActionAnEffectWouldCreateAnyNumberOfTokens.CreateTokensInstead(toks)` | `{VerbDo, Then: c.StepList(toks)}` (each `CreatableToken` is CC-15a's) |
| `ReplacementActionWouldPutIntoGraveyard.ExileItInstead` | `{VerbMove, Obj: {RefEventObject}, To: ZoneExile}` |
| `ActionPreventDamage.PreventThatDamage` | `{VerbPrevent}` |

Hallowed Fountain is the slice's shape for the last three `ReplacementActionWouldEnter` rows together:
`MayCost(PayLife 2)` then `Unless(CostWasPaid, [EntersTapped])`.

### 4. Deferred (the coverage tests list each with its reason)

- `Rule.ActivatedAbilityEffect` and its enums `ActivatedAbilities`, `ActivatedAbilityEffect`, `AbilitySources`
  (Pithing Needle's shape needs `NameFilter.TheChosenName`, deferred in CC-12);
- `Rule.AsPutIntoAGraveyardFromAnywhere` (with `PutIntoGraveyardAction`);
- `Rule.DeckConstruction`;
- `Cost.ItsManaCost`, `Cost.Or`, `Cost.PutTopCardOfLibraryOfTypeOnBattlefield`;
- `PlayerEffect.MayCastASpellOfEachPermanentFromAmongCardsInGraveyardsOnceEachTurn`,
  `PlayerEffect.MayPlayALandFromAmongCardsInGraveyardsOnceEachTurn`,
  `PlayerEffect.MayPlayExiledCardsAndMaySpendManaAsThoughAnyTypeToCast`;
- `SettableColor.Devoid`;
- `ReplacementActionWouldEnter.ChooseACardName` (as `Action.ChooseACardName`, CC-15a).

## Implementation steps

1. Add `fe_cost.go`, `fe_static.go` and `fe_replace.go`, registering in `init()`.
2. Write the tests.

## Tests

In `engine/cardc/mtgish`:
- `TestCostLowering`: one synthetic JSON row per section 1 row and satellite, compared with `ir.Equal`, including
  `And` of mana plus tap plus sacrifice (Fiend Artisan's shape) and a negative `Loyalty`.
- `TestStaticLowering`: one row per section 2 table row.
- `TestReplacementLowering`: one row per section 3 row, and Hallowed Fountain's whole `AsPermanentEnters` (the
  `VerbPayCost` step precedes the `VerbIf` whose Cond is `{CNot, [{CCostPaid}]}`).
- `TestGrantedStaticAbilityFrame`: `Rule.PermanentLayerEffect(ThisPermanent, [StaticLayerEffect.AddAbility([
  Rule.Activated(<stub cost>, Actions.Targeted([Target.TargetPlayer(<stub>)], …Ref_TargetPlayer…))])])`. The
  granted ability's Program holds its target at slot 0, and `c.TargetSlot("Ref_TargetPlayer")` is false after the
  lowering returns: the frame was restored to the empty top-level list.
- `TestStaticRuleKeysRegistered`: asserts `Registered` for every `Rule.<V>` key in sections 2 and 3.
- `TestStaticSliceVariantsRegistered`: `checkFamilyCoverage` over `Cost`, `ExilableCost`, `RemoveCountersCost`,
  `ReplacementActionWouldEnterCost`, `CostReductionSymbol`, `CostReductionSymbolX`, `StaticLayerEffect`,
  `SettableColor`, `PermanentRule`, `SpellsAndAbilities`, `PlayerEffect`, `CastEffect`, `SpellEffect`,
  `ReplacableEventWouldEnter`, `ReplacableEventWouldPutCounters`, `ReplacableEventWouldDealDamage`,
  `ReplacableEventAnEffectWouldCreateAnyNumberOfTokens`, `ReplacableEventWouldPutIntoGraveyard`,
  `EventPreventDamage`, `DamageRecipientsList`, `ReplacementActionWouldEnter`, `ReplacementActionWouldPutCounters`,
  `ReplacementActionWouldDealDamage`, `ReplacementActionAnEffectWouldCreateAnyNumberOfTokens`,
  `ReplacementActionWouldPutIntoGraveyard` and `ActionPreventDamage`, with the deferrals above.

## Done means

- [ ] `make test-one PKG=./engine/cardc/mtgish` passes (not ENV): every test in the package.
- [ ] `make test-one PKG=./internal/archtest RUN='^(TestDependencyOrderHolds|TestNothingImportsCardc|TestPublicBoundary)$'` passes.
- [ ] No `make test-targeted` or `make test-smart` per ticket: one `make test-smart BASE=origin/cardc` runs at the
  wave-2 head.

## Out of scope

- Ability, trigger and keyword `Rule`s (CC-16a); actions (CC-15a/b).
- Any edit under `engine/cardc/ir`, or to `lower.go` or CC-15a's files.
- Anything on GitHub outside gorge. Never open a pull request to i5jb/mtgish.

## Collisions

Depends on CC-15a. Its only files are `fe_cost.go`, `fe_static.go`, `fe_replace.go` and their tests. It runs in
parallel with CC-15b and CC-16a. No `Rule` key here is CC-10's or CC-16a's. No new package.
