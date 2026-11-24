# feat(cardc): CC-02 the mtgish reader, name index, lowering registry and variant census

Kind: feature

Target branch: `cardc`. Complexity: medium. Seat: Sonnet.
Design of record: `docs/design/2026-11-23-typed-script-values/PLAN.md` on `cardc`, sections 1-2.
Runs after CC-01 (`cc-01-ir-schema`).

## Goal

Create `engine/cardc/mtgish`: read the pinned `mtgish.lines.json`, index cards by every name gorge uses, and give
the front-end tickets (CC-10..16) one registry to plug variant lowerings into. Add `cmd/cardc report`, which counts
every `Enum.Variant` a slice uses and whether a lowering is registered.

## Behaviour

- `mtgish.DataPath()` returns the data file path:
  - `$GORGE_MTGISH` when set;
  - else `<module root>/.cards/mtgish/<ref>/mtgish.lines.json`, with `<ref>` read from `engine/cardc/MTGISH.lock`.
    The module root is the first ancestor of the working directory that holds `go.mod`.
- `mtgish.Load(path, want)` (lifted) returns a map keyed by front name. `mtgish.LoadAll(path)` returns every card as
  a `[]*Card` in file order, with no filter and no dedupe. Each line becomes a `*Card` holding its faces.
- `NewIndex(LoadAll(...)).Lookup(name)` finds a card by, in this order of precedence:
  1. the front face name (`Card.Name`);
  2. the `A // B` join of all face names in printed order;
  3. any other face name, first card in file order.

  The order matters: at `1553384`, 26 `Preparer` cards have a `Prepared` face that reuses a real card's name. Two are
  in the Legacy slice (Lightning Bolt and Swords to Plowshares, both earlier in the file as `Prepared` faces than as
  cards).
- Lowering an unregistered variant returns a `*RefusalError` naming `Enum.Variant` and the path.
- `cardc report -slice fdn|blb|legacy|decks|all [-data FILE]` prints one TSV line per variant
  (`key<TAB>cards<TAB>registered`), sorted by key, then a totals line.
- A test that needs the data and finds none calls
  `t.Skip("cardc: no mtgish corpus under .cards/mtgish -- run make mtgish-fetch")`. That text matches the ENV
  pattern of `scripts/test_targets.py:32` (it contains `.cards`), so the verdict is ENV, never PASS.

## Design

Decided.

1. **Lift, as code**, from gorge-compliance `oracle/mtgish` at commit `285a7038` (same owner, Apache-2.0):
   - `node.go`, the whole file: the tagged-union helpers `tagged`, `variantOf`, `enumOf`, `qualified`, `argsOf`,
     `rawArgs`, `argN`, `list`, `field`, `intOf`, `walk`. Drop `gameNumber`, which is summary-only.
   - `load.go`, the whole file: `Card`, `faceKeys`, `skippedLayouts`, `decodeCard`, `FaceNames`, `Load`, `Read`,
     `keep`, `SortedNames`.

   Change only the package clause, the doc comments and `faceKeys` (item 7). Do not lift `vocab.go`, `exclude.go` or
   any `map_*.go`: they produce summary tokens. Add a header comment naming the source repo, path and commit.
   - `loadall.go` (new, not lifted): `LoadAll(path string) ([]*Card, error)` and `ReadAll(r io.Reader) ([]*Card,
     error)`, built on the lifted `decodeCard`. The index needs file order, which `Load`'s map loses.
2. **Lock reader**, `lock.go`.
   - `type Lock struct { Repo, Upstream, Base, Ref string; Files []LockFile; Patches []LockPatch }`.
   - `type LockFile struct{ Sum, Path string }`.
   - `type LockPatch struct{ SHA string; Issue int }`.
   - `ReadLock(path string) (Lock, error)` reads the format CC-00 wrote: `repo`, `upstream`, `base`, `ref`, `sha256`
     and `patch` lines, with `#` comments.
3. **Index**, `index.go`.
   - `type Index struct` holds two unexported maps, `primary` (front names and `A // B` joins) and `secondary`
     (every other face name), plus a sorted `[]string` of all keys. The maps are only read by key; anything that
     lists keys uses the sorted slice.
   - `Lookup` tries `primary`, then `secondary`. Within one map, the first card in file order wins.
