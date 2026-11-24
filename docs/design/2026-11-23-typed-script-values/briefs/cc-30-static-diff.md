# feat(cardc): CC-30 the static diff (cardc's served card against Forge's, field by field)

Kind: feature

Target branch: `cardc`. Complexity: medium. Implementer: Sonnet.
Design of record: `docs/design/2026-11-23-typed-script-values/PLAN.md` on `cardc`, section 4 ("Static diff"). Runs
after CC-22a/b, CC-23 and CC-24 (all on `cardc`). Runs in parallel with CC-33a.

## Goal

For every card cardc serves, compare its `*cards.Card` with Forge's, item by item and field by field, after a fixed
set of value canonicalisations, and print one row per difference. Add `cardc diff`. CC-32 holds these rows in the
divergence ledger; this ticket only computes and prints them.

## Workspace facts (verified at `de85384ff1` plus CC-24)

- `gorgebe.Build(forge, compiled, fb)` (`engine/cardc/gorgebe/registry.go:38`) returns a `*Registry`; each
  `Entry` (`registry.go:19`) has `Origin`, `Forge` (Forge's card) and `Card` (cardc's, for `OriginCardc`).
- `cards.Card` / `Face` / `SA` / `Trigger` / `Static` / `Repl`: `engine/cards/ir.go:11-142`. `SA.Sub` is the linked
  `SubAbility$` chain; a `Trigger`'s body is `Effect`, a `Repl`'s is `With`; `Face.SVars` holds the SVar bodies.
- `TestServedCardsMatchForge` (`gorgebe_test.go:378`) already holds each served face's **shape** equal to Forge's:
  the sorted multiset of `K:<kw>`, `A:<Kind>:<chain>`, `T:<Mode>:<chain>`, `S:<Mode>`, `R:<Event>:<chain>`, with
  `chainOf` (`gorgebe_test.go:343`) the APIs down the `Sub` chain, `Cleanup` dropped. So on served cards every item
  pairs; this ticket compares what is inside the pairs.
- `cmd/cardc compile` (`cmd/cardc/compile.go`) builds the registry over a slice; `sliceCards`
  (`gorgebe_test.go:255`) is the test-side recipe.
- At CC-24's head, `cardc compile` serves 222 distinct cards of the FDN and Legacy slices.
- `TestNoVerbatimForgeLines` (`internal/archtest/public_boundary_test.go:498`) fails on any whole corpus script line
  of 30 or more characters in a committed file. Tests use synthetic `Zz` scripts and assert on rows.

## Design

Decided. The seat makes no design calls; what this ticket does not cover is out of scope, not improvised.

### 1. Package `engine/cardc/diff` (new; imports `engine/cards` only)

```go
// Row is one difference between cardc's card and Forge's.
type Row struct {
    Card   string // the card name
    Face   int    // face index
    Item   string // the item's shape key and its index among equal keys: "T:ChangesZone:Draw#0"
    Field  string // "-" for an unpaired item or a face-count row; "[k]<Key>" on chain node k; "<Key>" on the line
    Family string // see section 2; "shape" for unpaired items and face counts
    Cardc  string // canonical value; "" when absent
    Forge  string
}
func (r Row) Key() string // "<face>/<item>/<field>"
func (r Row) Sig() string // the first 8 hex digits of sha256(Cardc + "\x00" + Forge)

// Card compares cardc's definition of one card with Forge's. Rows come out in a fixed order:
// face, then item key, then field.
func Card(name string, cardc, forge *cards.Card) []Row
```

**Faces** pair by index. Unequal face counts give one row `Item "faces"`, `Field "-"`, `Family "shape"`, the two
counts as values, and nothing else for that card.

**Items.** Per face, each keyword, ability, trigger, static and replacement gets the shape key above (reuse the
`chainOf` rule; copy it into `diff`, the test helper stays). Items with the same key pair in order of appearance,
and `#i` is the index within that key. An item on one side only is a row with `Field "-"`, `Family "shape"`, and
`present` on the side that has it.

**Fields.** On a paired item, compare every key of section 2 present on either side:
- line-level params: the `Trigger`, `Static` or `Repl` `Params`, and the top `SA`'s `Params` for an `A:` item;
- chain params: each node `k` of the body chain (`A:` the SA itself and its `Sub`s; `T:` `Effect` and its `Sub`s;
  `R:` `With` and its `Sub`s), `Cleanup` nodes skipped so `k` matches `chainOf`, field `[k]<Key>`.
