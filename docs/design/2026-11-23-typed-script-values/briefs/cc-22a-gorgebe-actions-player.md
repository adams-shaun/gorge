# feat(cardc): CC-22a gorge backend spells and player, damage, mana and token actions

Kind: feature

Target branch: `cardc`. Complexity: medium. Implementer: Sonnet.
Design of record: `docs/design/2026-11-23-typed-script-values/PLAN.md` on `cardc`, sections 1.3 and 6. Runs after
CC-21a and CC-21b (`emit.go`, `ref.go`, `filter.go`, `target.go`, `value.go`, `cond.go` on `cardc`) and CC-15a/b.
Runs in parallel with CC-22b.

## Goal

1. Wire the printer core into `Print`, so a face prints every ability a registered printer covers.
2. Print **spell** abilities (`A:SP$ ...`).
3. Register the step printers for the player, damage, mana, counter-spell, token and control-flow verbs below.
4. Make `TestServedCardsMatchForge` compare each served card's whole ability shape with its Forge twin, and ratchet
   the served count.

## Workspace facts (verified at `35902648b9`, plus wave-3's CC-21a/b)

- `print.go:95-109` prints bare keywords and refuses any other ability (`print.go:98`); `Print` is `print.go:17`;
  `Build` calls `Print(c, oracle)` at `registry.go:67`.
- `emit.go` (CC-21a/b) holds `pr`, `node`, `p.program`, `p.operand`, `p.render`, `registerStep`, `registerAbility`
  and the family hooks. Read it first. `pr.tokens` exists but nothing sets it yet.
- Forge's token scripts are `cards.Registry.Tokens` (`engine/cards/registry.go:23-28`), keyed by the name a
  `TokenScript$` parameter references.
- `TestServedCardsMatchForge` (`gorgebe_test.go:328`) compares keywords in order, per-kind counts and each
  `Abilities[i].API`, with a floor of 25 served (`gorgebe_test.go:335`).
- `effects.Supported()` (`engine/effects/registry.go:692`) reports each implemented API as `api:<API>`.
- On the 149 refusal-free proof-slice cards, the verbs this ticket owns occur: `add-mana` 25, `draw` 22,
  `create-token` 19, `gain-life` 14, `deal-damage` 11, `lose-life` 11, `if` 10, `put-counters` 8, `counter` 7,
  `discard` 6, `surveil` 6, `for-each-player` 5, and one each of `mill`, `regenerate`, `end-turn`, `extra-turn`.
- `TestNoVerbatimForgeLines` (`internal/archtest/public_boundary_test.go:498`) fails on any whole corpus script line
  of 30 or more characters in a committed file. Assert on nodes (`API` plus `[]kv`), not on printed lines.

## Design

Decided. The seat makes no design calls; anything this brief does not cover is `Unprintable`.

### 1. Wiring (`print.go`, `registry.go`)

- Add `func PrintWith(c *ir.Card, oracle []string, tokens map[string]*cards.Card) ([]byte, error)`. `Print(c,
  oracle)` is `PrintWith(c, oracle, nil)`. `Build` calls `PrintWith(c, oracle, forge.Tokens)`.
- In `printFace` with abilities on, create `p := &pr{tokens: tokens}`. For each ability, in order:
  - a bare keyword (`AbKeyword`, `N`, `Cost` and `Filter` all nil) prints `K:<Name>` as today;
  - otherwise `abilityFns[a.Kind]` prints it; with no printer, keep today's refusals (`keyword:<Name>` for a
    keyword with arguments, `ability:<kind>` otherwise).
- Face output order: the CC-20 lines up to `Colors:`, then every `K:` line, then `p.lines`, then one
  `SVar:<K>:<V>` line per `p.svars` entry in order, then `Oracle:`.

### 2. Spells (`spell.go`)

`registerAbility(ir.AbSpell, ...)`. A spell with `Cost`, `Cond`, `Keyword`, `Zones`, `SorceryOnly`, `InstantOnly` or
`Once` set is `spell:<field>` (CC-23 owns spell keywords and costs). Otherwise `head := p.program(a.Body)` and the line
is `A:` + `p.render("SP", head)`.

### 3. Step printers

Common rules:
- **Who.** A player-acting verb puts `st.Who` on its node with `p.operand(n, "Defined", st.Who)`, so a target player
  is declared there. A nil `Who` prints no `Defined$`.
- **Optional.** `st.Optional` with a nil `Who` adds `OptionalDecider$ You`; with a `Who` it is `optional:<verb>`.
- **Amounts** are `p.amount(st.N)`.
- Every printer consumes one step, except the merges named below.

