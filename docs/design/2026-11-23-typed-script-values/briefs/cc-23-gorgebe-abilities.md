# feat(cardc): CC-23 gorge backend activated, triggered, static and replacement abilities and keyword arguments

Kind: feature

Target branch: `cardc`. Complexity: medium. Implementer: Sonnet.
Design of record: `docs/design/2026-11-23-typed-script-values/PLAN.md` on `cardc`, sections 1.3 and 6. Runs after
CC-22a and CC-22b (on `cardc`) and CC-16a/b.

## Goal

Register the ability printers for `AbActivated`, `AbTriggered`, `AbStatic` and `AbReplacement`, and for keywords with
arguments, plus the two replacement-body steps (`modify-enter`, `set-amount`). After this ticket every ability kind
the front end lowers has a printer, and each refusal names the construct it stops at.

## Workspace facts (verified at `35902648b9`, plus wave-3's CC-21a..22b)

- `emit.go` holds `registerAbility`, `p.program`, `p.render`, `p.sub` (an `Execute$` / `ReplaceWith$` SVar), `p.ev`
  and `p.evSelf`, `condForm.onTrigger`. CC-22a's `print.go` dispatches every non-bare-keyword ability to
  `abilityFns[a.Kind]`, and prints `p.lines` after the bare `K:` lines. CC-22b's `p.costText` prints a cost. Read all
  three first.
- IR shapes: `ir.Ability` `engine/cardc/ir/ability.go:351`, `ir.Trigger` `ability.go:380`, `EventKind`
  `ability.go:267-300`, `ir.Keyword` `ability.go:371`, `ir.Effect` `effect.go:108`, `ir.Enter` `effect.go:128`.
- On the 149 refusal-free proof-slice cards at this head: 72 triggered, 39 activated, 19 replacement and 14 static
  abilities. Trigger events: `enters` 36 (all "this enters"), `would-enter` 18 (replacements, all of the source
  itself), `attacks` 5, `draws` 5, `gains-life` 5, `cast` 4, `dies` 4, `upkeep` 3, `deals-combat-damage-to-player` 3,
  `end-step` 2, and one each of `becomes-target`, `counters-put`, `deals-noncombat-damage-to-player`, `loses-life`,
  `next-end-step`, `or`, `put-into-graveyard`, `would-deal-damage`.
- Keywords with arguments there: `Ward` with a mana cost and with a life cost, `Flashback`, `Kicker` (on a spell
  ability's `Keyword` field), `Miracle`, and `Protection` from one colour.
- The 14 statics, by effect: `adjust-pt` with or without keywords (3), `add-ability` under a condition (1),
  `cant-block`, `cant-be-blocked`, `cant-be-countered`, `cant-lose`, `cant-win`, `no-max-hand-size`, `flash`,
  `cast-without-paying`, `spells-cost-more`, `alternative-cost`, `additional-cost` (1 each).
- `TestServedCardsMatchForge` (CC-22a's shape form) fails any served card whose ability shape differs from Forge's.
  Several IR forms are a different Forge form: "enters with N counters" is a `K:etbCounter` keyword, and "can't
  lose", "can't win" and "can't be countered" are `R:` replacements. Section 4 prints Forge's form.
- `TestNoVerbatimForgeLines` (`internal/archtest/public_boundary_test.go:498`) fails on any whole corpus script line
  of 30 or more characters in a committed file. Assert on parameter lists.

## Design

Decided. The seat makes no design calls; anything this brief does not cover is `Unprintable`
(`activated:<what>`, `event:<kind>`, `static:<what>`, `repl:<what>`, `keyword:<Name>`).

### 1. Activated abilities (`ability_activated.go`)

`head := p.program(a.Body)`, then put first on `head`: `Cost$ <p.costText(a.Cost)>` (a nil cost is
`activated:no-cost`). Then add `SorcerySpeed$ True` (`SorceryOnly`), `ActivationLimit$ 1` (`Once`),
`ActivationZone$ <zones>` when `Zones` is set and is not battlefield alone (capitalised, comma-joined),
`Planeswalker$ True` when the cost has a `CostLoyalty` part, and `a.Cond`'s `onTrigger()` params (an activation
restriction reads `CheckSVar$`, `IsPresent$` and `PlayerTurn$`, `engine/effects/activation_params.go:395-439`).
`InstantOnly` is `activated:instant-only`. The line is `A:` + `p.render("AB", head)`.

### 2. Triggered abilities (`ability_triggered.go`)

Set `p.ev = a.Trigger.Event` and `p.evSelf` = the trigger's `Subject` is exactly `PIs` of `RefSelf` (for `cast`,
the `Object` is). Print the body, name it `p.sub("Trig"+head.API, head)`, and write
`T:Mode$ <mode> | <params> | Execute$ <name>`. Reset `p.ev` and `p.evSelf` after.