For an `A:` item the top SA is both the line and node 0: its keys print as `[0]<Key>`.

### 2. Compared keys and families

| family | keys |
|---|---|
| `targets` | `ValidTgts`, `TgtZone`, `TargetMin`, `TargetMax`, `TargetType`, `TargetUnique` |
| `who` | `Defined`, `ValidCards`, `ValidPlayers` |
| `amounts` | `Amount`, `ChangeNum`, `CharmNum`, `ChoiceAmount`, `CounterNum`, `CounterType`, `DigNum`, `LifeAmount`, `MinCharmNum`, `Num`, `NumAtt`, `NumCards`, `NumDef`, `NumDmg`, `NumTurns`, `Produced` |
| `zones` | `Origin`, `Destination`, `ChangeType` |
| `cost` | `Cost`, `SorcerySpeed`, `ActivationLimit`, `ActivationZone` |
| `trigger` | `ValidCard`, `ValidPlayer`, `ValidSource`, `ValidTarget`, `ValidActivatingPlayer`, `Phase`, `TriggerZones`, `CombatDamage`, `FirstTime`, `Number` |
| `static` | `Affected`, `AffectedZone`, `AddPower`, `AddToughness`, `AddKeyword`, `ValidAttacker`, `Activator`, `Type`, `SetMaxHandSize`, `MayPlay`, `MayPlayWithoutManaCost`, `Caster`, `ValidSA`, `EffectZone` |
| `repl` | `ActiveZones`, `Layer`, `ReplacementResult` |
| `cond` | `CheckSVar`, `SVarCompare`, `IsPresent`, `PresentCompare`, `PresentZone`, `PlayerTurn`, `ConditionCheckSVar`, `ConditionSVarCompare`, `ConditionPresent`, `ConditionCompare` |

PLAN.md names targets, amounts and trigger modes; `who`, `zones`, `cost`, `static`, `repl` and `cond` are the same
check applied to the other params gorgebe prints. A key in no row is never compared (descriptions, AI hints, prompts).
One map `key -> family` holds the table; a key listed twice is a test failure.

### 3. Canonical values (`canon.go`)

Applied to both sides, in this order, and nothing else:
1. `strings.TrimSpace`; runs of whitespace collapse to one space.
2. **SVar names.** A value that is exactly a key of that face's `SVars` becomes `=` + the canonical form of the body
   (steps 1, 3-5 on the body, one level: a body naming another SVar is not resolved again). So `NumDmg$ X` with
   `X:Count$xPaid` on one side equals `NumDmg$ Y`, `Y:Count$xPaid` on the other.
3. **Lists.** Except for `Cost`: split on `,`; within each element, a `Base.p1+p2+…` term keeps `Base.` and sorts the
   `+` terms; then sort the elements and join with `,`. `AddKeyword` splits on ` & ` and sorts instead.
4. **Booleans.** `true` / `false` in any case become `True` / `False`.
5. **Target defaults.** On a node with `ValidTgts` on either side, an absent `TargetMin` or `TargetMax` is `1`.

Anything else that means the same but is spelled differently is a row; CC-32 ledgers it as `equivalent`, and the
burn-down either respells gorgebe's output or adds a canonicalisation here with its own test.

### 4. `engine/cardc/slice` (new; imports `mtgish`, `gorgebe`, `engine/cards`)

The one recipe for "the cardc registry over named slices", for `cmd/cardc`, the diff tests and CC-32:

```go
// Build compiles the named slices' cards (each slice's names in order, a repeated name kept first), reads the
// fallback register under root/engine/cardc/testdata/fallback, and builds the gorge backend's registry.
// A missing mtgish data file is ErrNoData; an unresolved name is an error naming it.
func Build(root string, forge *cards.Registry, slices ...string) (*gorgebe.Registry, error)
var ErrNoData = errors.New(mtgish.SkipMessage)
```

`cmd/cardc compile` keeps its own loop (its per-slice output is unchanged); only the new code uses `slice.Build`.

### 5. `cardc diff` (`cmd/cardc/diff.go`, new; one dispatch line and one usage line in `main.go`)