| verb | API and params |
|---|---|
| `VerbDraw` | `Draw`, `NumCards$ <N>`, Who |
| `VerbGainLife` | `GainLife`, `LifeAmount$ <N>`, Who |
| `VerbLoseLife` | `LoseLife`, `LifeAmount$ <N>`, Who |
| `VerbMill` | `Mill`, `NumCards$ <N>`, Who |
| `VerbSurveil` | `Surveil`, `Amount$ <N>`, Who |
| `VerbExtraTurn` | `AddTurn`, `NumTurns$ 1`, Who |
| `VerbEndTurn` | `EndTurn` |
| `VerbDiscard`, Obj `RefChoose` of a hand filter with `Of` nil | `Discard`, `NumCards$ <Obj.N>`, `Mode$ TgtChoose`, Who |
| `VerbDiscard`, Obj `RefAll` of a hand filter | `Discard`, `Mode$ Hand`, Who |
| `VerbDealDamage` | `DealDamage`, `NumDmg$ <N>`, `p.operand(n, "Defined", Obj)`; `Source` nil or `RefSelf` prints nothing, any other source `DamageSource$ <defined>`; a `Who` is `verb:deal-damage-who` |
| `VerbDestroy`, Obj `RefAll` over objects | `DestroyAll`, `ValidCards$ <valid(Obj.Filter)>` |
| `VerbDestroy`, other Obj | `Destroy`, `p.operand(n, "Defined", Obj)` |
| `VerbCounter` | `Counter`, `p.operand(n, "Defined", Obj)` (a target declares `TargetType$ Spell`) |
| `VerbPutCounters` | `PutCounter`, `CounterType$ <Word>`, `CounterNum$ <N>`, `p.operand(n, "Defined", Obj)` |
| `VerbRegenerate` | `Regenerate`; Obj `RefSelf` prints nothing, else `p.operand(n, "Defined", Obj)` |

**`VerbAddMana`** (`Mana`):
- All symbols the same single colour L (or all `ManaColorless`, L is `C`): `Produced$ L`, and `Amount$ <count>`
  when there are several symbols. With `N` set (exactly one symbol), `Amount$ <N>`.
- One symbol of five colours: `Produced$ Any`; of two to four colours: `Produced$ Combo <letters>` in WUBRG order,
  space-separated. `N` gives `Amount$`.
- Several symbols of distinct single colours: `Produced$ <L1> <L2> ...` in IR order.
- Anything else, or `Optional`: `verb:add-mana`.
- **Merge.** Consecutive `VerbAddMana` steps at the head of `steps` whose symbols are all one same single colour L
  print as one node: `Produced$ L`, `Amount$ p.amount(VAdd(c1, c2, ...))`, where a step's count is `VConst(len(Mana))`
  when `N` is nil, else `N`. Build the sum as nested two-argument `VAdd`s. It consumes all of them. (Rite of Flame:
  two `R` plus one `R` per card named Rite of Flame in graveyards.)

**`VerbCreateToken`** (`Token`):
- `TokenAmount$ <N>`, `TokenScript$ <key>`, `TokenOwner$ You` (or `TokenOwner$ <defined(Who)>`), and
  `TokenTapped$ True` when `Enter.Tapped`. Any other `Enter` field is `token:enter`. Optional adds
  `OptionalDecider$ You`.
