# feat(cardc): CC-21a gorge backend printer core, references, filters and target declarations

Kind: feature

Target branch: `cardc`. Complexity: medium. Implementer: Sonnet.
Design of record: `docs/design/2026-11-23-typed-script-values/PLAN.md` on `cardc`, sections 1.3 and 6 ("What each
backend ticket checks"). Runs after CC-20 (`5eccca0078`) and CC-11/12 (merged). Runs in parallel with CC-21b.

## Goal

1. Add the printer core every wave-3 backend ticket prints into: `engine/cardc/gorgebe/emit.go`, copied byte for byte
   from `briefs/cc-21-emit.go.txt`.
2. Print `ir.Ref` as a `Defined$` value, `ir.Filter` as a Valid list, and `ir.Target` as target parameters, in the
   spellings gorge already reads.

No card prints differently after this ticket: nothing calls the new printers from `Print` until CC-22a. So
`TestServedCardsMatchForge` still serves 25.

## Workspace facts (verified at `35902648b9`)

- `print.go:95-109` prints only bare keywords; any other ability is `Unprintable{What: "ability:<kind>"}`
  (`print.go:98`). CC-22a replaces that loop.
- IR shapes: `ir.Ref` `engine/cardc/ir/ref.go:126`, `ir.Target` `ref.go:141`, `ir.Filter` `filter.go:314`, `Pred`
  list `filter.go:225-257`, `ir.Program.Targets` `program.go:175`. Target slots index the nearest enclosing Program's
  `Targets` (PLAN.md Decisions log, 11-24).
- On the proof slice (FDN pool plus the 12 Legacy decks, 412 cards), 149 cards compile with no refusal (measured with
  `mtgish.Compile` at this head). Inside those 149, the families this ticket prints occur as:
  - Ref kinds (14): `self` 118, `you` 62, `target` 30, `all` 20, `choose` 18, `event-object` 6, `top-of-library` 6,
    `iter-player` 5, `receipt` 5, `event-player` 3, `controller-of` 2, `player-or-controller-of` 2,
    `active-player` 1, `defending-player` 1.
  - Filter predicates (by subject): `permanent/is` 55, `permanent/card-type` 24, `player/player-is` 24, `player/any`
    19, `player/opponent-of` 17, `card/any` 15, `spell/any` 10, `player/attacked-this-turn` 8,
    `player/cards-in-graveyard` 6, `card/owned-by` 3, and 16 more kinds with one or two uses each.
  - A player operand is mostly spelled `(ref all :filter (filter pred player-is :subject player :ref (ref you)))`,
    not `(ref you)` (CC-12's lowering of `Players`).
- gorge reads these spellings through unexported code: `definedSpec` (`engine/effects/context.go:684`) and the
  compiled filter (`engine/effects/filter_compiled.go:117`). gorgebe cannot call either, so this ticket proves each
  spelling is **attested**: some Forge card in the corpus uses it in the same parameter family.
- `TestNoVerbatimForgeLines` (`internal/archtest/public_boundary_test.go:498`) fails when any committed file contains
  a whole corpus script line of 30 or more characters, anywhere in the file (`public_boundary_test.go:433`). Test
  expectations are therefore parameter lists and single values, never whole script lines.

## Design

Decided. The seat makes no design calls; anything this brief does not cover is `Unprintable`.

### 1. The core (`emit.go`)

`cp docs/design/2026-11-23-typed-script-values/briefs/cc-21-emit.go.txt engine/cardc/gorgebe/emit.go`. Do not edit
it: CC-21b creates the same file in parallel, and an identical add merges cleanly. Read it before writing code. What
this ticket uses:
- `pr`, the per-face printer; `p.ev` and `p.evSelf` carry the enclosing trigger's event (CC-23 sets them; tests set
  them by hand).
- The hooks `refFn`, `validFn` and `targetFn`. This ticket sets them from `init` in `ref.go`, `filter.go` and
  `target.go`. Callers go through `p.defined`, `p.valid` and `p.target`.
- `p.amount(v)` for numeric operands. It prints `VConst` itself; every other kind goes through CC-21b's `valueFn`, so
  until CC-21b lands only constants print.
- `p.frame()`, the innermost target frame. Its `targets[slot]` tells a target reference whether the slot is a player.
- `unprintable(what, path)`.

Every refusal is `Unprintable{What: "<family>:<kind>", Path: <node Src>}`, with family `ref`, `filter` or `pred`
(`pred:<pred name>`), or `target`.

### 2. References (`ref.go`): `refFn`

| IR | context | `Defined$` |
|---|---|---|
| `RefSelf` | any | `Self` |
| `RefYou` | any | `You` |
| `RefTarget` slot s | `p.frame().targets[s].Filter.Subject == SubjPlayer` | `TargetedPlayer` |
| `RefTarget` slot s | otherwise (object, or any target) | `Targeted` |
| `RefTarget` | no frame, or slot out of range | `ref:target-unbound` |
| `RefEventObject` | `p.evSelf` | `Self` |
| `RefEventObject` | `p.ev` is `EvEnters` | `TriggeredCard` |
| `RefEventObject` | `EvDies`, `EvPutIntoGraveyard`, `EvLeaves` | `TriggeredNewCardLKICopy` |
| `RefEventObject` | `EvAttacks` | `TriggeredAttacker` |
| `RefEventObject` | `EvDealsCombatDamage`, `EvDealsCombatDamageToPlayer`, `EvDealsNoncombatDamageToPlayer` | `TriggeredSourceLKICopy` |
| `RefEventObject` | `EvCast` | `TriggeredSpellAbility` |
| `RefEventObject` | `EvBecomesTarget` | `TriggeredSourceSA` |
| `RefEventObject` | `EvCountersPut` | `TriggeredCard` |
| `RefEventObject` | any `EvWould*` | `ReplacedCard` |
| `RefEventPlayer` | `EvGainsLife`, `EvLosesLife`, `EvUpkeep`, `EvEndStep` | `TriggeredPlayer` |
| `RefEventPlayer` | `EvDraws` | `TriggeredCardController` (CC-23 prints draw triggers as `Mode$ Drawn` over cards) |
| `RefEventPlayer` | `EvDealsCombatDamageToPlayer`, `EvDealsNoncombatDamageToPlayer` | `TriggeredTarget` |
| `RefEventPlayer` | `EvCast` | `TriggeredActivator` |
| `RefControllerOf` | `Of` is `RefTarget` | `TargetedController` |
| `RefControllerOf` | `Of` is `RefEventObject` and `p.ev` is `EvBecomesTarget` | `TriggeredSourceSAController` |
| `RefControllerOf` | `Of` is `RefEventObject` and `p.ev` is `EvCast` | `TriggeredActivator` |
| `RefControllerOf` | `Of` is `RefEventObject` and `p.ev` is `EvEnters`, `EvDies` or `EvPutIntoGraveyard` | `TriggeredCardController` |
| `RefControllerOf` | `Of` is `RefSelf` | `You` |
| `RefOwnerOf` | `Of` is `RefTarget` | `TargetedOwner` |
| `RefPlayerOrControllerOf` | `Of` is `RefTarget` | `TargetedOrController` |
| `RefAll` | `Filter.Subject == SubjPlayer` | the Valid list of `Filter` (section 3), with a lone `Opponent` written `Player.Opponent` |
| any other kind or context | | `ref:<kind>` (for an event ref, `ref:<kind>@<event>`) |

`RefChoose`, `RefTopOfLibrary`, `RefReceipt`, `RefIterPlayer`, `RefIterObject`, `RefUnion` and `RefAll` over objects
have no `Defined$` spelling: the step printers (CC-22a/b) read them as API parameters. Here they are `ref:<kind>`.

A **player helper** used by sections 2 and 3, `playerWord(r *ir.Ref) (string, bool)`:
- `RefYou`, or `RefAll` whose filter is `PPlayerIs` of `RefYou` → `You`;
- `RefAll` whose filter is `POpponentOf` of `RefYou` → `Opp`;
- `RefActivePlayer` → `Active`;
- `RefTarget` of a player slot → `TargetedPlayer`;
- anything else → false.

### 3. Filters (`filter.go`): `validFn`

A Valid list is comma-separated **terms** (OR). A term is a base word, then optionally `.` and `+`-joined qualifiers
(AND): `Creature.YouCtrl+nonToken`. `Zones` is never printed here; the caller places zones (`TgtZone$`, `Origin$`).

**Normalise** to a list of terms, each a base plus qualifiers:
- `FAny`: the union of its arguments' terms.
- `FAll`: the product of its arguments' term lists, each product term merging its parts (at most 8 terms, else
  `filter:too-wide`).