| event | params |
|---|---|
| `enters`, self | `Mode$ ChangesZone`, `Origin$ Any`, `Destination$ Battlefield`, `ValidCard$ Card.Self` |
| `enters`, other | `Mode$ ChangesZone`, `Destination$ Battlefield`, `ValidCard$ <valid(Subject)>`, `TriggerZones$ Battlefield` |
| `dies` or `put-into-graveyard`, self | `Mode$ ChangesZone`, `Origin$ Battlefield`, `Destination$ Graveyard`, `ValidCard$ Card.Self` |
| `dies`, other | the same with `ValidCard$ <valid(Subject)>` and `TriggerZones$ Battlefield` |
| `leaves`, self | `Mode$ ChangesZone`, `Origin$ Battlefield`, `Destination$ Any`, `ValidCard$ Card.Self` |
| `attacks`, self | `Mode$ Attacks`, `ValidCard$ Card.Self` |
| `attacks`, other | `Mode$ Attacks`, `ValidCard$ <valid(Subject)>`, `TriggerZones$ Battlefield` |
| `upkeep` / `end-step` / `begin-combat` | `Mode$ Phase`, `Phase$ Upkeep` / `End of Turn` / `BeginCombat`, `ValidPlayer$ <valid(Subject)>` (omitted when Subject is nil or any player), `TriggerZones$ Battlefield` |
| `draws` | `Mode$ Drawn`, `ValidCard$ Card.YouCtrl` (Subject you) or `Card.OppCtrl` (Subject opponent of you), `TriggerZones$ Battlefield`; `Cmp` EQ n → `Number$ <n>` |
| `gains-life` / `loses-life` | `Mode$ LifeGained` / `LifeLost`, `ValidPlayer$ <valid(Subject)>`, `TriggerZones$ Battlefield`; `Cmp` EQ 1 → `FirstTime$ True` |
| `cast`, Object self | `Mode$ SpellCast`, `ValidCard$ Card.Self` |
| `cast`, other | `Mode$ SpellCast`, `ValidCard$ <valid(Object)>`, `ValidActivatingPlayer$ <valid(Subject)>`, `TriggerZones$ Battlefield` |
| `deals-combat-damage-to-player` / `deals-noncombat-damage-to-player` | `Mode$ DamageDone`, `ValidSource$ <Card.Self when self, else valid(Subject)>`, `ValidTarget$ <valid(Object), Player when nil>`, `CombatDamage$ True` / `False`, `TriggerZones$ Battlefield` unless self |
| `becomes-target` | `Mode$ BecomesTarget`, `ValidTarget$ <valid(Subject)>` (`Card.Self` when self), `ValidSource$ <valid(Object) with each term's base Card written Spell>`, `TriggerZones$ Battlefield` |
| `counters-put` | `Mode$ CounterAddedOnce`, `CounterType$ <Word>`, `ValidCard$ <valid(Object)>`, `ValidSource$ <valid(Subject)>`, `TriggerZones$ Battlefield` |
| `or` | one `T:` line per alternative, in order, sharing one `Execute$` SVar; every line after the first adds `Secondary$ True`. `p.ev` is each alternative's event while its params print; the body prints once, with `p.ev` the first alternative's. An `or` whose alternatives disagree on `p.evSelf` is `event:or-mixed` |
| any other event | `event:<kind>` |

On every trigger line, then:
- `a.Zones` set and not battlefield alone replaces `TriggerZones$` with those zones.
- `Trigger.OncePerTurn` → `ActivationLimit$ 1`.
- `Trigger.Cmp` not covered by the table is `event:cmp`; `Trigger.Word` outside `counters-put` is `event:word`.
- `Trigger.Cond` and `a.Cond` each add their `onTrigger()` params; both set is `event:two-conds`.

### 3. Static abilities (`ability_static.go`)

The body must be one `apply-effect` step with `Dur` `while-active`, else `static:body`. `a.Cond`'s `onTrigger()`
params go on the line (a source-only present test folds to `Card.Self+...`). `Obj` and the effects decide the line:

