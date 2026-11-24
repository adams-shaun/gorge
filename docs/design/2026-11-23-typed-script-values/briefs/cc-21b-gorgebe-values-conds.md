# feat(cardc): CC-21b gorge backend printers for values and conditions

Kind: feature

Target branch: `cardc`. Complexity: medium. Implementer: Sonnet.
Design of record: `docs/design/2026-11-23-typed-script-values/PLAN.md` on `cardc`, sections 1.3 and 6 ("What each
backend ticket checks"). Runs after CC-20 and CC-13/14 (merged). Runs in parallel with CC-21a.

## Goal

Print `ir.Value` as a gorge count expression (an SVar body) or a literal, and `ir.Cond` as a `condForm`, in the
spellings gorge already reads. Both install into the printer core, `emit.go`, which this ticket creates byte for
byte from `briefs/cc-21-emit.go.txt`, as CC-21a does.

No card prints differently after this ticket: nothing calls the printers from `Print` until CC-22a.

## Workspace facts (verified at `35902648b9`)

- IR shapes: `ir.Value` `engine/cardc/ir/value.go:406` (kinds `value.go:334-360`), `ir.Cond` `cond.go:481` (kinds
  `cond.go:427-444`), `ir.Compare` and `CmpOp` `zone.go`.
- On the proof slice (412 cards), 149 compile with no refusal at this head. Inside them:
  - Values: `const` 143, `x` 7, `event-amount` 4, and one each of `count`, `counters-among`,
    `life-gained-this-turn`, `mul`, `receipt-amount`.
  - Conditions: `ref-matches` 17, `cost-paid` 5, `not` 5, `is-turn-of` 2, `exists-player` 1, `kicked` 1.
  - The `ref-matches` shapes: you attacked this turn (raid, 8); seven or more cards in your graveyard (threshold, 6);
    no cards in your hand (1); the source is untapped (1); target player's life is exactly 10 (1).
- gorge classifies an SVar body's head with `cards.ValueHead` (`engine/cards/value_heads.go:37`) and models the heads
  `effects.Supported()` reports as `count:<head>` (`engine/effects/registry.go:692`, list
  `engine/effects/value_heads.go:20`). Bodies the classifier does not cover (`TriggerCount$...`, `ReplaceCount$...`,
  `Targeted$...`, `TargetedPlayer$...`) are proved by corpus attestation instead.
- gorge reads ability conditions as `ConditionCheckSVar$`/`ConditionSVarCompare$`, `ConditionDefined$`/
  `ConditionPresent$`/`ConditionCompare$` and `ConditionPlayerTurn$` (`engine/effects/activation_params.go:443-450`),
  and trigger, static and activation gates as `CheckSVar$`/`SVarCompare$`, `IsPresent$`/`PresentCompare$`/
  `PresentDefined$` and `PlayerTurn$`/`OpponentTurn$` (`activation_params.go:395-439`). `condForm.onAbility` and
  `condForm.onTrigger` in `emit.go` render those; this ticket only fills `condForm`.
- `TestNoVerbatimForgeLines` (`internal/archtest/public_boundary_test.go:498`) fails on any whole corpus script line
  of 30 or more characters in a committed file. Test expectations are single values and `condForm`s.

## Design

Decided. The seat makes no design calls; anything this brief does not cover is `Unprintable`.

### 1. The core (`emit.go`)

`cp docs/design/2026-11-23-typed-script-values/briefs/cc-21-emit.go.txt engine/cardc/gorgebe/emit.go`. Do not edit
it. What this ticket uses: the hooks `valueFn` and `condFn` (set from `init` in `value.go` and `cond.go`); `p.amount`
(prints `VConst` itself and turns any non-literal body into an SVar name via `countVar`); `p.ev` (the enclosing
trigger's or replacement's event); `p.frame()`; `p.valid` and `p.defined` (CC-21a's hooks; until CC-21a lands they
return `Unprintable`, see Tests); `condForm` with `not()`.

Refusals are `Unprintable{What: "value:<kind>"}` or `"cond:<kind>"`, `Path` the node's `Src`.

### 2. Values (`value.go`): `valueFn`

`valueFn` returns an SVar body, or a decimal literal for constant folds. `VConst` never reaches it.

| IR | body |
|---|---|
| `VX` | `Count$xPaid` |
| `VAdd` of two constants, `VSub`, `VMul` likewise | the folded decimal |
| `VAdd(a, const k)` or `VAdd(const k, a)` | body(a) + `/Plus.<k>` |
| `VSub(a, const k)` | body(a) + `/Minus.<k>` |
| `VMul(a, const k)` or `VMul(const k, a)` | body(a) + `/Times.<k>` |
| `VHalfDown(a)`, `VHalfUp(a)` | body(a) + `/HalfDown`, `/HalfUp` |
| any of the four above where body(a) already holds `/` or is a literal it cannot fold | `value:<kind>-nested` |
| `VCount(f)` | `Count$Valid<Z> <valid(f)>`, where `<Z>` is empty when `f.Zones` is empty or battlefield only, else the zone names capitalised and comma-joined in `Zone` order (`Count$ValidGraveyard Card.namedX`) |
| `VCountersAmong(f, w)`, f is `PIs` of `RefSelf` | `Count$CardCounters.<w>` |
| `VPower`, `VToughness`, `VManaValue` of `RefSelf` | `Count$CardPower`, `Count$CardToughness`, `Count$CardManaCost` |
| the same of `RefTarget` | `Targeted$CardPower`, `Targeted$CardToughness`, `Targeted$CardManaCost` |
| `VLife` of you | `Count$YourLifeTotal` |
| `VLife` of a player target | `TargetedPlayer$LifeTotal` |
| `VStartingLife` of you | `Count$YourStartingLife` |
| `VLifeGainedThisTurn` of you | `Count$LifeYouGainedThisTurn` |
| `VDevotion`, one colour | `Count$Devotion.<Colour>` (`White Blue Black Red Green`) |
| `VDevotion`, two colours | `Count$DevotionDual.<Colour1>.<Colour2>`, in WUBRG order |
| `VChosenNumber` | `Count$ChosenNumber` |
| `VEventAmount`, `p.ev` is `EvGainsLife` or `EvLosesLife` | `TriggerCount$LifeAmount` |
| `VEventAmount`, `p.ev` is `EvDealsCombatDamage`, `EvDealsCombatDamageToPlayer` or `EvDealsNoncombatDamageToPlayer` | `TriggerCount$DamageAmount` |
| `VEventAmount`, `p.ev` is `EvWouldDealDamage` | `ReplaceCount$DamageAmount` |
| `VEventAmount`, `p.ev` is `EvCountersPut` | `TriggerCount$Amount` |
| any other kind, operand or event | `value:<kind>` (event amounts: `value:event-amount@<event>`) |

"Of you" means `RefYou`, or `RefAll` of `PPlayerIs` `RefYou`. "A player target" means `RefTarget` whose frame slot's
filter subject is `SubjPlayer`. `VReceiptAmount` stays `value:receipt-amount`: Exsanguinate's "life lost this way"
needs Forge's remembered-amount idiom, which is out of scope.

### 3. Conditions (`cond.go`): `condFn`

`n` is `p.amount(...)`; `CMP` is the `CmpOp` upper-cased. "Of you" as in section 2.

| IR | `condForm` |
|---|---|
| `CRefMatches`, Ref of you, Filter `PAttackedThisTurn` | `SVar: countVar("Count$AttackersDeclared")`, `Cmp: GE1` |
| `CRefMatches`, Ref of you, Filter `PCardsInGraveyard` cmp v | `SVar: countVar("Count$ValidGraveyard Card.YouOwn")`, `Cmp: <CMP><n>` |
| `CRefMatches`, Ref of you, Filter `PCardsInHand` cmp v | `SVar: countVar("Count$CardsInYourHand")`, `Cmp: <CMP><n>` |
| `CRefMatches`, Ref of you, Filter `PLifeTotal` cmp v | `SVar: countVar("Count$YourLifeTotal")`, `Cmp: <CMP><n>` |
| `CRefMatches`, Ref a player target, Filter `PLifeTotal` cmp v | `SVar: countVar("TargetedPlayer$LifeTotal")`, `Cmp: <CMP><n>` |
| `CRefMatches`, Ref an object (any non-player ref `p.defined` prints), any other Filter | `Present: valid(Filter)`, `Defined: defined(Ref)` (`""` when it is `Self`), `Cmp: GE1` |
| `CExistsPlayer`, Filter `FAll`(opponent of you, `PLifeTotal` cmp v), cmp `LE` or `LT` | `SVar: countVar("PlayerCountOpponents$LowestLifeTotal")`, `Cmp: <CMP><n>` |
| the same with cmp `GE` or `GT` | `PlayerCountOpponents$HighestLifeTotal` |
| `CNot(c)` | `cond(c)`, then `.not()` |
| `CKicked` | `SVar: countVar("Count$Kicked.1.0")`, `Cmp: GE1` |
| `CIsTurnOf`, Ref of you | `Turn: You` |
| `CIsTurnOf`, Filter opponent of you | `Turn: Opponent` |
| `CCompare(L, R)` with L not constant | `SVar: amount(L)` (an SVar name), `Cmp: <CMP><amount(R)>` |
| `CCountCompare(Filter, Cmp, R)` | `SVar: countVar` of `VCount(Filter)`'s body, `Cmp: <CMP><amount(R)>` |
| `CCostPaid` | `cond:cost-paid` (CC-22b's pay-cost printer consumes it before any condition prints) |
| any other kind or shape | `cond:<kind>` |

The structural cases read the Filter's own fields (`Pred`, `Cmp`, `Value`, the `FAll` arguments); they do not print
it. Match a filter shape exactly, with no extra predicates.

## Implementation steps

1. Copy `emit.go`. Write `value.go` and `cond.go`, each installing its hook from `init`.
2. Tests (below). No other file changes.

## Tests

In `engine/cardc/gorgebe`, hand-built IR:
- `TestValueBodies`: one case per table row of section 2 that needs no filter or ref printer (constants, X, the
  arithmetic suffixes and folds, the nested refusal, counters on self, the self and you rows, devotion, chosen
  number, the four event-amount rows, and the refusal rows). Assert the body, and for `p.amount` the SVar name and
  line (`X` for `Count$xPaid`, `N` then `N2` for others, the same name for a repeated body).
- `TestCondForms`: one case per table row of section 3 that needs no printer from CC-21a (raid, threshold, hellbent,
  life of you, the targeted-life row with a frame pushed, the two opponent-life rows, `not` of each SVar row and of
  both turn rows, kicked, both turn rows, `CCompare`, the cost-paid and unknown refusals).
- `TestCondRenderings`: `onAbility` and `onTrigger` of an SVar form, a present form with `Defined` empty (the
  trigger form's `+Self` / `.Self` restriction) and with `Defined: Targeted`, and both turn forms (the ability form
  of an opponent's turn is an `Unprintable`).
- `TestValueHeadsModelled` (corpus is not needed). For every body section 2 and 3 emit that `cards.ValueHead`
  classifies, `effects.Supported()["count:"+head]` is true. Name each unmodelled head in the failure.
- `TestBodiesAttested` (corpus; `testutil.CorpusRegistry(t)`, ENV when absent). Every body that `cards.ValueHead`
  does not classify (`TriggerCount$...`, `ReplaceCount$...`, `Targeted$...`, `TargetedPlayer$...`) is the exact body
  of some corpus SVar (each face's `SVars` values). Helper names start with `corpusSVar`.

The rows that print a filter or a reference (`VCount`, the present form of `CRefMatches`, `CCountCompare`) depend on
CC-21a's printers. Test them in CC-22a's `TestPrintersCompose`, not here.

## Done means

- [ ] `make test-one PKG=./engine/cardc/gorgebe RUN='^(TestValueBodies|TestCondForms|TestCondRenderings|TestValueHeadsModelled|TestBodiesAttested)$'` passes (not ENV).
- [ ] `make test-one PKG=./engine/cardc/gorgebe` passes (CC-20's tests unchanged).
- [ ] `make test-one PKG=./internal/archtest RUN='^(TestDependencyOrderHolds|TestNothingImportsCardc|TestPublicBoundary|TestNoVerbatimForgeLines)$'` passes.
- [ ] `diff docs/design/2026-11-23-typed-script-values/briefs/cc-21-emit.go.txt engine/cardc/gorgebe/emit.go` is empty.
- [ ] No `make test-targeted` or `make test-smart` per ticket: one `make test-smart` runs at the wave-3 head.

## Out of scope

- References, filters, targets (CC-21a). Steps, abilities, costs (CC-22a/b, CC-23).
- `VReceiptAmount` and any remembered-value idiom.
- Any edit to `engine/cardc/ir`, `engine/cardc/mtgish`, `engine/cards` or an engine package. If the IR cannot carry
  something this brief needs, stop and report it: CC-15a owns IR edits (PLAN.md Decisions log).
- Committing generated script text, Oracle text, or fixtures copied from Forge or mtgish data.
- Anything on GitHub outside gorge. Never open a pull request to i5jb/mtgish.

## Collisions

Creates `emit.go` (shared, byte for byte with CC-21a), `value.go`, `cond.go` and their tests in
`engine/cardc/gorgebe`. CC-21a creates `ref.go`, `filter.go`, `target.go` and the same `emit.go`. Test helper names
here start with `corpusSVar` / `valueTest`; CC-21a's start with `corpusParams` / `refTest`.
