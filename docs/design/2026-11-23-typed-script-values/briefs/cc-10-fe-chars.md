# feat(cardc): CC-10 front end, card compile driver, characteristics and keyword-only rules

Kind: feature

Target branch: `cardc`. Complexity: medium. Seat: Sonnet.
Design of record: `docs/design/2026-11-23-typed-script-values/PLAN.md` on `cardc`, section 1.
Runs after CC-02 (`cc-02-mtgish-reader`).

## Goal

Lower each mtgish card to an `*ir.Card`. That covers the layout, faces, type line, mana cost, colour indicator,
P/T, loyalty and defence. Also lower the rules that are a bare keyword (`{"_Rule":"Flying"}`) to `AbKeyword`
abilities. Every other rule is handed to the registry, which refuses it by name until a later ticket registers it.

## Behaviour

- `mtgish.Compile(card *Card) *ir.Card` never panics and never returns nil. For each face, in printed order:
  - each rule becomes an ability, or a `Refusal` on the face;
  - characteristics are filled.
- A layout cardc does not play yields one face, named after the card, holding the single refusal `OracleCard.<Layout>`.
- Type-line words are kept in mtgish's spelling (`TimeLord`, `AssemblyWorker`). The gorge backend (CC-20) maps them
  to gorge's spelling by letters-only, case-folded comparison, the rule gorge-compliance's `typeWord` uses. The IR
  does not respell them.

## Design

Decided.

1. **Layouts** (`_OracleCard`):

   | mtgish layout | `ir.Layout` |
   |---|---|
   | `Card` | `LayoutNormal` |
   | `Transforming` | `LayoutTransform` |
   | `ModalDFC` | `LayoutModalDFC` |
   | `Split` | `LayoutSplit` |
   | `Adventurer` | `LayoutAdventure` |
   | `Flip` | `LayoutFlip` |
   | `Melded`, `MeldPiece` | `LayoutMeld` |
   | `Preparer` | `LayoutPrepare` |
   | `Room` | `LayoutRoom` |

   Every other layout is refused whole: `Avenger`, `Conspiracy`, `Planar`, `Scheme`, `Vanguard`, `Dungeon`,
   `DoorInfo`, `FlipInfo`, `Ominous`, and any new one.
2. **Mana symbols** register as `ManaSymbol.<V>` and return an `ir.ManaSym` value:

   | mtgish symbol | `ir.ManaSym` |
   |---|---|
   | `ManaCostGeneric` (args n) | `ManaGeneric` N=n |
   | `ManaCostW`, `U`, `B`, `R`, `G` | `ManaColored` with that colour |
   | `ManaCostWU`, `UB`, `BR`, `RG`, `GW`, `WB`, `UR`, `BG`, `RW`, `GU` | `ManaHybrid` with both colours |
   | `ManaCostWP`, `UP`, `BP`, `RP`, `GP` | `ManaPhyrexian` |
   | `ManaCostRWP`, `RGP`, `GWP`, `GUP` (the schema's only four) | `ManaHybridPhyrexian` with both colours |
   | `ManaCost2W`, `2U`, `2B`, `2R`, `2G` | `ManaTwoHybrid` with that colour, N=2 |
   | `ManaCostC` | `ManaColorless` |
   | `ManaCostX` | `ManaX` |
   | `ManaCostS` | `ManaSnow` |

   `ManaCostCW`/`CU`/`CB`/`CR`/`CG` are not registered. One refused symbol leaves the face's `ManaCost` nil and
   `HasManaCost` false, and records the refusal. A face with no `ManaCost` field has `HasManaCost` false.
3. **Colour indicator:** each colour name maps to its `ColorSet` bit.
4. **P/T:** `CardPT{Power, Toughness}` becomes `*ir.PT{N}`. When the face's rules include `Rule.CDA_Power` (or
   `Rule.CDA_Toughness`), that PT has `Star = true` and N stays as given. The CDA rule itself is a static that CC-16
   lowers.
