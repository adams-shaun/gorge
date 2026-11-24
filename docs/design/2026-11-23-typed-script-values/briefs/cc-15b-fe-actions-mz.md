# feat(cardc): CC-15b front end, actions M-Z

Kind: feature

Target branch: `cardc`. Complexity: medium. Seat: Sonnet.
Design of record: `docs/design/2026-11-23-typed-script-values/PLAN.md` on `cardc`, section 1, and CC-15a's brief
(`cc-15a-fe-actions-al.md`), which fixes every IR shape used here. Runs after CC-15a (`cc-15a-fe-actions-al`).

## Goal

Register every `Action` variant whose name starts with M to Z that the proof slice uses, except the deferred list,
with the satellite enums `EnterFlag`, `PutCountersAction`, `RemoveCountersAction`,
`RevealTheTopNumberCardsOfLibraryAction`, `SearchLibraryAction` and `DamageToRecipients`.

## Behaviour

- Each lowering returns the `*ir.Step` below, or a `*ir.Program` to splice ("splice" rows), through CC-15a's
  `c.Steps`, `c.StepList`, `c.Enters`, `c.Cost`, `c.NestedBody` and `withZone`/`anyCardIn` helpers
  (`fe_program.go`).
- No edit to `engine/cardc/ir`, `lower.go` or any CC-15a file. A shape this brief cannot express with CC-15a's IR is
  a stop-and-report, not a new field.
- Argument order comes from each variant's line in `oracle_cards.d.ts` (`mtgish.SchemaPath()`).

## Design

Decided. "Acting player", `RefChoose` with nil `Of`, `const 1` (`refConst1()`, `fe_ref.go:23`) and the
library-position words are as CC-15a defines them. A `RefChoose` with nil `N` means "any number".

### Actions M-Z (`fe_action_mz.go`, new)

