# feat(cardc): CC-22b gorge backend zone moves, library digs, pump and animate, unless-costs and the cost printer

Kind: feature

Target branch: `cardc`. Complexity: medium. Implementer: Sonnet.
Design of record: `docs/design/2026-11-23-typed-script-values/PLAN.md` on `cardc`, sections 1.3 and 6. Runs after
CC-21a and CC-21b (on `cardc`) and CC-15a/b. Runs in parallel with CC-22a.

## Goal

1. Register the step printers for zone moves, searches, library looks and reveals, hand reveals, shuffles, tapping,
   sacrifice, `apply-effect` inside a program (pump and animate), and the pay-cost "unless" idiom.
2. Add the cost printer (`Cost$` / `UnlessCost$` text), which CC-23 reuses for activated abilities and keyword costs.

## Workspace facts (verified at `35902648b9`, plus wave-3's CC-21a/b)

- `emit.go` (CC-21a/b) holds `pr`, `node`, `p.program`, `p.operand`, `registerStep` and the family hooks. Read it
  first. This ticket must not edit it (CC-22a may).
- `manaCost` (`engine/cardc/gorgebe/print.go:144`) already prints a mana symbol list in gorge's spelling.
- IR shapes: `ir.Step` `engine/cardc/ir/program.go:182`, `ir.Cost`/`ir.CostPart` `cost.go:454,462`, `ir.Effect`
  `effect.go:108`, `ir.Enter` `effect.go:128`, `Duration` `program.go:132-143`.
- On the 149 refusal-free proof-slice cards at this head, the verbs this ticket owns occur: `move` 15, `exile` 8,
  `apply-effect` 6 inside programs (the other 14 are static abilities, CC-23), `pay-cost` 5, `choose` 3, `look` 3,
  `shuffle` 3, `search` 2, and one each of `look-at-hand` and `reveal`. Cost parts on those cards: `tap-self` 24,
  `sacrifice` 6, `discard` 4, `pay-life` 3, `remove-counters` 3, `exile` 2.
- The shapes, measured:
  - search: `search{choose(card in library, 1); move(receipt found) to hand; shuffle}`;
  - look: `look(top N){move optional (choose 1) to Z1 [word]; move to Z2 [word]}`, and
    `look(top 3){move to library top-any-order}` followed by an optional `shuffle`;
  - reveal: `reveal(top X){move optional (choose any) to battlefield; move to graveyard}`;
  - unless: `pay-cost optional [who]` then `if(not cost-paid){one step}`, or `if(cost-paid)`.
- `TestNoVerbatimForgeLines` (`internal/archtest/public_boundary_test.go:498`) fails on any whole corpus script line
  of 30 or more characters in a committed file. Assert on nodes (`API` plus `[]kv`) and on cost strings.

## Design

Decided. The seat makes no design calls; anything this brief does not cover is `Unprintable` (`verb:<verb>`,
`effect:<kind>`, `cost:<kind>`).

Common rules, as CC-22a: amounts are `p.amount(...)`; `st.Optional` with a nil `Who` adds `OptionalDecider$ You` and
with a `Who` is `optional:<verb>`; a player-acting `Who` goes on with `p.operand(n, "Defined", st.Who)`. Zone names
print capitalised: `Battlefield Hand Library Graveyard Exile Stack Command`.

### 1. Moves (`step_move.go`): `VerbMove` and `VerbExile`

`VerbExile` is `VerbMove` with `To` exile. By the Obj:

| Obj | API and params |
|---|---|
| `RefTarget` | `ChangeZone`, `Origin$ <From, else the target filter's single top-level zone, else Battlefield>`, `Destination$ <To>`, target declared by `p.operand(n, "Defined", Obj)` |
| `RefSelf` | `ChangeZone`, `Defined$ Self`, `Origin$ <From>` only when `From` is set, `Destination$ <To>` |
| `RefChoose` with `Of` nil and a filter of exactly one zone Z | `ChangeZone`, `Origin$ Z`, `Destination$ <To>`, `ChangeType$ <valid>`, `ChangeNum$ <Obj.N>`, `Mandatory$ True` |
| `RefAll` with a filter of exactly one zone Z | `ChangeZoneAll`, `ChangeType$ <valid>`, `Origin$ Z`, `Destination$ <To>` |
| `RefTopOfLibrary` (exile only) | `Mill`, `NumCards$ <Obj.N>`, `Destination$ Exile`; the player from `Obj.Of`, else `Who`, through `p.operand(n, "Defined", ...)` |