| effects, Obj | line |
|---|---|
| `adjust-pt` and/or `add-ability` (bare keywords); Obj self, or `RefAll` over objects | `S:Mode$ Continuous`, `Affected$ <Card.Self, or valid(filter)>`, `AddPower$ <p>`, `AddToughness$ <t>` (each omitted when 0), `AddKeyword$ <K1> & <K2>` |
| `cant-block`, Obj self | `S:Mode$ CantBlock`, `ValidCard$ Card.Self` |
| `cant-be-blocked`, Obj self | `S:Mode$ CantBlockBy`, `ValidAttacker$ Card.Self` |
| `no-max-hand-size`, Obj you | `S:Mode$ Continuous`, `Affected$ You`, `SetMaxHandSize$ Unlimited` |
| `flash` with a spell filter, Obj you | `S:Mode$ CastWithFlash`, `ValidCard$ <valid(filter)>`, `ValidSA$ Spell`, `Caster$ You` |
| `cast-without-paying`, filter any spell, Obj you | `S:Mode$ Continuous`, `Affected$ Card.nonLand+YouOwn`, `MayPlay$ True`, `MayPlayWithoutManaCost$ True`, `AffectedZone$ Hand` |
| `spells-cost-more` / `spells-cost-less` with a generic mana `Cost` of n; Obj `RefAll` players | `S:Mode$ RaiseCost` / `ReduceCost`, `Activator$ <valid(Obj.Filter)>`, `Type$ Spell`, `Amount$ <n>` (a spell filter other than any spell adds `ValidCard$ <valid>`) |
| `alternative-cost`, Obj self | `S:Mode$ AlternativeCost`, `ValidSA$ Spell.Self`, `EffectZone$ All`, `Cost$ <costText>` |
| `cant-lose`, Obj players | `R:Event$ GameLoss`, `ActiveZones$ Battlefield`, `ValidPlayer$ <valid players>`, `Layer$ CantHappen` |
| `cant-win`, Obj players | `R:Event$ GameWin`, the same params |
| `cant-be-countered`, Obj self, `a.Zones` stack | `R:Event$ Counter`, `ValidCard$ Card.Self`, `ValidSA$ Spell`, `Layer$ CantHappen` |
| `additional-cost`, or any other | `static:<effect kind>` |

"Obj players" is `RefYou` (`You`) or `RefAll` over players (its Valid list). The `R:` rows are Forge's form of these
statics; they are printed as replacements so the shape matches.

### 4. Replacements (`ability_replacement.go`) and their steps

Set `p.ev` to the event and `p.evSelf` = `Trigger.Ref` is `RefSelf`.

| event, body | output |
|---|---|
| `would-enter` self, body exactly one `modify-enter` whose `Enter` is a counter with `N` and nothing else | no `R:` line: `K:etbCounter:<Counter>:<amount(N)>` appended to `p.lines` |
| `would-enter` self, any other body | `R:Event$ Moved`, `ValidCard$ Card.Self`, `Destination$ Battlefield`, `ReplacementResult$ Updated`, `ReplaceWith$ <p.sub("DB"+head.API, head)>` |
| `would-deal-damage` | `R:Event$ DamageDone`, `ActiveZones$ Battlefield`, `ValidSource$ <valid(Subject)>`, `ValidTarget$ <valid(Object)>`, `ReplaceWith$ <name>` |
| any other | `repl:<event>` |

Steps (`step_replace.go`), valid only while `p.ev` is a `would-*` event (else `verb:<verb>-outside-replacement`):
- `VerbModifyEnter`: `Enter.Tapped` alone → `Tap`, `Defined$ Self`, `ETB$ True`; a counter alone → `PutCounter`,
  `Defined$ Self`, `ETB$ True`, `CounterType$ <Counter>`, `CounterNum$ <amount(N)>`; anything else
  `verb:modify-enter`. (Hallowed Fountain's pay-life-or-tapped body reaches this through CC-22b's unless idiom.)
- `VerbSetAmount` under `would-deal-damage`: `ReplaceEffect`, `VarName$ DamageAmount`, `VarValue$ <amount(N)>`.