5. **Loyalty and defence:**
   - `LoyaltyNumber.Integer(n)` sets `Loyalty`. `LoyaltyNumber.LoyaltyX` is not registered, so it is refused.
   - `Defense` is a plain number.
6. **Keyword-only rules** register `Rule.<V>` and return `&ir.Ability{Kind: AbKeyword, Keyword: &ir.Keyword{Name}}`,
   with Name as follows:
   - the word itself: Flying, Deathtouch, Defender, Flash, Haste, Hexproof, Indestructible, Lifelink, Menace, Reach,
     Trample, Vigilance, Prowess, Exalted, Storm, Undying, Delve, Evolve;
   - spaced: `DoubleStrike`→"Double Strike", `FirstStrike`→"First Strike", `LivingWeapon`→"Living Weapon".

   All 21 carry no args in the data at the pin. A registered keyword rule that does arrive with args is an error,
   reported as the refusal `Rule.<V>`.
7. **Driver:**
   - for each face, a fresh `Ctx{Card, Face}`;
   - each element of `Rules` goes through `c.ResetTargets()` then `c.Ability(rule)`;
   - a `*RefusalError` is appended to `Face.Refusals` and the rule is skipped;
   - any other error is also recorded as a refusal, with Key `error:<Enum.Variant>`;
   - `Src` on each node is `c.Src()`.

   Faces come from `Card.Faces` as CC-02 loads them.

## Implementation steps

1. Add `compile.go` (the driver and layout table) and `fe_chars.go` (mana symbols, colour indicator, PT, loyalty and
   defence helpers), plus `fe_keyword.go` (the 21 keyword rules), all in `engine/cardc/mtgish`.
2. Register every lowering in an `init()` in its own file.
3. Write the tests.

## Tests

In `engine/cardc/mtgish`:
- `TestCompileCharsLowering`: a table of synthetic JSON cards built from variant names only (no real card data).
  Each row checks an expected `*ir.Card` with `ir.Equal`:
  - a hybrid-and-phyrexian cost;
  - a `*` PT;
  - a refused layout;
  - a split card;
  - a refused mana symbol.
- `TestKeywordRulesLowering`: each of the 21 keyword rules lowers to its Name, and a keyword rule with args is
  refused.
- `TestCharsSliceVariantsRegistered`: `checkFamilyCoverage` over enums `ManaSymbol`, `OracleCard` and
  `LoyaltyNumber`, with deferred `{}`. At `1553384` the proof slice uses only the `Card` and `Transforming` layouts and
  `LoyaltyNumber.Integer`. The 21 keyword `Rule` keys are not a whole enum, so the test also asserts `Registered`
  for each of them.
- `TestCompileProofSliceNoPanic` (corpus, ENV when absent): compiles every proof-slice card. Every face has a name,
  and every non-refused face has types.

## Done means

- [ ] `make test-one PKG=./engine/cardc/mtgish RUN='^(TestCompileCharsLowering|TestKeywordRulesLowering|TestCharsSliceVariantsRegistered|TestCompileProofSliceNoPanic)$'` passes (not ENV).
- [ ] `make test-targeted BASE=origin/cardc` passes.
- [ ] `make test-smart BASE=origin/cardc` passes.

## Out of scope

- Every non-keyword `Rule` variant (CC-16).
- Keywords with costs or arguments: Ward, Equip, Kicker, Flashback, Protection, HexproofFrom, Affinity, Miracle,
  Surge (CC-16).
- Mapping type words to gorge's spelling (CC-20).
- Anything on GitHub outside gorge. Never open a pull request to i5jb/mtgish.

## Collisions

Depends on CC-02. Its files are `compile.go`, `fe_chars.go` and `fe_keyword.go`, which no other wave-1 ticket
touches. It runs in parallel with CC-11..14: they register other enums, and the registry is keyed by enum.