| `Action.` variant (schema args) | `*ir.Step` |
|---|---|
| `MayAction(a)` | the steps of `c.Steps(a)`: one step gets `Optional: true`; several become `{VerbDo, Optional, Then: Program{steps}}` |
| `MayActions(list)` | `{VerbDo, Optional, Then: c.StepList(list)}` |
| `MayCost(cost)` | `{VerbPayCost, Optional, Cost: c.Cost(cost)}` |
| `MustCost(cost)` | `{VerbPayCost, Cost}` |
| `MillNumberCards(n)` | `{VerbMill, N: c.Value(n)}` |
| `PermanentDealsDamage(p, n, dr)` | `{VerbDealDamage, Source: c.Ref(p), N, Obj: c.Ref(dr)}` |
| `PermanentDealsMultipleDamage(p, list)` | splice: one `{VerbDealDamage, Source: c.Ref(p), N, Obj}` per `DamageToRecipients` |
| `PlayerAction(pl, a)` | the steps of `c.Steps(a)`, each top-level step with nil `Who` getting `Who: c.Ref(pl)` (splice when several) |
| `PlayerActions(pl, list)` | `{VerbDo, Who: c.Ref(pl), Then: c.StepList(list)}` |
| `PlayerMayAction(pl, a)` | as `MayAction`, plus `Who: c.Ref(pl)` on the resulting step |
| `PlayerMayCost(pl, cost)` | `{VerbPayCost, Optional, Who: c.Ref(pl), Cost}` |
| `PutACardFromGraveyardIntoHand(gs)` | `{VerbMove, Obj: {RefChoose, Filter: c.Filter(gs), N: const 1}, To: ZoneHand}` |
| `PutACardFromHandOnBattlefield(hs, flags)` | `{VerbMove, Obj: {RefChoose, Filter, N: const 1}, To: ZoneBattlefield, Enter: c.Enters(flags)}` |
| `PutCounters(list)` | splice: one step per `PutCountersAction` |
| `PutEachCardFromEachPlayersGraveyardOntoTheBattlefield(cs, pls, flags)` | `{VerbMove, Obj: {RefAll, Filter: {FAll, [withZone(c.Filter(cs), ZoneGraveyard), {SubjCard, FPred, POwnedBy, Ref: {RefAll, Filter: c.Filter(pls)}}]}}, To: ZoneBattlefield, Enter}` |
| `PutEachGraveyardCardIntoHand(gs)` | `{VerbMove, Obj: {RefAll, Filter}, To: ZoneHand}` |
| `PutEachPermanentIntoItsOwnersHand(ps)` | `{VerbMove, Obj: {RefAll, Filter}, To: ZoneHand}` |
| `PutEachPermanentOnBottomOfOwnersLibraryInOrderOfOwnersChoice(ps)` | `{VerbMove, Obj: {RefAll, Filter}, To: ZoneLibrary, Word: "bottom-any-order"}` |
| `PutExiledCardOntoBattlefield(ce, flags)` | `{VerbMove, Obj: c.Ref(ce), To: ZoneBattlefield, Enter}` |
| `PutGraveyardCardIntoHand(cg)` | `{VerbMove, Obj, To: ZoneHand}` |
| `PutGraveyardCardOntoBattlefield(cg, flags)` | `{VerbMove, Obj, To: ZoneBattlefield, Enter}` |
| `PutNumCardsFromHandOnTopOfLibraryAnyOrder(n)` | `{VerbMove, Obj: {RefChoose, Filter: anyCardIn(ZoneHand), N: c.Value(n)}, To: ZoneLibrary, Word: "top-any-order"}` |
| `PutPermanentIntoItsOwnersHand(p)` | `{VerbMove, Obj, To: ZoneHand}` |
| `PutPermanentOnBottomOfOwnersLibrary(p)` | `{VerbMove, Obj, To: ZoneLibrary, Word: "bottom"}` |
| `PutTheTopCardOfPlayersLibraryOnTheBottomOfTheirLibrary(pl)` | `{VerbMove, Obj: {RefTopOfLibrary, Of: c.Ref(pl), N: const 1}, To: ZoneLibrary, Word: "bottom"}` |
| `PutTopOfLibraryInHand` | `{VerbMove, Obj: {RefTopOfLibrary, N: const 1}, To: ZoneHand}` |
| `Reflexive_CreateTokens_WhenYouDo(toks, flags, actions)` | splice: the `CreateTokens` steps (CC-15a's token rows and `Enter`), then `{VerbDelayedTrigger, Abilities: [{AbTriggered, Trigger: {Event: EvReflexive}, Body: c.NestedBody(actions)}]}` |
| `RegeneratePermanent(p)` | `{VerbRegenerate, Obj}` |
| `RemoveCounters(list)` | splice: one step per `RemoveCountersAction` |
| `ReturnDeadGraveyardCardToBattlefield(flags)` | `{VerbMove, Obj: {RefEventObject}, From: ZoneGraveyard, To: ZoneBattlefield, Enter}` |
| `ReturnEachCardFromPlayersGraveyardToBattlefield(gs, pl, flags)` | `{VerbMove, Obj: {RefAll, Filter: {FAll, [c.Filter(gs), {SubjCard, FPred, POwnedBy, Ref: c.Ref(pl)}]}}, To: ZoneBattlefield, Enter}` |
| `ReturnGraveyardCardToHand(cg)` | `{VerbMove, Obj, To: ZoneHand}` |
| `ReturnPermanentToTopOrBottomOfLibrary(p)` | `{VerbMove, Obj, To: ZoneLibrary, Word: "top-or-bottom"}` |
| `RevealACardFromHand` | `{VerbReveal, Obj: {RefChoose, Filter: anyCardIn(ZoneHand), N: const 1}}` |
| `RevealCardsFromTheTopOfLibraryUntilACardOfTypeIsRevealed(cs, subs)` | `{VerbRevealUntil, Filter: c.Filter(cs), Then: c.StepList(subs)}` |
| `RevealHand` | `{VerbRevealHand}` |
| `RevealHandAndPlayerChoosesACardToDiscard(pl, cs)` | `{VerbRevealHand, Then: [{VerbDiscard, Obj: {RefChoose, Of: c.Ref(pl), Filter: withZone(c.Filter(cs), ZoneHand), N: const 1}}]}` |
| `RevealHandAndPlayerChoosesACardToExile(pl, cs)` | the same with `VerbExile` |
| `RevealTheTopNumberCardsOfLibrary(n, subs)` | `{VerbReveal, Obj: {RefTopOfLibrary, N: c.Value(n)}, Then: c.StepList(subs)}` |
| `RevealTopCardOfLibrary` | `{VerbReveal, Obj: {RefTopOfLibrary, N: const 1}}` |
| `SacrificeAPermanent(ps)` | `{VerbSacrifice, Obj: {RefChoose, Filter, N: const 1}}` |
| `SacrificeEachPermanent(ps)` | `{VerbSacrifice, Obj: {RefAll, Filter}}` |
| `SacrificeNumberPermanents(n, ps)` | `{VerbSacrifice, Obj: {RefChoose, Filter, N: c.Value(n)}}` |
| `SacrificePermanent(p)` | `{VerbSacrifice, Obj}` |
| `SearchLibrary(list)` | `{VerbSearch, Then: c.StepList(list)}` |
| `Shuffle` | `{VerbShuffle}` |
| `ShuffleHandIntoLibrary` | `{VerbMove, Obj: {RefAll, Filter: anyCardIn(ZoneHand)}, To: ZoneLibrary, Word: "shuffle"}` |
| `SpellDealsDamage(s, n, dr)` | `{VerbDealDamage, Source: c.Ref(s), N, Obj: c.Ref(dr)}` |
| `Surveil(n)` | `{VerbSurveil, N}` |
| `TakeAnExtraTurn` | `{VerbExtraTurn}` |
| `TapPermanent(p)` | `{VerbTap, Obj}` |
| `TransformPermanent(p)` | `{VerbTransform, Obj}` |
| `Unless(cond, list)` | `{VerbIf, Cond: {CNot, Args: [c.Cond(cond)]}, Then: c.StepList(list)}` |
| `UntapPermanent(p)` | `{VerbUntap, Obj}` |

Meanings:
- In the `RevealHandAndPlayerChooses…` rows, the hand is the acting player's (the revealer's, usually set by an
  enclosing `PlayerAction(Ref_TargetPlayer, …)`), `Of` is the chooser, and the acting player discards or exiles.
  Pilfer is the slice's example.
