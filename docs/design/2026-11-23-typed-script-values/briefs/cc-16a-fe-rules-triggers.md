# feat(cardc): CC-16a front end, ability rules, triggers and keywords with arguments

Kind: feature

Target branch: `cardc`. Complexity: medium. Seat: Sonnet.
Design of record: `docs/design/2026-11-23-typed-script-values/PLAN.md` on `cardc`, section 1, and CC-15a's brief
(`cc-15a-fe-actions-al.md`), which fixes every IR shape used here. Runs after CC-15a (`cc-15a-fe-actions-al`).

## Goal

Register the `Rule` variants that make spell, activated and triggered abilities and keywords with arguments, every
`Trigger` variant the proof slice uses, and the satellites `ActivateModifier`, `DamageSources`, `Protectable` and
`ProtectableColor`. After this ticket and CC-15a/b, a proof-slice spell, activated or triggered ability whose
actions, costs and conditions are registered lowers to a full `*ir.Ability`.

## Behaviour

- `c.Ability(rule)` returns the `*ir.Ability` below for each listed `Rule.<V>`.
- The CC-10 driver (`compile.go:169`) still calls `c.ResetTargets()` before each top-level rule. Targets are
  declared only inside `Actions.Targeted`, which opens its own frame (CC-15a), so slots are per Targeted program.
- A wrapper rule (`Rule.If`, `Rule.FromGraveyard`, `Rule.FromStack`) is the **same** ability as the rule it wraps,
  not a nested one. It does not open a frame.
- No edit to `engine/cardc/ir`, `lower.go` or any CC-15a file.

## Design

Decided. `const n` is `{VConst, N: n}`; `refConst1()` is at `fe_ref.go:23`.

### 1. Ability rules (`fe_rule.go`, new)

| `Rule.` variant (schema args) | `*ir.Ability` |
|---|---|
| `SpellActions(actions)` | `{AbSpell, Body: c.Program(actions)}` |
| `SpellActions_Kicker(cost, kicked, unkicked)` | `{AbSpell, Keyword: {Name: "Kicker", Cost: c.Cost(cost)}, Body: Program{[{VerbIf, Cond: {CKicked}, Then: c.Program(kicked), Else: c.Program(unkicked)}]}}` |
| `Activated(cost, actions)` | `{AbActivated, Cost: c.Cost(cost), Body: c.Program(actions)}` |
| `ActivatedWithModifiers(cost, actions, mod)` | as `Activated`, then the modifier below applied |
| `TriggerA(trig, actions)` | `{AbTriggered, Trigger: c.Trigger(trig), Body}` |
| `TriggerI(trig, cond, actions)` | `{AbTriggered, Trigger, Cond: c.Cond(cond), Body}`; `Ability.Cond` on a trigger is the intervening if (CR 603.4) |
| `TriggerOnceEachTurn(trig, actions)` | as `TriggerA` with `Trigger.OncePerTurn = true` |
| `If(cond, rules)` | the one inner rule's `c.Ability`, with `Cond` = cond (or `{CAll, [cond, inner.Cond]}` when the inner has one); more than one inner rule is the refusal `Rule.If:multi` |
| `FromGraveyard(rule)` | the inner ability with `Zones = ir.MakeZoneSet(ZoneGraveyard)` |
| `FromStack(rule)` | the inner ability with `Zones = ir.MakeZoneSet(ZoneStack)` |

`ActivateModifier` lowers to an unexported `*abMod` that sets one field:
- `ActivateOnlyAsASorcery` → `SorceryOnly`;
- `ActivateOnlyAsAnInstant` → `InstantOnly`;
- `ActivateOnlyOnce` → `Once`;
- `ActivateOnlyIf(cond)` → `Cond` (`{CAll, …}` with any existing one).

A planeswalker's `Activated` with `Cost.Loyalty` needs nothing extra: the loyalty part (CC-16b) marks it.

### 2. Keywords with arguments (`fe_rule.go`)

Each returns `&ir.Ability{Kind: AbKeyword, Keyword: &ir.Keyword{…}}`. Name is the keyword's rules name, which is
also gorge's `K:` head (CC-10's rule).

| `Rule.` variant | `ir.Keyword` |
|---|---|
| `Ward(cost)` | `{Name: "Ward", Cost: c.Cost(cost)}` |
| `Equip(ps, cost)` | `{Name: "Equip", Cost, Filter: c.Filter(ps)}` |
| `Flashback(cost)` | `{Name: "Flashback", Cost}` |
| `Kicker(cost)` | `{Name: "Kicker", Cost}` |
| `Miracle(cost)` | `{Name: "Miracle", Cost}` |
| `MiracleX(syms)` | `{Name: "Miracle", Cost: {Mana: syms, HasMana: true}}`, each symbol through `c.lower` (CC-10 registers `ManaSymbol.*`; each returns `*ir.ManaSym`) |
| `Surge(syms)` | `{Name: "Surge", Cost: {Mana, HasMana: true}}` |
| `Affinity(ps)` | `{Name: "Affinity", Filter: c.Filter(ps)}` |
| `EnchantPermanent(ps)` | `{Name: "Enchant", Filter}` |
| `Protection(pr)` | `{Name: "Protection", Filter: c.Filter(pr)}` |
| `HexproofFrom(pr)` | `{Name: "Hexproof", Filter}` (a Hexproof with a Filter is "hexproof from") |