4. **Registry and context**, `lower.go`.
   - `type lowerFunc func(c *Ctx, n node) (any, error)`.
   - `func register(key string, f lowerFunc)`. key is `Enum.Variant`. A duplicate key panics.
   - `func Registered(key string) bool` and `func RegisteredKeys() []string`, sorted.
   - `type Ctx struct { Card string; Face int; path []string; targets []declared }`, with
     `type declared struct{ stem string; slot uint8 }`.
   - `func (c *Ctx) lower(x any) (any, error)`:
     - read `tagged(x)` to get the key;
     - push the key onto `path`, call the registered func, pop;
     - an unregistered key returns `&RefusalError{Key, Path: c.Src()}`.
   - Typed helpers call `lower` and type-assert:
     - `(c *Ctx) Ref(x) (*ir.Ref, error)`
     - `Filter(x) (*ir.Filter, error)`
     - `Value(x) (*ir.Value, error)`
     - `Cond(x) (*ir.Cond, error)`
     - `Compare(x) (*ir.Compare, error)`
     - `Target(x) (*ir.Target, error)`
     - `Ability(x) (*ir.Ability, error)`

     A wrong dynamic type is an error `"<key> lowers to <T>, want <U>"`.
   - `(c *Ctx) Src() string` joins `path` with `/`.
   - `(c *Ctx) DeclareTarget(stem string) uint8` appends a declaration and returns its slot, the index in
     declaration order. `stem` is the name the data's `Ref_` variants use for this target, which is not always the
     `Target` variant's name: `UptoOneTargetPermanent` declares stem `TargetPermanent`, and `NumberTargetPermanents`
     declares `TargetPermanents`. CC-12's target table gives every stem.
   - `(c *Ctx) ResetTargets()` empties the declarations. The CC-10 driver calls it before each rule, so slots are
     per ability.
   - `(c *Ctx) TargetSlot(refVariant string) (uint8, bool)`. Given `Ref_TargetPermanent2`, it:
     - strips `Ref_`;
     - takes a trailing decimal k (1-based) as "the k-th declaration with stem `TargetPermanent`";
     - with no digits, takes the latest declaration with that exact stem (`TargetPermanents` never matches
       `TargetPermanent`);
     - returns `false` when there is none, and for an `_<n>` suffix (`Ref_AnyTargets_1`), which no proof-slice card
       uses.
   - `type RefusalError struct{ Key, Path string }` implements `error`.
5. **Census**, `census.go`.
   - `type VariantCount struct{ Key string; Cards int; Registered bool }`.
   - `func Census(cards []*Card) []VariantCount` walks every node of every card with `walk`. It counts distinct cards
     per key, sorted by key.
6. **Slices**, `slice.go`. `func SliceNames(root, slice string) ([]string, error)`.
   - `fdn` and `blb` read `dirac/cardset/pools/<slice>/cards.txt`.
   - `decks` reads every `testkit/testutil/decks/*.json` file's `cards[].name`, with `encoding/json`. Import no
     testkit package.
   - `legacy` reads the 12 decks of `legacyDeckNames` (`testkit/testutil/decks.go:65`), with the list copied into
     `slice.go` and a comment citing it. CC-10..14's coverage tests use `fdn` plus `legacy`, the plan's proof slice.
   - `all` returns nil, meaning every card.