- `CCostPaid` (CC-14) refers to the nearest earlier `VerbPayCost` step in the same Program. No binding field is
  needed: the order of steps is the binding. Daze, Chain Lightning and Hallowed Fountain are the examples.
- `MayAction` over a `VerbIf` keeps `Optional` on the `VerbIf` step: "you may" governs the whole conditional.
- `Who` is set on a block step (`VerbDo`) as well as on simple steps. CC-15a's rule makes it apply to every step
  under it that has no `Who` of its own.

### Satellites (`fe_action_mz.go`)

**`EnterFlag`** → `*ir.Enter`; CC-15a's `c.Enters` merges the list:
- `EntersNormally` → `&ir.Enter{}`, dropped by the merge.
- `EntersTapped` → `{Tapped: true}`.
- `EntersUnderOwnersControl` → `{UnderOwner: true}`.
- `EntersUnderPlayersControl(pl)` → `{Controller: c.Ref(pl)}`.
- `EntersWithACounter(ct)` → `{Counter: c.lower(ct) string, N: const 1}`.
- `EntersWithNumberCounters(n, ct)` → `{Counter, N: c.Value(n)}`.
- `EntersWithLayerEffect(effs)` → `{Effects: c.Effects(effs)}`.

**`PutCountersAction`** → `*ir.Step`:
- `ACounterOfTypeOnPermanent(ct, p)` → `{VerbPutCounters, Obj: c.Ref(p), N: const 1, Word: counter}`.
- `ACounterOfTypeOnEachPermanent(ct, ps)` → `{VerbPutCounters, Obj: {RefAll, Filter}, N: const 1, Word}`.
- `NumberCountersOfTypeOnPermanent(n, ct, p)` → `{VerbPutCounters, Obj, N: c.Value(n), Word}`.
- `DoubleCountersOfTypeOnPermanent(ct, p)` → `{VerbDoubleCounters, Obj, Word}`.
- `DoubleAllCountersOnEachPermanent(ps)` → `{VerbDoubleCounters, Obj: {RefAll, Filter}}` (empty Word: every kind).

**`RemoveCountersAction.AllCountersOfTypeFromEachPermanent(ct, ps)`** → `{VerbRemoveCounters, Obj: {RefAll,
Filter}, Word}`; nil `N` means all of them.

**`DamageToRecipients.DamageToRecipients(n, dr)`** → an unexported `*damagePart{N *ir.Value; To *ir.Ref}` that
`PermanentDealsMultipleDamage` reads.

**`SearchLibraryAction`** → `*ir.Step`. A `VerbChoose` step inside a `VerbSearch`'s `Then` produces the receipt
`RcFound` (elsewhere `VerbChoose` produces `RcChosen`).
- `FindACardOfType(cl)` → `{VerbChoose, Obj: {RefChoose, Filter: c.Filter(cl), N: const 1}}`.
- `FindAGenericCard` → `{VerbChoose, Obj: {RefChoose, Filter: anyCardIn(ZoneLibrary), N: const 1}}`.
- `FindNumberCardsOfType(n, cl)` → `{VerbChoose, Obj: {RefChoose, Filter, N: c.Value(n)}}` (up to N).
- `PutFoundCardsIntoHand` → `{VerbMove, Obj: {RefReceipt, Receipt: RcFound}, To: ZoneHand}`.
- `PutFoundCardsOntoBattlefield(flags)` → `{VerbMove, Obj: {RefReceipt, RcFound}, To: ZoneBattlefield, Enter}`.
- `RevealFoundCards` → `{VerbReveal, Obj: {RefReceipt, RcFound}}`.
- `Shuffle` → `{VerbShuffle}`.
- `SetAsideFoundCards` → `{VerbDo}`, an empty block: the found cards stay found.
- `PutSetAsideCardsOnTopOfLibrary` → `{VerbMove, Obj: {RefReceipt, RcFound}, To: ZoneLibrary, Word: "top"}`.