- `<key>` is the first key, in sorted order, of a token in `p.tokens` whose front face matches the IR `Token` face:
  - colours: the `ColorIndicator` letters equal the token's `Colors` words (as a set);
  - types: `Supertypes`, `Types` and `Subtypes`, each through `Respell`, equal the token's `Types` as a multiset;
  - P/T: when the IR face has both, `<p>/<t>` equals the token's `PT`;
  - keywords: the IR face's bare keyword names equal the token's `Keywords` as a set. A keyword with arguments is
    `token:keyword-args`; any other IR ability is `token:abilities`;
  - name: with `Name` set, the token's name is `Name` or `Name + " Token"`; with no `Name`, it is the subtypes joined
    by one space, plus `" Token"`;
  - an IR face with no `Name` matches only tokens with no `Abilities`, `Triggers` or `Statics`. (Named predefined
    tokens, Food and Treasure, carry their ability in Forge's script and not in the IR.)
- `p.tokens` nil is `token:no-registry`; no match is `token:unmatched`.

**`VerbForEachPlayer`.** Obj is `RefAll` with a player filter, `Then` has exactly one step, that step's `Who` is
`RefIterPlayer`, and a reflect walk over the step finds no other `*ir.Ref` of kind `RefIterPlayer`. Then print a
shallow copy of that step with `Who` set to `st.Obj`, through `stepFns`. Otherwise `verb:for-each-player`.
(`Defined$ Player.Opponent` comes from CC-21a's `RefAll` row.)

**`VerbIf`.**
1. `c := p.cond(st.Cond)`. Save a copy of `p.frame()`'s `declared` slice (nil outside a frame).
2. `then := p.program(st.Then)`.
3. **Amount merge.** If `Else` is set, both branches are one step of the same verb, `c` is an SVar form, and, with
   the saved `declared` restored before printing it, `els := p.program(st.Else)` is a single node with the same API
   whose params equal `then`'s except one param whose two values are decimal literals a and b: print `then` with that
   param set to `p.countVar("Count$Compare " + c.SVar + " " + c.cmp() + "." + a + "." + b)`. Keep the frame state as
   it was after `then`. (Cabal Ritual; Burst Lightning once CC-23 prints its kicker.)
4. Otherwise every node of `then`'s chain gets `c.onAbility()`'s params. With `Else`: restore the saved `declared`,
   print it, give every node of its chain `c.not()`'s `onAbility()` params, and chain it after `then`. A target slot
   both branches declare is `if:shared-target`.

## Implementation steps

1. `print.go` and `registry.go` (section 1). `spell.go`.
2. One file per group: `step_player.go` (draw, gain and lose life, mill, surveil, discard, extra and end turn),
   `step_damage.go` (deal damage, destroy, counter, put counters, regenerate), `step_mana.go`, `step_token.go`,
   `step_flow.go` (if, for-each-player).
3. Run `go run ./cmd/cardc compile -slice fdn` and `-slice legacy`, and record the served totals for the report.

## Tests

In `engine/cardc/gorgebe`:
- `TestStepNodes`: hand-built steps, one case per table row and refusal; assert `API` and `Params`.
- `TestAddMana`: every add-mana form, the merge, and the refusals.
- `TestTokenMatch`: a synthetic `map[string]*cards.Card` of three token cards built through `ParseScript` (a 1/1
  white Cat, a 1/1 white Cat with lifelink, a named Food with an activated ability); the plain Cat, the lifelinked
  Cat, Food, unmatched, keyword-args, abilities and no-registry cases.
- `TestIfAndForEach`: the amount merge (both a mana and a damage variant), the conditioned chain with and without
  `Else`, the shared-target refusal, `for-each-player` over opponents, and its refusals.
- `TestPrintersCompose`: the cross-family cases CC-21b deferred: `VCount` of a named card in graveyards,
  `CRefMatches` of the source being untapped (both renderings), `CCountCompare`, a `RefAll` opponents operand, a
  `cmcLEX` filter whose `X` SVar is `Count$xPaid`.
- `TestPrintSkeleton` and `TestParseScriptRoundTrip` still pass: the unprintable triggered-ability case keeps its
  `ability:triggered` refusal until CC-23.
- `TestServedCardsMatchForge`, rewritten:
  - Compare each served face's **shape** with its Forge face's, as sorted string lists. A shape holds `K:<keyword>`
    per keyword; `A:<SP|AB>:<API chain>` per ability, the chain the APIs down `Sub` joined by `>`; `T:<Mode>:<chain>`
    per trigger, the chain of its `Effect`; `S:<Mode>` per static; `R:<Event>:<chain>` per replacement, the chain of
    `With` (empty when nil). Drop `Cleanup` from Forge's chains.
  - Raise the floor from 25 to the served count you measure, and log it.
  - A served card whose shape differs is a failure. Fix it by making the printer return `Unprintable{What:
    "shape:<construct>"}` for the construct responsible, and list each one in your report. Never loosen the
    comparison.
- `TestServedAPIsSupported` (corpus): every API down every served face's ability, trigger and replacement chains is
  in `effects.Supported()` as `api:<API>`.

## Done means

- [ ] `make test-one PKG=./engine/cardc/gorgebe RUN='^(TestStepNodes|TestAddMana|TestTokenMatch|TestIfAndForEach|TestPrintersCompose|TestServedCardsMatchForge|TestServedAPIsSupported)$'` passes (not ENV).
- [ ] `make test-one PKG=./engine/cardc/gorgebe` passes.
- [ ] `make test-one PKG=./cmd/cardc` passes.
- [ ] `make test-one PKG=./internal/archtest RUN='^(TestDependencyOrderHolds|TestNothingImportsCardc|TestPublicBoundary|TestNoVerbatimForgeLines)$'` passes.
- [ ] The report gives the served count before and after (`cardc compile -slice fdn` and `-slice legacy` totals).
- [ ] No `make test-targeted` or `make test-smart` per ticket: one `make test-smart` runs at the wave-3 head.

## Out of scope

- Zone moves, searches, looks, reveals, `pay-cost`, `apply-effect` and costs (CC-22b). Activated, triggered, static
  and replacement abilities and keyword arguments (CC-23).
- `emit.go` may gain a helper this ticket needs; CC-22b must not edit it.
- Any edit to `engine/cardc/ir`, `engine/cardc/mtgish`, `engine/cards` or an engine package. If the IR cannot carry
  something this brief needs, stop and report it: CC-15a owns IR edits (PLAN.md Decisions log).
- Committing generated script text, Oracle text, or fixtures copied from Forge or mtgish data.
- Anything on GitHub outside gorge. Never open a pull request to i5jb/mtgish.

## Collisions

Edits `print.go`, `registry.go` and `gorgebe_test.go`; may edit `emit.go`. Creates `spell.go`, `step_player.go`,
`step_damage.go`, `step_mana.go`, `step_token.go`, `step_flow.go` and their tests. CC-22b creates only new files and
registers different verbs. Whichever of the two finishes second rebases onto the other and re-runs
`TestServedCardsMatchForge`.
