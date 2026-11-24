# feat(cardc): CC-15a wave-2 IR additions, program plumbing, target frames and actions A-L

Kind: feature

Target branch: `cardc`. **Complexity: high. Review: Opus** (it fixes every IR shape wave 2 adds; CC-15b, CC-16a and
CC-16b only register lowerings onto it). Implementer: Sonnet.
Design of record: `docs/design/2026-11-23-typed-script-values/PLAN.md` on `cardc`, sections 1 and 6. This brief
fixes the exact IR additions and supersedes the plan's "a family appends only in its own ir file" rule for wave 2.
Runs after CC-10..14 (all merged on `cardc` at `69951fbdb5`).

## Goal

1. Add every IR type, field and enum value that wave 2 needs (actions, rules, triggers, statics, replacements, costs),
   so CC-15b, CC-16a and CC-16b edit nothing under `engine/cardc/ir` and run in parallel.
2. Add the program plumbing in `engine/cardc/mtgish`: typed helpers, step splicing, and **target frames** (the save
   and restore of target declarations around `Actions.Targeted` and around every nested ability).
3. Register the `Actions` containers and every `Action` variant whose name starts with A to L, with the satellite
   enums listed below.

## Behaviour

- After this ticket, a proof-slice rule whose actions are all A-L variants lowers fully once CC-16a registers its
  `Rule` wrapper. Until then the rule is refused by its `Rule.*` key, as today.
- `RefTarget.Slot` indexes the `Targets` of the nearest enclosing `ir.Program` that has targets. `Ability.Targets`
  is removed. Each `Actions.Targeted` lowers to a Program carrying its own `Targets`, slots from 0.