7. **Schema catalogue**, `schema.go`. mtgish commits its JSON shape as ts-rs output,
   `typescript_types/oracle_cards.d.ts`; CC-00 fetches it beside the data. At `1553384` it holds 403 types and 7,455
   `Enum.Variant`s, and every variant the data uses (7,407) is among them (measured from a checkout at the pin).
   - `func ReadSchema(path string) (Schema, error)` parses the union lines with one regexp:
     `^\| \{ "_(\w+)": "(\w+)"(, "args": (.+?))?`, plus struct-variant fields `Name?: Type`.
   - `type Schema struct` holds a sorted `[]Variant{Enum, Name, Args string; Fields []string}` and `Has(key) bool`.
   - A variant listed under two TS types (`ManaSymbol` and `ManaSymbolX` repeat the same `_ManaSymbol` lines) is one
     entry; the first line seen wins.
   - `SchemaPath()` sits beside `DataPath()`.
   - The schema drives `faceKeys`: every field of an `_OracleCard` variant whose type is `Card`, `FlipInfo` or
     `DoorInfo` is a face key (`Cards: Array<Card>` stays the lifted code's separate list case). At `1553384` that is
     `FrontFace`, `BackFace`, `Unflipped`, `Flipped`, `LeftDoor`, `RightDoor`, `Adventure`, `Omen`, `Prepared` and
     `Vengeance` (the `Avenger` layout).
   - So append `"Vengeance"` to the lifted `faceKeys`; it is the one edit to `load.go` beyond the package clause and
     comments. `TestFaceKeysMatchSchema` holds the two lists equal as sets.
   - `cardc report` gains a `schema` column (`yes`/`no`). A data variant missing from the schema fails the report.
8. **Shared test helpers**, `helpers_test.go`, for CC-10..14.
   - `proofSlice(t) []*Card` resolves the `fdn` plus `legacy` names through `NewIndex(LoadAll(DataPath()))`, and
     skips with the ENV message when the data is absent.
   - `familyCoverageProblems(census []VariantCount, enums []string, deferred map[string]string, registered
     func(string) bool) []string` is a pure function. For every census key whose enum is in `enums`, it reports:
     - a key neither registered nor in `deferred`;
     - a key in `deferred` that is registered (stale);
     - a `deferred` key that no longer occurs in the census.

     Each `deferred` value is a one-line reason.
   - `checkFamilyCoverage(t, enums, deferred)` runs `Census` on the proof slice, calls `familyCoverageProblems` with
     `Registered`, and reports each problem with `t.Error`.
   - A ticket that must cover single keys rather than a whole enum (CC-10's keyword `Rule`s) asserts `Registered`
     for each key itself.
   - `withStubs(t, stubs map[string]lowerFunc)` overlays test-only lowerings on the registry for one test and
     restores it in `t.Cleanup`. It uses an unexported overlay map consulted before the real registry in `lower.go`.
     Tests that use it must not call `t.Parallel`. With it, CC-11..14 test their own family without waiting on each
     other, and no wave-1 ticket edits `lower.go`.
9. **`cmd/cardc`**: `main.go` with the subcommand `report`.
   - Flags: `-slice` (default `fdn`) and `-data` (default `mtgish.DataPath()`).
   - Output as in Behaviour. Exit 1 when any slice name does not resolve, listing the names.

## Implementation steps

1. Copy `node.go` and `load.go` as described, and adapt their package clause.
2. Write `loadall.go`, `lock.go`, `index.go`, `lower.go`, `census.go`, `slice.go`, `schema.go` and `data.go`
   (DataPath and SchemaPath).
3. Write `cmd/cardc/main.go`.
4. Run `make mtgish-fetch` (from CC-00), then `go run ./cmd/cardc report -slice fdn`. Every FDN name resolves; all
   counts show `registered=false`, because no lowering exists yet.

## Tests

In `engine/cardc/mtgish`:
- `TestReadLock`: parses `engine/cardc/MTGISH.lock`, and a fixture string with two `patch` lines.
- `TestTaggedHelpers`: synthetic JSON nodes for a tuple variant, a struct variant and a unit variant.
- `TestLoweringRefusesUnregistered`: lowering `{"_Action":"NoSuchVariant"}` returns a `*RefusalError` with that key
  and path.
- `TestRegisterDuplicatePanics`.
- `TestTargetSlotResolution`: declaration stems `TargetPermanent`, `TargetPlayer`, `TargetPermanent`,
  `TargetPermanents`:
  - `Ref_TargetPermanent` → 2;
  - `Ref_TargetPermanent1` → 0;
  - `Ref_TargetPermanent2` → 2;
  - `Ref_TargetPermanents` → 3;
  - `Ref_TargetSpell` → false;
  - `Ref_TargetPermanents_1` → false;
  - after `ResetTargets`, `Ref_TargetPermanent` → false.
- `TestIndexPrefersFrontFace`: synthetic cards, a `Preparer` whose `Prepared` face is named `Zz Bolt` and, later in
  file order, a `Card` named `Zz Bolt`. `Lookup("Zz Bolt")` returns the `Card`; `Lookup` of the `Preparer`'s front
  name returns the `Preparer`.
- `TestSliceNamesResolve` (corpus; ENV when the data is absent): every name from `fdn`, `blb` and `decks` resolves
  through the index. `legacy` is covered too.
- `TestSchemaCoversData` (corpus; ENV when the data is absent): every `Enum.Variant` in the data is in the schema.
- `TestFaceKeysMatchSchema` (corpus; ENV when the data is absent): `faceKeys` equals the schema-derived list.
- `TestCoverageHelperDetectsStale`: calls `familyCoverageProblems` on a synthetic census and registry, and checks
  each of the three problems is reported, and nothing for a clean input.
- `TestRegisteredKeysInSchema` (corpus; ENV when the data is absent): every registered key is in the schema. This is
  the check later lowering tickets rely on to catch a misspelt or renamed variant.

In `cmd/cardc`:
- `TestReportOnFixture`: runs the report on a three-line fixture written to `t.TempDir()`. The fixture holds only
  synthetic card names and invented variants, never real card data.

## Done means

- [ ] `make test-one PKG=./engine/cardc/mtgish RUN='^(TestReadLock|TestTaggedHelpers|TestLoweringRefusesUnregistered|TestRegisterDuplicatePanics|TestTargetSlotResolution|TestIndexPrefersFrontFace|TestSliceNamesResolve|TestSchemaCoversData|TestFaceKeysMatchSchema|TestCoverageHelperDetectsStale|TestRegisteredKeysInSchema)$'` passes (not ENV).
- [ ] `make test-one PKG=./cmd/cardc RUN='^TestReportOnFixture$'` passes.
- [ ] `make test-one PKG=./internal/archtest RUN='^(TestDependencyOrderHolds|TestNothingImportsCardc|TestPublicBoundary)$'` passes.
- [ ] `make test-targeted BASE=origin/cardc` passes.
- [ ] `make test-smart BASE=origin/cardc` passes.

## Out of scope

- Any registered lowering (CC-10..16).
- Any import of `engine/cards` from `engine/cardc/mtgish` (forbidden by CC-00's archtest row).
- Committing any mtgish data or any test fixture copied from it.
- Anything on GitHub outside gorge. Never open a pull request to i5jb/mtgish.

## Collisions

Depends on CC-01. Creates new files only: `engine/cardc/mtgish/*` and `cmd/cardc/main.go`. CC-10..14 then each add
their own `fe_*.go` files in this package and register through `register`. Nobody else edits `lower.go`. Later
`cardc` subcommands (CC-20, CC-30, CC-31) each add their own file under `cmd/cardc`.