- `FNot` of a single `FPred`: one term holding that predicate's negated qualifier (table below). `FNot` of anything
  else is `filter:not`.
- `FPred`: one term from the predicate table.

**Merge a term's parts.** Each part contributes either a base word (a type) or a qualifier.
- The first type word becomes the base. Every later type word becomes a qualifier (`Creature.Artifact`).
- With no type word, the base is the subject's: `SubjPermanent` → `Permanent`; `SubjCard`, `SubjSpell` and
  `SubjObject` → `Card`; `SubjPlayer` → `Player`. Take the subject from the outermost filter whose `Subject` is set.
- Qualifiers keep the IR's order, without duplicates.
- A player term with exactly one qualifier, `You` or `Opponent`, prints that word alone (`You`, not `Player.You`).

**The any-target rule.** An `FAny` whose terms are exactly creature, planeswalker and battle permanents plus any
player, in any order, prints `Any`.

**Predicate table** (`n` is `p.amount(f.Value)`; `CMP` is `f.Cmp` upper-cased: `EQ NE LT LE GT GE`):

| `Pred` | contributes | negated (`FNot`) |
|---|---|---|
| `PAny` | nothing (the base alone) | `pred:any-not` |
| `PCardType` w | type word `Respell(w)` | `non<Respell(w)>` |
| `PSubtype` w | type word `Respell(w)` | `non<Respell(w)>` |
| `PSupertype` w | qualifier `Respell(w)` | `non<Respell(w)>` |
| `PPermanentCard` | type word `Permanent` | `nonPermanent` |
| `PColor`, one colour | `White`, `Blue`, `Black`, `Red` or `Green` | `non<Colour>` |
| `PColor`, several colours | `pred:color-multi` | `pred:color-multi` |
| `PColorless` | `Colorless` | `nonColorless` |
| `PMulticolored` | `MultiColor` | `nonMultiColor` |
| `PManaValue` | `cmc<CMP><n>` | `pred:mana-value-not` |
| `PPower`, `PToughness` | `power<CMP><n>`, `toughness<CMP><n>` | `pred:<name>-not` |
| `PToken` | `token` | `nonToken` |
| `PTapped` | `tapped` | `untapped` |
| `PAttacking`, `PBlocking` | `attacking`, `blocking` | `pred:<name>-not` |
| `PHasKeyword` w | `with<w>` | `without<w>` |
| `PCounters` w | `counters_<CMP><n>_<w>` | `pred:counters-not` |
| `PName` w | `named<w>` | `notnamed<w>` |
| `PControlledBy` r | `playerWord(r)` + `Ctrl`: `YouCtrl`, `OppCtrl`, `ActivePlayerCtrl`, `TargetedPlayerCtrl` | `pred:controlled-by-not` |
| `POwnedBy` r | `YouOwn`, `OppOwn`, `TargetedPlayerOwn` (same rule) | `pred:owned-by-not` |
| `PIs` r, r is `RefSelf`, or `RefEventObject` with `p.evSelf` | `Self` | `Other` |
| `PPlayerIs` r | `You`, `Active` or `TargetedPlayer` from `playerWord` (`Opp` is `pred:player-is`) | `!<word>` (`!Active`) |
| `POpponentOf` r, r is you | `Opponent` | `pred:opponent-of-not` |
| any other predicate, or an operand the table does not cover | `pred:<name>` | |