`cardc diff [-slice fdn|legacy|proof]` (default `proof` = fdn then legacy, de-duplicated by name). Over the served
entries (`OriginCardc`), in registry order, print each row as
`card<TAB>face<TAB>item<TAB>field<TAB>family<TAB>cardc<TAB>forge`, then one line per family that has rows,
`family<TAB><name><TAB>rows=<n><TAB>cards=<m>`, sorted by name, then
`total<TAB>slice=<s><TAB>served=<n><TAB>clean=<c><TAB>differing=<d><TAB>rows=<r>`. Exit 0 whatever the rows; a
build error exits 1. Values go to stdout only; nothing is written under the repo.

### 6. Classification

`docs/site/architecture/packages.tiers.json`: add `"engine/cardc/diff"` and `"engine/cardc/slice"`, both
`{"tier": "engine", "status": "TOOL"}`, beside the other `engine/cardc/*` rows. The pre-commit hook refuses the commit
without them. `go run ./cmd/pkgmap -check` must exit 0.

## Tests

In `engine/cardc/diff`:
- `TestCanon`: each rule of section 3, one case each, plus `Cost` left unsplit and a two-level SVar left at one level.
- `TestCardRows`: pairs of synthetic scripts parsed with `gorgebe.ParseScript` (`Zz` names, ten lines or fewer,
  no corpus text): a `NumDmg` difference (one `amounts` row, `[0]NumDmg`); `ValidTgts` in another order (clean);
  SVar names differing with equal bodies (clean); `TargetMax` absent against `1` (clean); a trigger `ValidCard`
  difference on a `T:` item (field `ValidCard`, family `trigger`); a difference on chain node 1 (`[1]…`); an extra
  ability on one side (`shape`, `-`); two face counts (one `faces` row); a description-only difference (clean).
- `TestFamiliesDisjoint`: no key in two families.
- `TestDiffIdentityIsClean` (corpus, `testutil.CorpusRegistry(t)`): every card of the FDN and Legacy slices,
  reparsed from its own Forge script (`os.ReadFile(c.Path)`, `gorgebe.ParseScript`) and diffed against Forge's, gives
  no rows. A row here is a diff bug, never a card's.
- `TestDiffSlice` (corpus; ENV without mtgish data via `slice.ErrNoData`): over `slice.Build(root, forge, "fdn",
  "legacy")`, diff every served card twice and require equal rows (determinism); every row's family is a family of
  section 2 or `shape`; no `shape` rows (`TestServedCardsMatchForge` holds shapes). Log served, differing and rows
  per family.

In `engine/cardc/slice`: `TestBuildProofSlice` (corpus): `Build` over `fdn`, `legacy` serves at least 222 cards (CC-24's
`servedFloor`) and returns `ErrNoData` with `GORGE_MTGISH` pointing at a missing file.

In `cmd/cardc`: `TestDiffUsage`: `run([]string{"diff", "-slice", "nope"}, …)` exits non-zero.

## Done means

- [ ] `make test-one PKG=./engine/cardc/diff RUN='^(TestCanon|TestCardRows|TestFamiliesDisjoint|TestDiffIdentityIsClean|TestDiffSlice)$'` passes (not ENV).
- [ ] `make test-one PKG=./engine/cardc/slice RUN='^TestBuildProofSlice$'` passes (not ENV).
- [ ] `make test-one PKG=./cmd/cardc RUN='^(TestDiffUsage|TestLockstepUsage)$'` passes.
- [ ] `make test-one PKG=./internal/archtest RUN='^(TestDependencyOrderHolds|TestNothingImportsCardc|TestPublicBoundary|TestNoVerbatimForgeLines)$'` passes.
- [ ] `go run ./cmd/pkgmap -check` exits 0.
- [ ] The hand-back quotes `go run ./cmd/cardc diff`'s family and total lines.
- [ ] No `make test-targeted` or `make test-smart` per ticket: one `make test-smart` runs at the wave-4 head.

## Out of scope

- The ledger, its classes and its ratchet (CC-32). Fixing any card a row names.
- Any edit to `engine/cardc/ir`, `engine/cardc/mtgish`, `engine/cardc/gorgebe` (non-test), `engine/cards` or an engine
  package. The IR-level `engine/cardc/canon` of PLAN.md's layout (this ticket canonicalises `*cards.Card` values).
- Committing generated script text, Oracle text, row values or fixtures copied from Forge or mtgish data.
- Anything on GitHub outside gorge. Never open a pull request to i5jb/mtgish.

## Collisions

New packages `engine/cardc/diff` and `engine/cardc/slice`, new `cmd/cardc/diff.go`; one dispatch line and one usage
line in `cmd/cardc/main.go`; two `packages.tiers.json` rows. CC-33a touches none of these.
