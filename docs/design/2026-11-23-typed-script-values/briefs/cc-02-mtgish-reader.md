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
- `mtgish.Load(path, want)` reads the file. Each line becomes a `*Card` holding its faces.
- `NewIndex(cards).Lookup(name)` finds a card by:
  - any face name;
  - the front face name;
  - the `A // B` join of all face names in printed order.
- Lowering an unregistered variant returns a `*RefusalError` naming `Enum.Variant` and the path.
- `cardc report -slice fdn|blb|legacy|decks|all [-data FILE]` prints one TSV line per variant
  (`key<TAB>cards<TAB>registered`), sorted by key, then a totals line.
- A test that needs the data and finds none calls
  `t.Skip("cardc: no mtgish corpus under .cards/mtgish -- run `make mtgish-fetch`")`. That text matches the ENV
  pattern of `scripts/test_targets.py:32`, so the verdict is ENV, never PASS.

## Design

Decided.

1. **Lift, as code**, from gorge-compliance `oracle/mtgish` at commit `285a7038` (same owner, Apache-2.0):
   - `node.go`, the whole file: the tagged-union helpers `tagged`, `variantOf`, `enumOf`, `qualified`, `argsOf`,
     `rawArgs`, `argN`, `list`, `field`, `intOf`, `walk`. Drop `gameNumber`, which is summary-only.
   - `load.go`, the whole file: `Card`, `faceKeys`, `skippedLayouts`, `decodeCard`, `FaceNames`, `Load`, `Read`,
     `keep`, `SortedNames`.

   Change only the package clause and the doc comments. Do not lift `vocab.go`, `exclude.go` or any `map_*.go`:
   they produce summary tokens. Add a header comment naming the source repo, path and commit.
2. **Lock reader**, `lock.go`.
   - `type Lock struct { Repo, Upstream, Base, Ref string; Files []LockFile; Patches []LockPatch }`.
   - `type LockFile struct{ Sum, Path string }`.
   - `type LockPatch struct{ SHA string; Issue int }`.
   - `ReadLock(path string) (Lock, error)` reads the format CC-00 wrote: `repo`, `upstream`, `base`, `ref`, `sha256`
     and `patch` lines, with `#` comments.
3. **Index**, `index.go`.
   - `type Index struct` holds an unexported `map[string]*Card` and a sorted `[]string` of keys. The map is only read
     by key; anything that lists keys uses the sorted slice.
   - For a duplicate key, the first card in file order wins (as `Read` already does).
4. **Registry and context**, `lower.go`.
   - `type lowerFunc func(c *Ctx, n node) (any, error)`.
   - `func register(key string, f lowerFunc)`. key is `Enum.Variant`. A duplicate key panics.
   - `func Registered(key string) bool` and `func RegisteredKeys() []string`, sorted.
   - `type Ctx struct { Card string; Face int; path []string; targets []declared }`, with
     `type declared struct{ variant string; slot uint8 }`.
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
   - `(c *Ctx) DeclareTarget(variant string) uint8` appends a declaration and returns its slot, the index in
     declaration order.
   - `(c *Ctx) TargetSlot(refVariant string) (uint8, bool)`. Given `Ref_TargetPermanent2`, it:
     - strips `Ref_`;
     - takes a trailing decimal k (1-based) as "the k-th declaration of variant `TargetPermanent`";
     - with no digits, takes the latest declaration of that variant;
     - returns `false` when there is none.
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
   - `SchemaPath()` sits beside `DataPath()`.
   - The schema drives `faceKeys`: every field of an `_OracleCard` variant whose type is `Card` is a face key. That
     adds `Vengeance` (the `Avenger` layout), which the compliance list lacks. `TestFaceKeysMatchSchema` holds them
     equal.
   - `cardc report` gains a `schema` column (`yes`/`no`). A data variant missing from the schema fails the report.