`Protectable` → `*ir.Filter` (Subject `SubjObject`):
- `FromColor(pc)` → `{FPred, PColor, Colors: <ProtectableColor>}`;
- `FromEverything` → `{FPred, PAny}`;
- `FromTypes(cs)` → `c.Filter(cs)`.

`ProtectableColor.Colors(list)` → `ir.ColorSet` by value, each element through `filtColor` (`fe_filter.go:130`).

### 3. Triggers (`fe_trigger.go`, new)

Each `Trigger.<V>` → `*ir.Trigger`.
- **Subject** is the event's actor (the attacking creature, the casting player, the dying permanent).
- **Object** is the other party (the damaged player, the cast spell, the permanent counters go on).
- `S(i)` and `O(i)` mean argument i through `c.Filter`.

| variant (schema args) | `ir.Trigger` |
|---|---|
| `AtTheBeginningOfAPlayersEndStep(pls)` | `{EvEndStep, Subject: S(0)}` (whose turn) |
| `AtTheBeginningOfAPlayersUpkeep(pls)` | `{EvUpkeep, Subject: S(0)}` |
| `AtTheBeginningOfCombatDuringAPlayersTurn(pls)` | `{EvBeginCombat, Subject: S(0)}` |
| `If(cond, trig)` | the inner trigger with `Cond = c.Cond(cond)` |
| `Or(list)` | `{EvOr, Args: [c.Trigger(t)…]}` |
| `WhenACreatureAttacks(ps)` | `{EvAttacks, Subject: S(0)}` |
| `WhenACreatureDealsCombatDamage(ps)` | `{EvDealsCombatDamage, Subject: S(0)}` |
| `WhenACreatureDealsCombatDamageToAPlayer(ps, pls)` | `{EvDealsCombatDamageToPlayer, Subject: S(0), Object: O(1)}` |
| `WhenASourceDealsNoncombatDamageToAPlayer(ds, pls)` | `{EvDealsNoncombatDamageToPlayer, Subject: S(0), Object: O(1)}` |
| `WhenAPermanentBecomesTheTargetOfASpell(ps, ss)` | `{EvBecomesTarget, Subject: S(0), Object: O(1)}` (Subject is the targeted permanent) |
| `WhenAPermanentBecomesUntapped(ps)` | `{EvUntaps, Subject: S(0)}` |
| `WhenAPermanentDies(ps)` | `{EvDies, Subject: S(0)}` |
| `WhenAPermanentEntersTheBattlefield(ps)` | `{EvEnters, Subject: S(0)}` |
| `WhenAPermanentEntersTheBattlefieldUnderAPlayersControl(ps, pls)` | `{EvEnters, Subject: S(0), Object: O(1)}` (Object: its controller) |
| `WhenAPermanentIsPutIntoAPlayersGraveyard(ps, pls)` | `{EvPutIntoGraveyard, Subject: S(0), Object: O(1)}` (Object: the graveyard's owner) |
| `WhenAPermanentLeavesTheBattlefield(ps)` | `{EvLeaves, Subject: S(0)}` |
| `WhenAPlayerAttacks(pls)` | `{EvPlayerAttacks, Subject: S(0)}` |
| `WhenAPlayerAttacksWithANumberOfCreatures(pls, cmp, ps)` | `{EvPlayerAttacks, Subject: S(0), Cmp: c.Compare(cmp), Object: O(2)}` (Cmp: how many attackers match Object) |
| `WhenAPlayerCastsASpell(pls, ss)` | `{EvCast, Subject: S(0), Object: O(1)}` |
| `WhenAPlayerDiscardsACard(pls, cs)` | `{EvDiscards, Subject: S(0), Object: O(1)}` |
| `WhenAPlayerDrawsACard(pls)` | `{EvDraws, Subject: S(0)}` |
| `WhenAPlayerDrawsTheirNthCardEachTurn(pls, cmp)` | `{EvDraws, Subject: S(0), Cmp: c.Compare(cmp)}` (the event is the player's Cmp-th this turn) |
| `WhenAPlayerGainsLife(pls)` | `{EvGainsLife, Subject: S(0)}` |
| `WhenAPlayerGainsLifeForTheFirstTimeEachTurn(pls)` | `{EvGainsLife, Subject: S(0), Cmp: {CmpEQ, V: const 1}}` |
| `WhenAPlayerLosesLife(pls)` | `{EvLosesLife, Subject: S(0)}` |
| `WhenAPlayerPlaysALand(pls, ps)` | `{EvPlaysLand, Subject: S(0), Object: O(1)}` |
| `WhenAPlayerPutsAnyNumberOfCountersOfTypeOnAPermanent(pls, ct, ps)` | `{EvCountersPut, Subject: S(0), Word: counter(1), Object: O(2)}` |
| `WhenAnyNumberOfCountersOfTypeArePutOnAPermanent(ct, ps)` | `{EvCountersPut, Word: counter(0), Object: O(1)}` (nil Subject: any player) |

- `counter(i)` is argument i through `c.lower`, CC-13's `CounterType` string.
- "First time each turn" and "the nth card each turn" share one encoding, a `Cmp` on the event's ordinal this turn,
  so the backend has one rule for both.

`DamageSources.ControlledByAPlayer(pls)` → `&ir.Filter{Subject: SubjObject, Op: FPred, Pred: PControlledBy, Ref:
&ir.Ref{Kind: RefAll, Filter: c.Filter(pls)}}`. CC-16b's damage replacement reads it too.

### 4. Nested abilities and target frames (the operator's open item)

The mechanism is CC-15a's `withTargetFrame`. CC-16a owns its contract at the rule level:
1. A rule lowered as **part of another ability's effect** (granted by `LayerEffect.AddAbility` or
   `StaticLayerEffect.AddAbility`, an emblem's, a token's) goes through `c.NestedAbilities`. So does a delayed or
   reflexive trigger's body (`c.NestedBody`). Each is a separate ability: its `Actions.Targeted` slots start at 0,
   and its event refs (`Trigger_ThatPermanent`) are its own trigger's.
2. After the nested ability, the outer ability's declarations are exactly as before. A later `Ref_TargetPermanent`
   in the outer body resolves to the outer slot.
3. A wrapper rule (section 1) is not nested.
4. A `Ref_*` inside a nested ability that names only an outer target is unbound, the `:unbound` refusal (CC-11).
   None occurs in the proof slice at `1553384` (measured: no `Ref_*` under a nested `Rule` without a `Targeted`
   between them).

## Deferred (the coverage tests list each with its reason)

- `Rule.IfCardIsInOpeningHand` (pregame actions);
- `ProtectableColor.TheChosenColor` (chosen colour, as CC-12's `Color.TheChosenColor`).

## Implementation steps

1. Add `fe_rule.go` and `fe_trigger.go`, registering in `init()`.
2. Write the tests.

## Tests

In `engine/cardc/mtgish`:
- `TestRuleLowering`: one synthetic JSON row per section 1 and 2 row, and per `ActivateModifier`, compared with
  `ir.Equal`. Costs (CC-16b), actions and conditions are stubbed with `withStubs`. It includes `Rule.If` over two
  rules (the `:multi` refusal) and `Rule.If` over a `TriggerI` (the `CAll` of both conditions).
- `TestTriggerLowering`: one row per section 3 row.
- `TestNestedAbilityTargets`: the operator's case, built from real variant names with stubs only for leaves.

  ```
  Rule.TriggerA(<stub trigger>,
    Actions.Targeted([Target.TargetPermanent(<stub>)], Actions.ActionList([
      Action.CreatePermanentLayerEffectUntil(Permanent.Ref_TargetPermanent,
        [LayerEffect.AddAbility([Rule.Activated(<stub cost>,
           Actions.Targeted([Target.TargetPlayer(<stub>)],
             Actions.ActionList([Action.PlayerAction(Player.Ref_TargetPlayer, Action.DrawACard)])))])],
        Expiration.UntilEndOfTurn),
      Action.DestroyPermanent(Permanent.Ref_TargetPermanent)])))
  ```

  Check:
  - the outer Program has one target and both outer references are slot 0;
  - the granted ability's Program has its own single target, and its `Who` is slot 0;
  - lowering the same rule with the `DestroyPermanent` step first gives the same slots.
- `TestWrapperIsNotNested`: `Rule.FromGraveyard(Rule.Activated(…Targeted([TargetPermanent], …)…))` gives one
  ability with `Zones` Graveyard whose Program holds the target.
- `TestRuleKeysRegistered`: asserts `Registered` for every `Rule.<V>` key in sections 1 and 2. They are not a
  whole enum: CC-10 and CC-16b own the other `Rule` keys.
- `TestTriggerSliceVariantsRegistered`: `checkFamilyCoverage` over `Trigger`, `ActivateModifier`, `DamageSources`,
  `Protectable` and `ProtectableColor`, with the deferrals above.

## Done means

- [ ] `make test-one PKG=./engine/cardc/mtgish` passes (not ENV): every test in the package.
- [ ] `make test-one PKG=./internal/archtest RUN='^(TestDependencyOrderHolds|TestNothingImportsCardc|TestPublicBoundary)$'` passes.
- [ ] No `make test-targeted` or `make test-smart` per ticket: one `make test-smart BASE=origin/cardc` runs at the
  wave-2 head.

## Out of scope

- `Cost` and every static, replacement, cast and CDA rule (CC-16b).
- Any edit under `engine/cardc/ir`, or to `lower.go` or CC-15a's files.
- Anything on GitHub outside gorge. Never open a pull request to i5jb/mtgish.

## Collisions

Depends on CC-15a. Its only files are `fe_rule.go`, `fe_trigger.go` and their tests. It runs in parallel with
CC-15b and CC-16b. The `Rule` enum is split by key: CC-10 owns the 21 bare keywords, this ticket sections 1 and 2,
CC-16b the rest, so no key is registered twice. No new package.