`PAttackedThisTurn`, `PCardsInGraveyard`, `PCardsInHand` and `PLifeTotal` are deliberately `pred:<name>` here: they
occur only inside conditions, and CC-21b reads them structurally.

### 4. Target declarations (`target.go`): `targetFn`

Parameters, in this order:
1. `ValidTgts$ <valid(Filter)>`.
2. `TargetType$ Spell` when the filter's subject is `SubjSpell`.
3. `TgtZone$ <zones>` when the filter's top-level `Zones` holds any zone other than battlefield and stack: the zone
   names capitalised (`Graveyard`, `Hand`, `Library`, `Exile`), comma-joined in `Zone` order.
4. `TargetMin$ <Min>` and `TargetMax$ <Max>` unless `Min == 1 && Max == 1`. With `MaxV` set, `TargetMax$` is
   `p.amount(MaxV)`.
5. `TargetUnique$ True` when `Distinct`.

No `TgtPrompt$`: prompts are display text.

## Implementation steps

1. Copy `emit.go`. Write `ref.go`, `filter.go`, `target.go`, each installing its hook from `init`.
2. Tests (below). No other file changes.

## Tests

In `engine/cardc/gorgebe`, all hand-built IR, asserting single values or `[]kv` lists:
- `TestDefinedTable`: one row per table line of section 2, including each refusal row; event rows set `p.ev` and
  `p.evSelf` by hand, target rows push a frame (`p.frames = append(p.frames, &frame{...})`).