8. **Shared test helpers**, `helpers_test.go`, for CC-10..14.
   - `proofSlice(t) []*Card` loads `fdn` plus `legacy`, and skips with the ENV message when the data is absent.
   - `checkFamilyCoverage(t, enums []string, deferred map[string]string)` runs `Census` on the proof slice. For every
     key whose enum is in `enums`:
     - it fails when the key is neither registered nor in `deferred`;
     - it fails when the key is in `deferred` but registered (stale);
     - it fails when a `deferred` key no longer occurs in the slice.

     Each `deferred` value is a one-line reason.
   - `withStubs(t, stubs map[string]lowerFunc)` overlays test-only lowerings on the registry for one test and
     restores it in `t.Cleanup`. It uses an unexported overlay map consulted before the real registry in `lower.go`.
     Tests that use it must not call `t.Parallel`. With it, CC-11..14 test their own family without waiting on each
     other, and no wave-1 ticket edits `lower.go`.
9. **`cmd/cardc`**: `main.go` with the subcommand `report`.
   - Flags: `-slice` (default `fdn`) and `-data` (default `mtgish.DataPath()`).
   - Output as in Behaviour. Exit 1 when any slice name does not resolve, listing the names.

## Implementation steps

1. Copy `node.go` and `load.go` as described, and adapt their package clause.
2. Write `lock.go`, `index.go`, `lower.go`, `census.go`, `slice.go`, `schema.go` and `data.go` (DataPath and
   SchemaPath).
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
- `TestTargetSlotResolution`: declarations `TargetPermanent`, `TargetPlayer`, `TargetPermanent`:
  - `Ref_TargetPermanent` → 2;
  - `Ref_TargetPermanent1` → 0;
  - `Ref_TargetPermanent2` → 2;
  - `Ref_TargetSpell` → false.
- `TestSliceNamesResolve` (corpus; ENV when the data is absent): every name from `fdn`, `blb` and `decks` resolves
  through the index. `legacy` is covered too.
- `TestSchemaCoversData` (corpus; ENV when the data is absent): every `Enum.Variant` in the data is in the schema.
- `TestFaceKeysMatchSchema` (corpus; ENV when the data is absent): `faceKeys` equals the schema-derived list.
- `TestCoverageHelperDetectsStale`: runs `checkFamilyCoverage` against a fake `*testing.T` recorder and a synthetic
  census, and checks the three failure modes.
- `TestRegisteredKeysInSchema` (corpus; ENV when the data is absent): every registered key is in the schema. This is
  the check later lowering tickets rely on to catch a misspelt or renamed variant.

In `cmd/cardc`:
- `TestReportOnFixture`: runs the report on a three-line fixture written to `t.TempDir()`. The fixture holds only
  synthetic card names and invented variants, never real card data.

## Done means

- [ ] `make test-one PKG=./engine/cardc/mtgish RUN='^(TestReadLock|TestTaggedHelpers|TestLoweringRefusesUnregistered|TestRegisterDuplicatePanics|TestTargetSlotResolution|TestSliceNamesResolve|TestSchemaCoversData|TestFaceKeysMatchSchema|TestCoverageHelperDetectsStale|TestRegisteredKeysInSchema)$'` passes (not ENV).
- [ ] `make test-one PKG=./cmd/cardc RUN='^TestReportOnFixture$'` passes.
- [ ] `make test-one PKG=./internal/archtest RUN='^(TestDependencyOrderHolds|TestNothingImportsCardc|TestPublicBoundary)$'` passes.
- [ ] `make test-targeted BASE=origin/cardc` passes.

## Out of scope

- Any registered lowering (CC-10..16).
- Any import of `engine/cards` from `engine/cardc/mtgish` (forbidden by CC-00's archtest row).
- Committing any mtgish data or any test fixture copied from it.

## Collisions

Depends on CC-01. Creates new files only: `engine/cardc/mtgish/*` and `cmd/cardc/main.go`. CC-10..14 then each add
their own `fe_*.go` files in this package and register through `register`. Nobody else edits `lower.go`. Later
`cardc` subcommands (CC-20, CC-30, CC-31) each add their own file under `cmd/cardc`.