Then, on that node:
- `Word`: `top` → `LibraryPosition$ 0`; `bottom` or `bottom-any-order` → `LibraryPosition$ -1`; `top-any-order` →
  `Reorder$ True`; any other word is `verb:move-word`.
- `Enter`: `Tapped` → `Tapped$ True`; `UnderOwner` prints nothing (owner's control is the default); `Controller`,
  `Counter` or `Effects` is `verb:move-enter`.
- An Obj not in the table (`RefReceipt` outside the idioms below, `RefChoose` with a chooser) is `verb:move`.

### 2. Searches, looks and reveals (`step_dig.go`)

**`VerbSearch`.** `Then` is exactly `[choose(RefChoose of a library filter, N), move(RefReceipt found) to Z,
shuffle]`, none optional: `ChangeZone`, `Origin$ Library`, `Destination$ Z`, `ChangeType$ <valid>`,
`ChangeNum$ <N>`, `Mandatory$ True`, plus `Tapped$ True` when the move's `Enter.Tapped`. Any other shape is
`verb:search`.

**`VerbLook` and `VerbReveal`**, Obj `RefTopOfLibrary` with `N`:
- **Dig.** `Then` is `[move m1 (Obj RefChoose, To Z1), move m2 (no Obj, To Z2)]`: `Dig`, `DigNum$ <N>`,
  `ChangeNum$ <m1.Obj.N, or Any when nil>`, `ChangeValid$ <valid(m1.Obj.Filter), or Card when nil>`,
  `DestinationZone$ Z1`, `DestinationZone2$ Z2`; then `Optional$ True` when m1 is optional; `Reveal$ True` for
  `VerbReveal` or when m1's word is `reveal`; `LibraryPosition$ 0` when Z1 is library with word `top`;
  `LibraryPosition2$ -1` when Z2 is library with word `bottom-any-order` (or `bottom`). Any other word is
  `verb:dig-word`.
- **Rearrange** (`VerbLook` only). `Then` is `[move (no Obj) to library top-any-order]`: `RearrangeTopOfLibrary`,
  `NumCards$ <N>`. When the next outer step is an optional `VerbShuffle` with no `Who`, add `MayShuffle$ True` and
  consume it too (2 steps).
- Any other shape is `verb:look` / `verb:reveal`.

**`VerbLookAtHand`**: `RevealHand`, `Look$ True`, `p.operand(n, "Defined", Obj)`. **`VerbRevealHand`**: `RevealHand`
with the same operand.

**`VerbChoose`** registers a printer that always returns `verb:choose`: a choice prints only inside the idioms above.

**`VerbShuffle`** (alone): `Shuffle`, Who. Optional is `verb:shuffle-optional`.

### 3. Tap, untap, sacrifice (`step_permanent.go`)

| verb, Obj | API and params |
|---|---|
| `VerbTap` / `VerbUntap`, `RefAll` over objects | `TapAll` / `UntapAll`, `ValidCards$ <valid>` |
| `VerbTap` / `VerbUntap`, other | `Tap` / `Untap`, `p.operand(n, "Defined", Obj)` |
| `VerbSacrifice`, `RefSelf` | `Sacrifice`, `Defined$ Self` |
| `VerbSacrifice`, `RefChoose` with `Of` nil | `Sacrifice`, `SacValid$ <valid>`, `Amount$ <Obj.N>`, Who |

### 4. Apply-effect inside a program (`step_effect.go`)

Obj `RefSelf` prints `Defined$ Self`; a target goes through `p.operand(n, "Defined", Obj)`; any other Obj is
`effect:obj`. `Dur`: `end-of-turn` prints nothing, `permanent` → `Duration$ Permanent`, `until-your-next-turn` →
`Duration$ UntilYourNextTurn`, `end-of-combat` → `Duration$ UntilEndOfCombat`; any other is `effect:dur-<name>`.

- **Pump**, when every effect is `EffAdjustPT` or an `EffAddAbility` of bare keywords: `Pump`; `NumAtt$` and
  `NumDef$` from the summed `P` and `T`, printed signed (`+2`, `-1`, `+X`) and omitted when the sum is the constant
  0; `KW$ <K1> & <K2>` in order.
- **Animate**, when every effect is one of `EffSetPT`, `EffAddCardType`, `EffAddSubtype`, `EffSetSubtype`,
  `EffSetColor`, `EffAddAbility` (bare keywords) and `EffAllCreatureTypes`, and at least one is not a pump effect:
  `Animate`, with `Power$ <P>` and `Toughness$ <T>` (set-pt); `Types$ <added card types, then subtypes, each
  Respell'd, comma-joined>`; `RemoveCreatureTypes$ True` (set-subtype, whose words also go in `Types$`);
  `Colors$ <colour names, comma-joined, WUBRG order>` and `OverwriteColors$ True` (set-color); `Keywords$ <K1> & <K2>`;
  `AddAllCreatureTypes$ True` (all-creature-types).
- Anything else is `effect:<first kind outside the sets>`.

### 5. Unless-costs (`step_unless.go`): `VerbPayCost`

`steps[0]` is an optional `pay-cost`; `steps[1]` is a `VerbIf` with no `Else` whose `Cond` is `CNot(CCostPaid)` or
`CCostPaid`. Print `t := p.program(steps[1].Then)`; it must be a single node, else `verb:pay-cost-chain`. Then set,
on `t`:
- `UnlessCost$ <costText(steps[0].Cost)>`;
- `UnlessPayer$ <defined(Who)>`, or `UnlessPayer$ You` when `Who` is nil (the default payer is not the controller);
- `UnlessSwitched$ True` when the condition is `CCostPaid` (paying makes it happen).

Consume 2 steps. Any other shape is `verb:pay-cost`. Print `t` first, then compute the payer, so a payer naming the
target (`TargetedController`) follows the node that declares it.

### 6. Costs (`cost.go`)

`func (p *pr) costText(c *ir.Cost) (string, error)`: the mana symbols through `manaCost` when `HasMana`, then each
part in IR order, space-joined. `n` is `p.amount(part.N)` (literal or `X`), or 1 when `N` is nil.

| part | text |
|---|---|
| `CostTapSelf` / `CostUntapSelf` | `T` / `Q` |
| `CostSacrifice`, Obj `RefSelf` | `Sac<1/CARDNAME>` |
| `CostSacrifice`, Obj `RefChoose` (`Of` nil or you) | `Sac<n/<valid>>`, n from `Obj.N` |
| `CostDiscard`, Obj `RefChoose` of a hand filter | `Discard<n/<valid>>` |
| `CostDiscard`, Obj `RefSelf` | `Discard<1/CARDNAME>` |
| `CostPayLife` | `PayLife<n>` |
| `CostExile`, Obj `RefSelf` | `Exile<1/CARDNAME>` |
| `CostExile`, Obj `RefChoose` of a hand filter / a graveyard filter | `ExileFromHand<n/<valid>>` / `ExileFromGrave<n/<valid>>` |
| `CostRemoveCounters`, Obj `RefSelf` | `SubCounter<n/<Word>>` |
| `CostReturnToHand`, Obj `RefSelf` | `Return<1/CARDNAME>` |
| `CostTapUntapped`, Obj `RefChoose` | `tapXType<n/<valid>>` |
| `CostReveal`, Obj `RefChoose` of a hand filter | `Reveal<n/<valid>>` |
| `CostMill` | `Mill<n>` |
| `CostLoyalty` | `AddCounter<n/LOYALTY>` for n ≥ 0, `SubCounter<-n/LOYALTY>` for n < 0 (`N` must be a constant) |
| anything else | `cost:<kind>` |

An empty cost (`HasMana` false, no parts) is `cost:empty`.

## Implementation steps

1. `cost.go`, then `step_move.go`, `step_dig.go`, `step_permanent.go`, `step_effect.go`, `step_unless.go`, each
   registering from `init`.
2. Tests (below).

## Tests

In `engine/cardc/gorgebe`, hand-built IR, asserting `API` plus `Params`:
- `TestMoveNodes`: every row and word and `Enter` rule of section 1, and the refusals.
- `TestDigNodes`: the search idiom; both measured dig shapes (a reveal-to-hand with bottom-any-order, a top-or-
  graveyard) and the reveal-any-to-battlefield shape; rearrange with and without the shuffle; and the refusals.
- `TestPermanentNodes`: section 3.
- `TestEffectNodes`: a pump of `+1/+0`, a pump with a keyword, an animate with P/T, types, colours and keywords,
  set-subtype, all-creature-types, each duration, and the refusals.
- `TestUnless`: the counter-unless shape with a target payer (`TargetedController` after the target declaration),
  the you-pay shape with a nil `Who` (`UnlessPayer$ You`), the switched shape, a two-node `Then`, and an orphan
  pay-cost.
- `TestCostText`: every row of section 6, a mana-plus-parts cost (`1 T Sac<1/CARDNAME>`), and the refusals.
- `TestCostsAttested` (corpus; `testutil.CorpusRegistry(t)`, ENV when absent): each cost-part head this printer emits
  (`Sac<`, `Discard<`, `PayLife<`, `Exile<`, `ExileFromHand<`, `ExileFromGrave<`, `SubCounter<`, `Return<`,
  `tapXType<`, `Reveal<`, `Mill<`, `AddCounter<`) occurs in some corpus ability's `Cost` parameter, and each API this
  ticket emits is in `effects.Supported()` as `api:<API>`.

## Done means

- [ ] `make test-one PKG=./engine/cardc/gorgebe RUN='^(TestMoveNodes|TestDigNodes|TestPermanentNodes|TestEffectNodes|TestUnless|TestCostText|TestCostsAttested)$'` passes (not ENV).
- [ ] `make test-one PKG=./engine/cardc/gorgebe` passes.
- [ ] `make test-one PKG=./internal/archtest RUN='^(TestDependencyOrderHolds|TestNothingImportsCardc|TestPublicBoundary|TestNoVerbatimForgeLines)$'` passes.
- [ ] If CC-22a lands first, rebase onto it and the whole package still passes, including
  `TestServedCardsMatchForge` with its floor raised to the new served count. Report both counts.
- [ ] No `make test-targeted` or `make test-smart` per ticket: one `make test-smart` runs at the wave-3 head.

## Out of scope

- `VerbDelayedTrigger`, `VerbCopySpell`, `VerbCreateTokenCopy`, `VerbAttach`, `VerbFight`, `VerbGainControl`,
  `VerbTransform`: leave them unregistered.
- `modify-enter` and `set-amount` (replacement bodies, CC-23). Static abilities (CC-23).
- Editing `emit.go`, `print.go` or `registry.go` (CC-22a owns those in this wave).
- Any edit to `engine/cardc/ir`, `engine/cardc/mtgish`, `engine/cards` or an engine package. If the IR cannot carry
  something this brief needs, stop and report it: CC-15a owns IR edits (PLAN.md Decisions log).
- Committing generated script text, Oracle text, or fixtures copied from Forge or mtgish data.
- Anything on GitHub outside gorge. Never open a pull request to i5jb/mtgish.

## Collisions

Creates only new files in `engine/cardc/gorgebe`: `cost.go`, `step_move.go`, `step_dig.go`, `step_permanent.go`,
`step_effect.go`, `step_unless.go` and their tests. CC-22a registers different verbs and edits `print.go`,
`registry.go` and `gorgebe_test.go`. Whichever finishes second rebases onto the other.