- `TestValidTerms`: one case per predicate-table row and negation; `FAll` of a type and two qualifiers
  (`Creature.YouCtrl+nonToken`); `FAny` of two types (`Creature,Planeswalker`); the any-target rule; a two-by-two
  `FAll` of `FAny`s giving four terms; `filter:too-wide`; `filter:not` on a compound; the player forms `You`,
  `Opponent`, `Player`, `Player.!Active`; the controlled-by ref spelled `(ref all ... player-is you)`.
- `TestTargetParams`: Lightning Bolt's any target (`ValidTgts$ Any`), a spell target (`ValidTgts$ Card`,
  `TargetType$ Spell`), a graveyard card target with `TgtZone$ Graveyard`, min 0 max 1, a `MaxV` of `VConst`, and
  `Distinct`.
- `TestSpellingsAttested` (corpus; `testutil.CorpusRegistry(t)`, ENV when absent). Walk every corpus card face: each
  `SA` and its `Sub` chain, each trigger's `Params` and `Effect` chain, each static's and replacement's `Params` and
  the replacement's `With` chain. Collect:
  - the values of `Defined`, `DefinedPlayer`, `Affected`, `ValidPlayer`, `Activator`, `UnlessPayer` and
    `Controller`;
  - the qualifier tokens (each term split at the first `.`, then at `+`) and base words of every param whose key
    starts with `Valid` or is `ChangeType`, `ChangeValid`, `Affected`, `IsPresent`, `ConditionPresent`.

  Then assert every `Defined$` spelling of section 2's table is a collected value, and every qualifier and base the
  predicate table emits for the operands used in `TestValidTerms` is a collected token. Name each unattested
  spelling in the failure.
- `TestProofSliceRefsFilters` (needs `.cards/mtgish` and the Forge corpus; ENV when either is absent). Over the 149
  refusal-free proof-slice cards (reuse `sliceCards` from `gorgebe_test.go:242`, skipping cards with refusals), walk
  every `ir.Target` and every `ir.Filter` that is a `Target.Filter`, and print it through `p.target` with a frame
  pushed and `p.evSelf` false. Log the count printed and each `Unprintable.What` with its count. Assert at least 23
  of the 25 declarations print (measured: 8 any-target, 5 player, 5 spell, 2 opponent, and one each of two
  permanents, a permanent card you own in your graveyard, and up to one graveyard card; the other two use "another",
  which needs `p.evSelf`).

## Done means

- [ ] `make test-one PKG=./engine/cardc/gorgebe RUN='^(TestDefinedTable|TestValidTerms|TestTargetParams|TestSpellingsAttested|TestProofSliceRefsFilters)$'` passes (not ENV).
- [ ] `make test-one PKG=./engine/cardc/gorgebe` passes (CC-20's tests unchanged).
- [ ] `make test-one PKG=./internal/archtest RUN='^(TestDependencyOrderHolds|TestNothingImportsCardc|TestPublicBoundary|TestNoVerbatimForgeLines)$'` passes.
- [ ] `diff docs/design/2026-11-23-typed-script-values/briefs/cc-21-emit.go.txt engine/cardc/gorgebe/emit.go` is empty.
- [ ] No `make test-targeted` or `make test-smart` per ticket: one `make test-smart` runs at the wave-3 head.

## Out of scope

- Values and conditions (CC-21b). Step, ability and cost printing (CC-22a/b, CC-23).
- Any edit to `engine/cardc/ir`, `engine/cardc/mtgish`, `engine/cards` or any engine package. If the IR cannot carry
  something this brief needs, stop and report it: CC-15a owns IR edits (PLAN.md Decisions log).
- Committing generated script text, Oracle text, or fixtures copied from Forge or mtgish data.
- Anything on GitHub outside gorge. Never open a pull request to i5jb/mtgish.

## Collisions

Creates `emit.go` (shared, byte for byte with CC-21b), `ref.go`, `filter.go`, `target.go` and their tests in
`engine/cardc/gorgebe`. CC-21b creates `value.go`, `cond.go` and `emit.go`. Test helper names here start with
`corpusParams` / `refTest`; CC-21b's start with `corpusSVar` / `valueTest`, so the two test files never declare the
same name.