**`RevealTheTopNumberCardsOfLibraryAction`** → `*ir.Step`. Inside a `VerbReveal` or `VerbRevealUntil` step's
`Then`, a `RefChoose` chooses among the revealed cards, and a nil `Obj` means the remaining revealed cards.
- `MayPutAnyNumberOfCardsOfTypeOntoTheBattlefield(cs, flags)` → `{VerbMove, Optional, Obj: {RefChoose, Filter:
  c.Filter(cs)}, To: ZoneBattlefield, Enter}` (nil N: any number).
- `PutTheCardFoundThisWayIntoHand` → `{VerbMove, Obj: {RefReceipt, RcFound}, To: ZoneHand}`.
- `PutTheRemainingCardsIntoGraveyard` → `{VerbMove, To: ZoneGraveyard}`.
- `PutTheRemainingCardsOnTheBottomOfLibraryInARandomOrder` → `{VerbMove, To: ZoneLibrary, Word: "bottom-random"}`.

### Deferred (the coverage test lists each with its reason)

- `Action.PermanentDealsDistributedDamage` (needs `Actions.TargetedDistributed`, deferred in CC-15a);
- `Action.RepeatThisProcess`, `Action.RepeatableActions` (a loop construct);
- `Action.RestartTheGameLeavingEachExiledCardInExileThenPutThoseCardsOntoTheBattlefield` (restart);
- `Action.RevealCardFromOutsideGameAndPutInHand`,
  `Action.RevealCardFromOutsideGameAndPutInHandOrPutAnExiledCardInOwnersHand` (outside the game);
- `Action.VoteForAPermanent` (voting).

## Implementation steps

1. Add `fe_action_mz.go`, registering every row and satellite in `init()`.
2. Write the tests.

## Tests

In `engine/cardc/mtgish`:
- `TestActionLoweringMZ`: one synthetic JSON row per table row and per satellite row, compared with `ir.Equal`.
  Nested refs, filters, values, costs and conditions are stubbed with `withStubs`.
- `TestPlayerActionSetsWho`: `PlayerAction(Ref_TargetPlayer, MayAction(DiscardACard))` gives one step with `Who`
  slot 0 and `Optional`; an inner step that already has `Who` keeps it.
- `TestReflexiveTargetFrame`: `Reflexive_CreateTokens_WhenYouDo` whose `Actions` is
  `Targeted([TargetPermanent], …Ref_TargetPermanent…)`, lowered inside an outer `Targeted([TargetPlayer], …)`. The
  reflexive body's Program has its own target at slot 0, and after the step the outer `Ref_TargetPlayer` still
  resolves to slot 0. This is the slice's Faebloom Trick shape.
- `TestSearchReceipts`: `SearchLibrary([FindACardOfType, PutFoundCardsIntoHand, Shuffle])` gives a `VerbSearch`
  whose second step reads `RcFound`.
- `TestActionMZSliceVariantsRegistered`: CC-15a's `actionCoverage(t, 'M', 'Z', deferred)` with the Action
  deferrals above.
- `TestActionMZSatellitesRegistered`: `checkFamilyCoverage` over `EnterFlag`, `PutCountersAction`,
  `RemoveCountersAction`, `RevealTheTopNumberCardsOfLibraryAction`, `SearchLibraryAction` and `DamageToRecipients`,
  deferred `{}`.

## Done means

- [ ] `make test-one PKG=./engine/cardc/mtgish` passes (not ENV): every test in the package.
- [ ] `make test-one PKG=./internal/archtest RUN='^(TestDependencyOrderHolds|TestNothingImportsCardc|TestPublicBoundary)$'` passes.
- [ ] No `make test-targeted` or `make test-smart` per ticket: one `make test-smart BASE=origin/cardc` runs at the
  wave-2 head.

## Out of scope

- Any edit under `engine/cardc/ir`, or to `lower.go` or CC-15a's files.
- `Rule` wrappers, triggers and costs (CC-16a/b). `MayCost`'s cost is lowered through `c.Cost`, which CC-16b fills.
- Anything on GitHub outside gorge. Never open a pull request to i5jb/mtgish.

## Collisions

Depends on CC-15a. Its only files are `fe_action_mz.go` and `fe_action_mz_test.go`. It runs in parallel with
CC-16a and CC-16b, which register other enums. No new package.