- A nested ability (granted by an effect, an emblem's, a token's, a delayed or reflexive trigger's) is lowered in a
  fresh target frame. Its slots start at 0, and after it the outer ability's `Ref_*` variants resolve exactly as
  before it.
- Measured on the proof slice at `1553384` (FDN pool plus the 12 Legacy decks): no `Actions.Targeted` sits inside
  another without a `Rule` between them, and no `Ref_*` inside a nested rule points at an outer target. So a
  reference that would need an outer frame stays the `:unbound` refusal CC-11 already returns.

## Design

Decided. The seat makes no design calls.

### 1. IR additions (`engine/cardc/ir`)

Every new enum follows CC-01's shape: `uintN`, zero value `XxxNone`, unexported `xxxCount` sentinel last, a name
table of lower-kebab **letters-only** names (`TestEnumNamesComplete`'s regexp is `^[a-z]+(-[a-z]+)*$`), `String`,
`ParseXxx`, and JSON by name. Every new field gets a lower-camel `json:"…,omitempty"` tag, sits before `Src`, and is
added to `equal.go`, `print.go` (declaration order, printed only when non-zero) and the fixtures in `ir_test.go`.

**`program.go`**
- Append to `Verb`, in this order, with these names:

  | const | name | const | name |
  |---|---|---|---|
  | `VerbAddMana` | `add-mana` | `VerbDo` | `do` |
  | `VerbAttach` | `attach` | `VerbPayCost` | `pay-cost` |
  | `VerbChoose` | `choose` | `VerbMill` | `mill` |
  | `VerbChooseColor` | `choose-color` | `VerbDealDamage` | `deal-damage` |
  | `VerbChooseModes` | `choose-modes` | `VerbMove` | `move` |
  | `VerbCopySpell` | `copy-spell` | `VerbPutCounters` | `put-counters` |
  | `VerbCounter` | `counter` | `VerbDoubleCounters` | `double-counters` |
  | `VerbApplyEffect` | `apply-effect` | `VerbRegenerate` | `regenerate` |
  | `VerbDelayedTrigger` | `delayed-trigger` | `VerbRemoveCounters` | `remove-counters` |
  | `VerbCreateReplacement` | `create-replacement` | `VerbReveal` | `reveal` |
  | `VerbCreateToken` | `create-token` | `VerbRevealHand` | `reveal-hand` |
  | `VerbCreateTokenCopy` | `create-token-copy` | `VerbRevealUntil` | `reveal-until` |
  | `VerbDestroy` | `destroy` | `VerbSacrifice` | `sacrifice` |
  | `VerbDiscard` | `discard` | `VerbSearch` | `search` |
  | `VerbDraw` | `draw` | `VerbShuffle` | `shuffle` |
  | `VerbForEachPlayer` | `for-each-player` | `VerbSurveil` | `surveil` |
  | `VerbEndTurn` | `end-turn` | `VerbExtraTurn` | `extra-turn` |
  | `VerbExile` | `exile` | `VerbTap` | `tap` |
  | `VerbFight` | `fight` | `VerbUntap` | `untap` |
  | `VerbGainControl` | `gain-control` | `VerbTransform` | `transform` |
  | `VerbGainLife` | `gain-life` | `VerbModifyEnter` | `modify-enter` |
  | `VerbEmblem` | `emblem` | `VerbChooseCreatureType` | `choose-creature-type` |
  | `VerbIf` | `if` | `VerbChooseNumber` | `choose-number` |
  | `VerbLookAtHand` | `look-at-hand` | `VerbSetAmount` | `set-amount` |
  | `VerbLook` | `look` | `VerbPrevent` | `prevent` |
  | `VerbLoseLife` | `lose-life` | | |

  Left column first, top to bottom, then the right column. Each verb's operand fields are given where it is lowered
  (section 4 here, CC-15b, CC-16b).
- Append `DurUntilEndOfYourNextTurn` (`until-end-of-your-next-turn`) to `Duration`.
- `Program` gains `Targets []*Target` (before `Src`).
- `Step` gains, in this order, before `Src`:
  - `Source *Ref`: the acting object (damage source, the first fighter).
  - `Other *Ref`: a second object operand (what `Obj` is attached to).
  - `Filter *Filter`: a selection filter (`VerbRevealUntil`'s "until a card matching").
  - `Cost *Cost`: `VerbPayCost`'s cost.
  - `Mana []ManaSym`: `VerbAddMana`'s mana. A `ManaColored` symbol with more than one colour is **one** mana of any
    one of those colours.
  - `Modes []*Program`: `VerbChooseModes`' modes; `N` is how many are chosen.
  - `Effects []*Effect`: `VerbApplyEffect`'s and `VerbCreateTokenCopy`'s effects.
  - `Abilities []*Ability`: abilities the step creates (`VerbEmblem`, `VerbDelayedTrigger`,
    `VerbCreateReplacement`).
  - `Token *Face`: `VerbCreateToken`'s token.
  - `Enter *Enter`: how a moved or created object enters the battlefield.
- Doc comment on `Step.Who`: the acting player for this step and every step nested under it that has no `Who` of its
  own; nil means the ability's controller.

**`effect.go`** (new)
- `type EffectKind uint16`, names in parentheses:
  `EffNone` (`none`), `EffAdjustPT` (`adjust-pt`), `EffSetPT` (`set-pt`), `EffAddAbility` (`add-ability`),
  `EffAddCardType` (`add-card-type`), `EffAddSubtype` (`add-subtype`), `EffSetCardType` (`set-card-type`),
  `EffSetSubtype` (`set-subtype`), `EffAllCreatureTypes` (`all-creature-types`), `EffSetColor` (`set-color`),
  `EffLoseAllAbilities` (`lose-all-abilities`), `EffSetName` (`set-name`), `EffCDAPower` (`cda-power`),
  `EffCDAToughness` (`cda-toughness`), `EffCDAColor` (`cda-color`), `EffCantBlock` (`cant-block`),
  `EffCantBeBlocked` (`cant-be-blocked`), `EffCantBeBlockedBy` (`cant-be-blocked-by`), `EffCantBeTargeted`
  (`cant-be-targeted`), `EffNoUntap` (`no-untap`), `EffMustAttack` (`must-attack`), `EffCantCast` (`cant-cast`),
  `EffCantLose` (`cant-lose`), `EffCantWin` (`cant-win`), `EffSpellsCostLess` (`spells-cost-less`),
  `EffSpellsCostMore` (`spells-cost-more`), `EffNoMaxHandSize` (`no-max-hand-size`), `EffHexproof` (`hexproof`),
  `EffMayPlay` (`may-play`), `EffMayCast` (`may-cast`), `EffFlash` (`flash`), `EffCastWithoutPaying`
  (`cast-without-paying`), `EffExtraLand` (`extra-land`), `EffCantBeCountered` (`cant-be-countered`),
  `EffAdditionalCost` (`additional-cost`), `EffAlternativeCost` (`alternative-cost`), `EffSelfCostLess`
  (`self-cost-less`), then `effectKindCount`.
- `type Effect struct`, fields in order: `Kind EffectKind`; `P, T *Value` (power and toughness amounts); `N *Value`
  (a count or generic amount); `Words []string` (type, subtype or name words); `Colors ColorSet`;
  `Abilities []*Ability` (granted); `Filter *Filter`; `Ref *Ref`; `Cost *Cost`; `Cond *Cond`; `Src string`.
- `type Enter struct`, fields in order: `Tapped bool`; `UnderOwner bool` (under its owner's control);
  `Controller *Ref` (under that player's control); `Counter string` (a counter word); `N *Value` (that many);
  `Effects []*Effect` (enters with these effects); `Src string`.
- Each gets `String()` through `render`, an `Equal` case and an `eq` function, and a row in `ir_test.go`'s `enums`
  and `structTypes`.

**`ability.go`**
- Remove `Ability.Targets` (from the struct, `eqAbility`, `prAbility`). Fix the comment on `ir.Target` in `ref.go`:
  "The enclosing Program's Targets index is the RefTarget Slot."
- `Ability` gains, before `Src`: `SorceryOnly bool` (activate only as a sorcery), `InstantOnly bool` (activate only
  as an instant: Lion's Eye Diamond) and `Once bool` (activate only once).
- Append to `EventKind`, in order: `EvEndStep` (`end-step`), `EvUpkeep` (`upkeep`), `EvBeginCombat`
  (`begin-combat`), `EvOr` (`or`), `EvAttacks` (`attacks`), `EvDealsCombatDamage` (`deals-combat-damage`),
  `EvDealsCombatDamageToPlayer` (`deals-combat-damage-to-player`), `EvDealsNoncombatDamageToPlayer`
  (`deals-noncombat-damage-to-player`), `EvBecomesTarget` (`becomes-target`), `EvUntaps` (`untaps`), `EvDies`
  (`dies`), `EvEnters` (`enters`), `EvPutIntoGraveyard` (`put-into-graveyard`), `EvLeaves` (`leaves`),
  `EvPlayerAttacks` (`player-attacks`), `EvCast` (`cast`), `EvDiscards` (`discards`), `EvDraws` (`draws`),
  `EvGainsLife` (`gains-life`), `EvLosesLife` (`loses-life`), `EvPlaysLand` (`plays-land`), `EvCountersPut`
  (`counters-put`), `EvNextEndStep` (`next-end-step`), `EvReflexive` (`reflexive`), `EvWouldEnter`
  (`would-enter`), `EvWouldPutCounters` (`would-put-counters`), `EvWouldDealDamage` (`would-deal-damage`),
  `EvWouldCreateTokens` (`would-create-tokens`), `EvWouldDie` (`would-die`).
- `Trigger` gains, before `Src`: `Object *Filter` (the event's other party: the recipient, the spell, the
  permanent counters go on); `Cmp *Compare` (the nth such event this turn, or an attacker count); `Word string`
  (a counter word, or `combat`/`noncombat`); `Cond *Cond` (a condition on the trigger event itself, not an
  intervening if); `Args []*Trigger` (`EvOr`'s alternatives).

**Other appends**
- `ref.go`: append `RcFound` (`found`) to `ReceiptKind`: the cards a search found or a reveal-until stopped on.
- `cond.go`: append `CKicked` (`kicked`): the spell was kicked.
- `cost.go`: append `CostLoyalty` (`loyalty`): a loyalty cost, `N` signed.

**`engine/cardc/mtgish/lower.go`**: the one edit is three new cases in `setSrc` (`lower.go:95`): `*ir.Effect`,
`*ir.Enter` and `*ir.Face`.

### 2. Plumbing (`engine/cardc/mtgish/fe_program.go`, new)

- Typed helpers, each `lowerAs` (`lower.go:132`) or a type switch on `c.lower`:
  - `(c *Ctx) Program(x any) (*ir.Program, error)`
  - `(c *Ctx) Steps(x any) ([]*ir.Step, error)`: an action lowering returns `*ir.Step`, or `*ir.Program` whose steps
    are spliced into the enclosing list in order. `Steps` accepts both.
  - `(c *Ctx) StepList(xs []any) (*ir.Program, error)`: the Program of every element's `Steps`, concatenated.
  - `(c *Ctx) Cost(x any) (*ir.Cost, error)`, `(c *Ctx) Effect(x any) (*ir.Effect, error)`,
    `(c *Ctx) Trigger(x any) (*ir.Trigger, error)`, `(c *Ctx) Enter(x any) (*ir.Enter, error)`.
  - `(c *Ctx) Duration(x any) (ir.Duration, error)`: `Expiration.*` lowerings return `*ir.Duration`.
  - `(c *Ctx) Effects(xs []any) ([]*ir.Effect, error)` and `(c *Ctx) Enters(xs []any) (*ir.Enter, error)`. `Enters`
    merges a flag list into one `Enter`: bools OR together; a pointer or string field set by two flags is an error
    `"<key>: conflicting enter flags"`. An empty list, or one holding only `EnterFlag.EntersNormally`, gives nil.
- **Target frames:**

  ```go
  // withTargetFrame runs f with an empty target-declaration list and restores
  // the caller's list afterwards, so f's declarations get slots from 0 and the
  // caller's Ref_ variants resolve after f exactly as before it.
  func (c *Ctx) withTargetFrame(f func() error) error {
      saved := c.targets
      c.targets = nil
      err := f()
      c.targets = saved
      return err
  }
  ```

  `c.targets = nil` (not `[:0]`) is required: the nested frame must not write into the saved slice's backing array.
- `(c *Ctx) NestedAbilities(rules []any) ([]*ir.Ability, error)`: each rule through
  `withTargetFrame(func() error { ab, err = c.Ability(rule); … })`. Every lowering that holds a `Rule` list calls
  it: granted abilities, emblems, token rules. `NestedBody(x any) (*ir.Program, error)` does the same for a delayed
  or reflexive trigger's `Actions`.
- `func withZone(f *ir.Filter, z ir.Zone) *ir.Filter` returns `&ir.Filter{Subject: ir.SubjCard, Zones:
  ir.MakeZoneSet(z), Op: ir.FAll, Args: []*ir.Filter{f}}`, for a `Cards` filter read in one zone.
- `func anyCardIn(z ir.Zone) *ir.Filter` returns `{SubjCard, Zones: z, Op: FPred, Pred: PAny}`.

### 3. Containers (`fe_program.go`)

| key | lowers to |
|---|---|
| `Actions.ActionList(list)` | `c.StepList(list)` |
| `Actions.Targeted(targets, actions)` | in one `withTargetFrame`: each target through `c.Target` (CC-12; it declares the slot), then `c.Program(actions)`; the Program's `Targets` = the targets in order. A body that already has `Targets` is the error `"Actions.Targeted: nested target list"`. |
| `Actions.Modal_ChooseOne(list)` | `Program{Steps: [{VerbChooseModes, N: const 1, Modes: [c.Program(mode)…]}]}` |

`const 1` is `refConst1()` (`fe_ref.go:23`). Every mode that declares targets is itself `Actions.Targeted`, so each
mode's slots start at 0. Deferred: `Actions.TargetedDistributed` (dividing damage).

### 4. Actions A-L (`fe_action_al.go`, new)

"Acting player" is the nearest enclosing `Who`, else the controller (section 1). `RefChoose` with nil `Of` is chosen
by the acting player (CC-01). Each row returns one `*ir.Step` unless it says "splice".

| `Action.` variant (schema args) | `*ir.Step` |
|---|---|
| `AddMana(mp)` | `{VerbAddMana, Mana: mana(mp)}` |
| `AddManaRepeated(mp, n)` | `{VerbAddMana, Mana: mana(mp), N: c.Value(n)}` (N times) |
| `AttachPermanentToPermanent(a, b)` | `{VerbAttach, Obj: c.Ref(a), Other: c.Ref(b)}` |
| `ChooseAColor(cc)` | `{VerbChooseColor, Colors: <ChoosableColor>}` |
| `ChooseAnExiledCard(ce)` | `{VerbChoose, Obj: {RefChoose, Filter: c.Filter(ce), N: const 1}}`; its receipt is `RcChosen` |
| `CopySpellAndMayChooseNewTargets(s)` | `{VerbCopySpell, Obj: c.Ref(s)}` |
| `CopySpellForEachAndMayChooseNewTargets(s, n)` | `{VerbCopySpell, Obj, N: c.Value(n)}` |
| `CounterSpell(s)` | `{VerbCounter, Obj}` |
| `CreateEachPermanentLayerEffectUntil(ps, effs, exp)` | `{VerbApplyEffect, Obj: {RefAll, Filter: c.Filter(ps)}, Effects: c.Effects(effs), Dur: c.Duration(exp)}` |
| `CreateFutureTrigger(ft, actions)` | `{VerbDelayedTrigger, Abilities: [{AbTriggered, Trigger: c.Trigger(ft), Body: c.NestedBody(actions)}]}` |
| `CreateGraveyardCardEffectUntil(cg, effs, exp)` | `{VerbApplyEffect, Obj: c.Ref(cg), Effects, Dur}` |
| `CreatePermanentLayerEffect(p, effs)` | `{VerbApplyEffect, Obj, Effects, Dur: DurPermanent}` |
| `CreatePermanentLayerEffectUntil(p, effs, exp)` | `{VerbApplyEffect, Obj, Effects, Dur}` |
| `CreatePermanentRuleEffectUntil(p, rules, exp)` | `{VerbApplyEffect, Obj, Effects: c.Effects(rules), Dur}` (`PermanentRule` is CC-16b's) |
| `CreatePlayerEffectUntil(pl, effs, exp)` | `{VerbApplyEffect, Obj: c.Ref(pl), Effects, Dur}` (`PlayerEffect` is CC-16b's) |
| `CreatePreventDamageUntil(ev, acts, exp)` | `{VerbCreateReplacement, Dur, Abilities: [{AbReplacement, Trigger: c.Trigger(ev), Body: c.StepList(acts)}]}` (the event and action enums are CC-16b's) |
| `CreateReplaceWouldPutIntoGraveyardUntil(ev, acts, exp)` | as the row above |
| `CreateTokens(toks, flags)` | splice: each `CreatableToken` through `c.Steps`; then each created step's `Enter` = `c.Enters(flags)` merged into it |
| `DestroyEachPermanent(ps)` | `{VerbDestroy, Obj: {RefAll, Filter}}` |
| `DestroyPermanent(p)` | `{VerbDestroy, Obj}` |
| `DiscardACard` | `{VerbDiscard, Obj: {RefChoose, Filter: anyCardIn(ZoneHand), N: const 1}}` |
| `DiscardCards(hs)` | `{VerbDiscard, Obj: {RefAll, Filter: c.Filter(hs)}}` |
| `DiscardNumberCards(n)` | `{VerbDiscard, Obj: {RefChoose, Filter: anyCardIn(ZoneHand), N: c.Value(n)}}` |
| `DoubleCreaturesPowerUntilEndOfTurn(p)` | `{VerbApplyEffect, Obj: r, Effects: [{EffAdjustPT, P: {VPower, Ref: r}, T: const 0}], Dur: DurEndOfTurn}`, r = `c.Ref(p)` |
| `DrawACard` | `{VerbDraw, N: const 1}` |
| `DrawNumberCards(n)` | `{VerbDraw, N}` |
| `EachPlayerAction(ps, a)` | `{VerbForEachPlayer, Obj: {RefAll, Filter: c.Filter(ps)}, Then: Program{c.Steps(a)}}`; each inner top-level step with nil `Who` gets `Who: {RefIterPlayer}` |
| `EndTheTurn` | `{VerbEndTurn}` |
| `Exile(xs, flags)` | splice: one `{VerbExile, Obj: c.Ref(x)}` per `Exilable`, each with the flags below |
| `Fight(a, b)` | `{VerbFight, Source: c.Ref(a), Obj: c.Ref(b)}` |
| `GainControlOfPermanentUntil(p, exp)` | `{VerbGainControl, Obj, Dur}` |
| `GainLife(n)` | `{VerbGainLife, N}` |
| `GainLifeForEach(n, m)` | `{VerbGainLife, N: {VMul, Args: [c.Value(n), c.Value(m)]}}` |
| `GetAnEmblem(rules)` | `{VerbEmblem, Abilities: c.NestedAbilities(rules)}` |
| `If(cond, list)` | `{VerbIf, Cond: c.Cond(cond), Then: c.StepList(list)}` |
| `IfElse(cond, a, b)` | `{VerbIf, Cond, Then: c.StepList(a), Else: c.StepList(b)}` |
| `LookAtPlayersHand(pl)` | `{VerbLookAtHand, Obj: c.Ref(pl)}` |
| `LookAtTheTopCardOfPlayersLibrary(pl)` | `{VerbLook, Obj: {RefTopOfLibrary, Of: c.Ref(pl), N: const 1}}` |
| `LookAtTheTopNumberCardsOfLibrary(n, subs)` | `{VerbLook, Obj: {RefTopOfLibrary, N: c.Value(n)}, Then: c.StepList(subs)}` |
| `LookAtTopOfLibrary` | `{VerbLook, Obj: {RefTopOfLibrary, N: const 1}}` |
| `LoseLife(n)` | `{VerbLoseLife, N}` |

Notes:
- A receipt reference inside a delayed trigger's body (`TheCardExiledThisWay` in Kykar, Zephyr Awakener's) means
  the receipt of the program that created the trigger.
- `ExileFlag.ExilesWithACounter(ct)` sets the step's `Word` to the counter word (`c.lower(ct)`, CC-13).
  `ExileFlag.Limited(exp)` sets `Dur` ("exile until"). Two flags setting one field is an error.

### 5. Satellite enums (in `fe_action_al.go` unless named)

- **`ManaProduce`** lowers to an unexported `*manaList{syms []ir.ManaSym}`; the helper `mana(c, x)` returns the
  slice.
  - `ManaProduceW`/`U`/`B`/`R`/`G` → `[{ManaColored, that colour}]`; `ManaProduceC` → `[{ManaColorless}]`.
  - `AnyManaColor` → `[{ManaColored, Colors: W|U|B|R|G}]`.
  - `And(list)` → the concatenation.
  - `Or(list)` → one `ManaColored` symbol whose Colors is the union. Every alternative must lower to exactly one
    `ManaColored` symbol, else the refusal `ManaProduce.Or:shape`.
- **`ChoosableColor.AnyColor`** → `ir.ColorSet` W|U|B|R|G, by value.
- **`Expiration`** → `*ir.Duration`:
  - `UntilEndOfTurn` → `DurEndOfTurn`.
  - `UntilPlayersNextTurn(pl)`: pl must lower to `{RefYou}`, giving `DurUntilYourNextTurn`; else the refusal
    `Expiration.UntilPlayersNextTurn:player`.
  - `UntilTheEndOfPlayersNextTurn(pl)` → `DurUntilEndOfYourNextTurn`, with the same You rule and refusal suffix.
  - `UntilPermanentLeavesBattlefield(p)`: p must be `{RefSelf}`, giving `DurWhileSourceOnBattlefield`; else
    `Expiration.UntilPermanentLeavesBattlefield:permanent`.
- **`FutureTrigger.AtTheBeginningOfTheNextEndStep`** → `&ir.Trigger{Event: ir.EvNextEndStep}`.
- **`LayerEffect`** → `*ir.Effect`, where `k(n)` is `{VConst, N: n}`:

  | variant | Effect |
  |---|---|
  | `AddAbility(rules)` | `{EffAddAbility, Abilities: c.NestedAbilities(rules)}` |
  | `AddCardtype(t)`, `AddCardtypes(list)` | `{EffAddCardType, Words}` |
  | `AddCreatureType(w)` | `{EffAddSubtype, Words: [w]}` |
  | `AdjustPT(p, t)` | `{EffAdjustPT, P: k(p), T: k(t)}` |
  | `AdjustPTForEach(p, t, n)` | `{EffAdjustPT, P: {VMul, [k(p), v]}, T: {VMul, [k(t), v]}}`, v = `c.Value(n)` |
  | `AdjustPTX(mp, mt, n)` | `{EffAdjustPT, P: v, T: v}`; both `ModX` must be `PlusX`, else the refusal `LayerEffect.AdjustPTX:mod` |
  | `HasAllCreatureTypes` | `{EffAllCreatureTypes}` |
  | `SetColor(sc)` | `{EffSetColor, Colors}` (`SettableColor` is CC-16b's; it returns `ir.ColorSet` by value, 0 for colourless) |
  | `SetCreatureType(w)`, `SetCreatureTypes(list)` | `{EffSetSubtype, Words}` |
  | `SetPT(pt)` | `{EffSetPT, P, T}` from the PT pair |
  | `SetPowerAndToughnessBoth(n)` | `{EffSetPT, P: v, T: v}` |

- **`GraveyardCardEffect.AddAbility(rules)`** → `{EffAddAbility, Abilities: c.NestedAbilities(rules)}`.
- **`ModX.PlusX`** → `*int32` holding 1.
- **`PT`** → an unexported `*ptPair{P, T *ir.Value}`: `PT.PT(p, t)` gives two consts; `PT.ManualPT(p, t)` gives
  `c.Value` of each.
- **`CreatableToken`** (`fe_token.go`, new). Each returns one `*ir.Step` with `N` the count:

  | variant | Step |
  |---|---|
  | `TokenWithPT(pt, colors, supers, types, subs, rules)` | `{VerbCreateToken, N: const 1, Token: face}` |
  | `NamedTokenWithPT(name, pt, colors, supers, types, subs, rules)` | the same, with `Token.Name = name` |
  | `NumberTokens(n, tok)` | the inner step with `N = c.Value(n)` |
  | `FoodToken` | `{VerbCreateToken, N: const 1, Token: {Name: "Food", Types: ["Artifact"], Subtypes: ["Food"]}}` |
  | `TreasureToken` | the same with `Treasure` |
  | `TokenCopyOfPermanent(p, effs)`, `TokenCopyOfGraveyardCard(cg, effs)` | `{VerbCreateTokenCopy, N: const 1, Obj: c.Ref(arg), Effects}` |

  - `face` is `&ir.Face{ColorIndicator: colors, Supertypes, Types, Subtypes, Power: &ir.PT{N}, Toughness:
    &ir.PT{N}, Abilities: c.NestedAbilities(rules)}`.
  - The PT must be `PT.PT`. A `PT.ManualPT` is the refusal `CreatableToken.TokenWithPT:manual-pt`.
  - A predefined token (Food, Treasure) carries no abilities: its rules come from its name (CR 111.10). The gorge
    backend maps it to Forge's token script.
- **`TokenColorList`**: `Colors(list)` → `ir.ColorSet` by value, through `charsColorNames` (`fe_chars.go:14`);
  `Colorless` → `ir.ColorSet(0)`.
- **`TokenSubtypes.SubtypesList(list)`** → `[]string` by value.
- **`TokenFlag.EntersTapped`** → `&ir.Enter{Tapped: true}`.
- **`TokenCopyEffects`**: `NoTokenCopyEffects` → a nil `[]*ir.Effect` by value; `TokenCopyEffects(list)` →
  `c.Effects(list)` by value. **`TokenCopyEffect`**: `AddAbility(rules)` → `EffAddAbility`;
  `AddCreatureTypes(list)` → `{EffAddSubtype, Words}`.
- **`LookAtTopOfLibraryAction`** → `*ir.Step`. Inside a `VerbLook` step's `Then`, a `RefChoose` chooses among the
  looked-at cards, and a nil `Obj` means "the remaining looked-at cards".

  | variant | Step |
  |---|---|
  | `MayRevealACardOfTypeAndPutIntoHand(cs)` | `{VerbMove, Optional, Obj: {RefChoose, Filter: c.Filter(cs), N: const 1}, To: ZoneHand, Word: "reveal"}` |
  | `MayRevealAndPutACardOfTypeOntoTheBattlefield(cs, flags)` | the same with `To: ZoneBattlefield, Enter: c.Enters(flags)` |
  | `MayPutAGenericCardOnTopOfLibrary` | `{VerbMove, Optional, Obj: {RefChoose, N: const 1}, To: ZoneLibrary, Word: "top"}` |
  | `PutTheRemainingCardsIntoGraveyard` | `{VerbMove, To: ZoneGraveyard}` |
  | `PutTheRemainingCardsOnTheBottomOfLibraryInARandomOrder` | `{VerbMove, To: ZoneLibrary, Word: "bottom-random"}` |
  | `PutTheRemainingCardsOnTheBottomOfLibraryInAnyOrder` | `… Word: "bottom-any-order"` |
  | `PutTheRemainingCardsOnTopOfLibraryInAnyOrder` | `… Word: "top-any-order"` |

  The library-position words are exactly `top`, `bottom`, `bottom-random`, `top-any-order`, `bottom-any-order` and
  `top-or-bottom` (CC-15b uses the last).
- `EnterFlag` itself is CC-15b's. Until it lands, tests stub it.

### 6. Deferred (the coverage test lists each with its reason)

- `Action.AddManaWithModifiers` (spend restrictions), `Action.BecomeTheMonarch` (monarch),
  `Action.BeginGameWithCardOnBattlefield` (pregame actions), `Action.CastAnyNumberOfSpellsFromExileWithoutPaying`,
  `Action.ChooseACardName` (needs `CardsInOracle` and the chosen-name readers CC-12 deferred),
  `Action.ChooseAPermanentOfEachPermanentTypeAvailable`, `Action.EachPlayerMayCostOrFallback`,
  `Action.EachPlayerStartingWithAction`;
- `Actions.TargetedDistributed`;
- `ManaProduce.EachManaColorAmongPermanents`, `ManaProduce.ManaOfTheChosenColor`;
- `CreatableToken.ThoseTokens`;
- `LookAtTopOfLibraryAction.APlayerChoosesAPileTopPutIntoHand`, `SeparateIntoFaceUpFileAndFaceDownPile` (piles);
- `Expiration.UntilAPlayerBecomesTheMonarch`.

No key that a wave-1 coverage test lists as deferred is registered here.

## Implementation steps

1. IR: `program.go`, `effect.go`, `ability.go`, `ref.go`, `cond.go`, `cost.go`, then `equal.go`, `print.go` and
   the `ir_test.go` tables. Run the IR tests before touching the front end.
2. `lower.go` `setSrc` cases. `fe_program.go`, `fe_action_al.go`, `fe_token.go`, each registering in its own
   `init()`.
3. Tests.

## Tests

In `engine/cardc/ir`: the existing table-driven tests cover the new enums and structs once their rows are added.
Add to `TestPrintMatchesEqual`'s spot checks
`str(&Step{Verb: VerbDraw, N: two}, "(step draw :n (value const 2))")` (the existing `knode` syntax,
`print.go:102`).

In `engine/cardc/mtgish`:
- `TestTargetFrames`: synthetic `Actions.Targeted([TargetPermanent], ActionList[…])` whose list holds, in order,
  a step referencing `Ref_TargetPermanent`, a `CreatePermanentLayerEffectUntil` granting
  `AddAbility([<stub rule>])`, and a second reference to `Ref_TargetPermanent`. The stub rule (via `withStubs`)
  lowers its own `Actions.Targeted([TargetPlayer], …Ref_TargetPlayer…)`. Check:
  - both outer references are slot 0;
  - the nested Program's `Targets` has one entry and its reference is slot 0;
  - after the call, `c.TargetSlot("Ref_TargetPermanent")` is unchanged from before it.
- `TestModesHaveOwnTargets`: a `Modal_ChooseOne` of two `Targeted` modes, each with `TargetPermanent`: both modes'
  references are slot 0, and each mode Program has one target.
- `TestActionLoweringAL`: one synthetic JSON row per table row in sections 3-5, compared with `ir.Equal`, stubbing
  `Cost`, `PermanentRule`, `PlayerEffect`, `EnterFlag`, `SettableColor` and the replacement enums with `withStubs`.
- `TestStepsSplice`: `CreateTokens` with two tokens and `Exile` with two exilables each splice two steps into the
  enclosing `ActionList`.
- `TestEntersMerge`: `EntersTapped` plus `EntersUnderOwnersControl` merge; two counter flags error.
- `TestActionALSliceVariantsRegistered`: put the helper `actionCoverage(t, lo, hi byte, deferred)` in
  `fe_action_al_test.go`. It filters `Census(proofSlice(t))` to `Action.<V>` keys with `lo <= V[0] <= hi`, and
  calls `familyCoverageProblems` (`helpers_test.go:108`) with `[]string{"Action"}`. CC-15b reuses the helper. This
  test calls it with `'A', 'L'` and the Action deferrals.
- `TestProgramSliceVariantsRegistered`: `checkFamilyCoverage` over `Actions`, `ManaProduce`, `ChoosableColor`,
  `Expiration`, `FutureTrigger`, `LayerEffect`, `GraveyardCardEffect`, `ModX`, `PT`, `CreatableToken`,
  `TokenColorList`, `TokenSubtypes`, `TokenFlag`, `TokenCopyEffects`, `TokenCopyEffect`, `ExileFlag` and
  `LookAtTopOfLibraryAction`, with the deferrals above.
- `TestCompileProofSliceNoPanic` (CC-10's) still passes.

## Done means

- [ ] `make test-one PKG=./engine/cardc/ir` passes.
- [ ] `make test-one PKG=./engine/cardc/mtgish` passes (not ENV): every test in the package, wave 1's included.
- [ ] `make test-one PKG=./internal/archtest RUN='^(TestDependencyOrderHolds|TestNothingImportsCardc|TestPublicBoundary)$'` passes.
- [ ] No `make test-targeted` or `make test-smart` per ticket: one `make test-smart BASE=origin/cardc` runs at the
  wave-2 head.

## Out of scope

- `Action` variants M-Z (CC-15b); every `Rule`, `Trigger`, `Cost` and static or replacement enum (CC-16a/b).
- Any backend (CC-20+).
- Anything on GitHub outside gorge. Never open a pull request to i5jb/mtgish.

## Collisions

The wave-2 hub. It is the only wave-2 ticket that edits `engine/cardc/ir` or `lower.go`. CC-15b, CC-16a and CC-16b
depend on it and run in parallel after it, each in its own new `fe_*.go` files. No new package, so no
`packages.tiers.json` row.