A `would-enter` counter body under a condition (Goblin Boarders' raid) is `repl:etb-counter-cond`.

### 5. Keywords with arguments (`keyword_args.go`)

`registerAbility(ir.AbKeyword, ...)`; CC-22a calls it only for a keyword with arguments. Append to `p.lines`:

| keyword | line |
|---|---|
| `Kicker`, `Flashback`, `Miracle`, `Ward` with a `Cost` | `K:<Name>:<costText(Cost)>` |
| `Protection` with a `PColor` filter of one colour | `K:Protection from <colour, lower case>` |
| any other | `keyword:<Name>` |

**A spell's keyword.** CC-22a's `spell.go` refuses a spell ability whose `Keyword` is set. Change it: a `Kicker`
keyword there prints its `K:` line as above, then the spell; any other keyword keeps the refusal
(`spell:keyword`).

## Implementation steps

1. `ability_activated.go`, `ability_triggered.go`, `ability_static.go`, `ability_replacement.go`,
   `step_replace.go`, `keyword_args.go`; the `spell.go` change.
2. Update `TestPrintSkeleton`'s triggered-ability and `Ward`-with-cost cases, which now print; keep a split layout
   and a refused face as the unprintable cases, and add an `event:<kind>` case.
3. Run `go run ./cmd/cardc compile -slice fdn` and `-slice legacy`; record the served totals.

## Tests

In `engine/cardc/gorgebe`, hand-built IR, asserting parameter lists:
- `TestActivated`: a mana ability (`Cost$ T`), a sacrifice-cost damage ability with a target, sorcery speed, once,
  a graveyard zone, a raid restriction (`CheckSVar$`), a loyalty ability, and the refusals.
- `TestTriggerParams`: one case per event row of section 2, the `or` case (`Secondary$ True`, one shared
  `Execute$`), each trailing rule, and the refusals.
- `TestStatics`: one case per row of section 3, with and without a threshold condition, and the present-form
  condition on the source.
- `TestReplacements`: the tapped land, the pay-life-or-tapped land, the `etbCounter` line with `X`, the damage
  doubler (`ReplaceCount$DamageAmount/Times.2`), and the refusals.
- `TestKeywordArgs`: each row of section 5 and the kicked spell.
- `TestServedCardsMatchForge` and `TestServedAPIsSupported` (CC-22a) pass with the floor raised to the served count
  you measure. A served card whose shape differs is fixed by an `Unprintable{What: "shape:<construct>"}` for the
  construct responsible, never by loosening the comparison; list each one in your report.
- `TestModesAttested` (corpus; `testutil.CorpusRegistry(t)`, ENV when absent): every trigger `Mode$`, static `Mode$`
  and replacement `Event$` this ticket emits occurs in the corpus, and each `Phase$` value is attested.

## Done means

- [ ] `make test-one PKG=./engine/cardc/gorgebe RUN='^(TestActivated|TestTriggerParams|TestStatics|TestReplacements|TestKeywordArgs|TestServedCardsMatchForge|TestServedAPIsSupported|TestModesAttested)$'` passes (not ENV).
- [ ] `make test-one PKG=./engine/cardc/gorgebe` passes.
- [ ] `make test-one PKG=./cmd/cardc` passes.
- [ ] `make test-one PKG=./internal/archtest RUN='^(TestDependencyOrderHolds|TestNothingImportsCardc|TestPublicBoundary|TestNoVerbatimForgeLines)$'` passes.
- [ ] The report gives the served count before and after, and the ten most common refusal reasons from
  `cardc compile`.
- [ ] No `make test-targeted` or `make test-smart` per ticket: one `make test-smart` runs at the wave-3 head.

## Out of scope

- Delayed and reflexive triggers (`next-end-step`, `reflexive`), copy and token-copy effects, additional costs, CDAs.
- Lockstep (CC-31 exists; running it over the newly served cards is CC-32's). The static diff (CC-30).
- Any edit to `engine/cardc/ir`, `engine/cardc/mtgish`, `engine/cards` or an engine package. If the IR cannot carry
  something this brief needs, stop and report it: CC-15a owns IR edits (PLAN.md Decisions log).
- Committing generated script text, Oracle text, or fixtures copied from Forge or mtgish data.
- Anything on GitHub outside gorge. Never open a pull request to i5jb/mtgish.

## Collisions

Creates `ability_*.go`, `step_replace.go`, `keyword_args.go` and their tests; edits `spell.go` and
`gorgebe_test.go`; may edit `emit.go`. Runs alone, after CC-22a and CC-22b.
