# feat(cardc): CC-20 gorge backend skeleton, the card registry and the fallback register

Kind: feature

Target branch: `cardc`. **Complexity: high. Review: Opus** (a hub: every later backend ticket prints into it, and
CC-31's lockstep seats its cards). Implementer: Sonnet.
Design of record: `docs/design/2026-11-23-typed-script-values/PLAN.md` on `cardc`, sections 1.3, 1.4 and 5
("Fallback"). Runs after CC-10 (`cc-10-fe-chars`, merged at `69951fbdb5`). It does not depend on CC-15/16.

## Goal

Create `engine/cardc/gorgebe`:
1. **Print** an `*ir.Card` as gorge script text, in gorge's own vocabulary.
2. **Load** that text through gorge's existing parser, so the result is an ordinary `*cards.Card`.
3. **Build** a registry that serves each slice card from cardc when cardc covers it, and Forge's `*cards.Card` by
   reference otherwise, with the reason.

This ticket prints the **skeleton**: characteristics plus bare keywords. Any other ability is "unprintable" for now,
so its card is served from Forge. CC-21..23 widen the printer without changing this API. Add `cardc compile`.

## Behaviour

- `gorgebe.Print(c, oracle)` returns script text, or an `*Unprintable` naming the first construct it cannot print.
- `gorgebe.ParseScript(path, text)` returns a game-ready `*cards.Card`: `cards.ParseBytes` (`parse.go:21`), then
  `(*Card).Link` (`link.go:15`), then `(*Face).ApplyIntrinsics` (`intrinsic.go:44`) on each face. This is the
  recipe `testutil`'s `parseCard` uses (`testkit/testutil/sampledecks.go:101`).
- `gorgebe.Bind(cs)` adds the cards to a **private** `cards.Registry` and calls `CompileMetadata`
  (`compiled_catalog.go:192`), so they get catalog bindings as Forge's cards do.
  - It never receives a Forge card or token: `CompileMetadata` rebinds every face it is given.
  - No engine package reads a compiled ID outside `engine/cards` (checked at this base), so separate catalogs are
    safe.
- `gorgebe.Build(forge, compiled, fallbacks)` decides each card's origin as described under Design.
  `(*Registry).Deck(d)` swaps every served card in a deck, keeping order and duplicates.
- On the proof slice at `69951fbdb5`, 25 of 412 cards are served by cardc: the ones whose every rule is a bare
  keyword (measured). Every other card is served by Forge with a reason.
- Nothing outside `engine/cardc/...` and `cmd/cardc` imports gorgebe (`TestNothingImportsCardc`). So no engine
  behaviour changes, and no golden moves.

## Design

Decided.

### 1. Printer (`print.go`)

```go
// Unprintable is a construct the backend cannot print yet.
type Unprintable struct{ What, Path string }
func (e *Unprintable) Error() string { return "gorgebe: cannot print " + e.What + " at " + e.Path }

// Print prints c as gorge script text. oracle[i] is face i's Oracle text.
func Print(c *ir.Card, oracle []string) ([]byte, error)
// PrintCharacteristics prints the same text without any K: line. It fails only on the layout or the face count.
func PrintCharacteristics(c *ir.Card, oracle []string) ([]byte, error)
```

**Layouts.**
- `LayoutNormal` prints one face.
- `LayoutTransform` prints `AlternateMode:DoubleFaced` as its first line, then the faces separated by a line
  `ALTERNATE`.
- Every other layout is `Unprintable{What: "layout:<name>"}`. Only `Card` and `Transforming` occur in the proof
  slice, and only these two are checked against Forge here.
- `len(oracle) != len(c.Faces)` is `Unprintable{What: "faces"}`.

**Each face**, lines in this order (a missing value prints no line):

| line | from |
|---|---|
| `Name:<name>` | `Face.Name` |
| `ManaCost:<cost>` | `ManaCost`, or `no cost` when `HasManaCost` is false |
| `Types:<words>` | Supertypes, then Types, then Subtypes, each respelled, joined by one space |
| `PT:<p>/<t>` | when both are set |
| `Loyalty:<n>`, `Defense:<n>` | when set |
| `Colors:<list>` | when `ColorIndicator` is non-zero: `white`, `blue`, `black`, `red`, `green` in that order, joined by `,` |
| `K:<name>` | one per `AbKeyword` ability, in order (not in `PrintCharacteristics`) |
| `Oracle:<text>` | `oracle[i]`, verbatim |

**Mana symbols.**
- `ManaGeneric` prints N.
- `ManaColored` prints its letter (`W U B R G`).
- `ManaHybrid` prints its pair in the order `WU UB BR RG GW WB UR BG RW GU`.
- `ManaPhyrexian` prints the letter then `P`; `ManaHybridPhyrexian` prints the pair then `P`.
- `ManaTwoHybrid` prints `2` then the letter.
- `ManaColorless` prints `C`; `ManaX` prints `X`; `ManaSnow` prints `S`.
- Symbols are joined by one space. An empty cost with `HasManaCost` true prints `0`.

**PT.** N in decimal. A `Star` PT prints `*`, then `+N` when N > 0 or `-|N|` when N < 0.

**Measured.** On every one of the proof slice's 413 faces at `69951fbdb5`, these rules reproduce the Forge face's
`ManaCost`, `Types` order, `PT`, `Loyalty` and `Colors` exactly.

**Respelling** (`typewords.go`). A type word is printed as in this table, else unchanged:

| mtgish | gorge |
|---|---|
| `AssemblyWorker` | `Assembly-Worker` |
| `Ctan` | `C'tan` |
| `MindlessOne` | `Mindless-One` |
| `Shiar` | `Shi'ar` |
| `PowerPlant` | `Power-Plant` |
| `Urzas` | `Urza's` |
| `TimeLord` | `Time Lord` (Forge stores multiword types as separate tokens; `engine/effects/creature_types.go:13`) |

The table is complete for mtgish's `CardType`, `SuperType`, `CreatureType`, `LandType`, `ArtifactType`,
`EnchantmentType`, `PlaneswalkerType`, `SpellType` and `BattleType` unions at `1553384`. The list was measured
against gorge's type vocabulary, `predicateTypeWords` (`engine/effects/typewords.go:12`), by letters-only,
case-folded comparison. Plane names are not covered: `Planar` layouts are refused.

**Abilities.**
- `AbKeyword` whose `Keyword` has nil `N`, `Cost` and `Filter` prints `K:<Name>`. CC-10's names are gorge's `K:`
  heads (`First Strike`, `Double Strike`, `Living Weapon`, …).
- Any other keyword is `Unprintable{What: "keyword:<Name>"}`.
- Any other ability kind is `Unprintable{What: "ability:<kind>"}`.
- A face with a `Refusal` is `Unprintable{What: "refusal:<Key>"}`.

**Oracle text** comes from the Forge card of the same name, face by face, in memory.
- The engine reads `Face.Oracle`: optional-move inference (`engine/effects/zone_hand.go:787`), derived
  characteristics (`engine/rules/chars/derive.go:413`) and colour identity (`engine/cards/face.go:505`). So a card
  without it would play differently from its Forge twin for a reason that is not a card-definition difference.
- mtgish carries no Oracle text.
- The text is never written to a committed file. `cardc compile` writes it only under `.cards`, which is gitignored
  (`.gitignore:4`).

### 2. Loading (`load.go`)

```go
// ParseScript parses text, links it and applies intrinsics. Diags from the parse and the link are returned together.
func ParseScript(path string, text []byte) (*cards.Card, []cards.Diag)
// Bind gives cs catalog bindings in a private registry. Never pass Forge cards or tokens.
func Bind(cs []*cards.Card) error
```

### 3. Registry and fallback (`registry.go`, `fallback.go`)

```go
type Origin uint8
const (
    OriginCardc Origin = iota + 1
    OriginForge
)

type Entry struct {
    Name   string
    Origin Origin
    Reason string      // "" for OriginCardc; else "fallback:<class>", "refused:<Key>", "unprintable:<What>" or "parse:<first diag>"
    Forge  *cards.Card // the Forge card of this name
    Card   *cards.Card // what games seat: the cardc card, or Forge when Origin is OriginForge
    Script []byte      // the printed text, for OriginCardc
}

type Registry struct {
    forge   *cards.Registry
    entries []*Entry
    byForge map[*cards.Card]*Entry // read by key only, never ranged
}

func Build(forge *cards.Registry, compiled []*ir.Card, fb map[string]Fallback) (*Registry, error)
func (r *Registry) Entries() []*Entry                  // in compiled order
func (r *Registry) Deck(d []*cards.Card) []*cards.Card // a new slice; cards without an entry stay as they are
func (r *Registry) Served() int                        // entries with OriginCardc
```

`Build`, for each compiled card in order (a repeated name keeps its first):
1. `forge.Lookup(name)`. A miss is an error: `"gorgebe: no Forge card %q"`.
2. A name in `fb` → Forge, reason `fallback:<class>`.
3. Any face with a refusal → Forge, reason `refused:<first refusal key>`.
4. `Print(c, <Forge faces' Oracle>)`. An `*Unprintable` → Forge, reason `unprintable:<What>`.
5. `ParseScript("cardc/<name>.txt", text)`. Any diag → Forge, reason `parse:<first diag Msg>`.

Then `Bind` every card that survived, and serve them as `OriginCardc`. `Tokens` and the name universe stay Forge's:
the caller seats `forge.Tokens` and `forge.Universe()`.

**Fallback register**, `engine/cardc/testdata/fallback/` (PLAN.md section 5).
- One file per card. The stem is the card name lower-cased, with every run of characters outside `[a-z0-9]`
  replaced by `_`: `Lightning Bolt` → `lightning_bolt.txt`.
- The file holds three lines: `name <card name>`, `class <class>`, `issue <upstream issue number, or 0>`. `class` is
  one of PLAN.md's four: `mtgish-bug`, `cardc-gap`, `forge-bug`, `equivalent`.
- `ReadFallbacks(dir string) (map[string]Fallback, error)`, with `type Fallback struct{ Name, Class string; Issue
  int }`. A missing directory is an empty map, not an error. A malformed file, a bad class, or a stem that does not
  match its `name` is an error naming the file.
- The directory starts empty, and git cannot hold an empty directory. So this ticket adds no file there; CC-33
  adds the first.
- The register holds card names and class tokens only (PLAN.md section 1.4).

### 4. `cmd/cardc compile` (`cmd/cardc/compile.go`, new; one dispatch line in `main.go`'s `run`)

`cardc compile [-slice fdn|blb|legacy|decks] [-data FILE] [-out DIR]`:
1. Open Forge with `cards.OpenCorpus(<root>/.cards)` (`open.go:52`) and mtgish with `mtgish.LoadAll`.
2. Resolve the slice through `mtgish.NewIndex` (CC-02), `mtgish.Compile` each card, and call
   `gorgebe.ReadFallbacks(<root>/engine/cardc/testdata/fallback)` and `gorgebe.Build`.
3. Print one TSV line per entry, `name<TAB>origin<TAB>reason` (origin `cardc` or `forge`), then
   `total<TAB>slice=<s><TAB>cards=<n><TAB>cardc=<k><TAB>forge=<m>`.
4. Write each served card's `Script` to `-out`, default `<root>/.cards/cardc/<MTGISH ref>/scripts/<stem>.txt`,
   under `.cards`, which is gitignored.

A missing `.cards` corpus exits 1 with a message naming `make fetch-cards` and `make mtgish-fetch`.

### 5. Guards and classification

- `docs/site/architecture/packages.tiers.json`: add `"engine/cardc/gorgebe": {"tier": "engine", "status":
  "CORE"}`, sorted among the `engine/cardc/*` rows. The pre-commit hook (`.githooks/pkgmap-check`) refuses the
  commit without it.
- `internal/archtest/arch_test.go`, `TestDependencyOrderHolds` (`arch_test.go:254`): add the row
  `{module + "/engine/cardc/gorgebe", module + "/engine/cardc/mtgish"}` beside the three cardc rows
  (`arch_test.go:342-346`). The backend never reads the front end; `cmd/cardc` joins them.

## Implementation steps

1. `engine/cardc/gorgebe/doc.go` (package doc pointing at PLAN.md), `print.go`, `typewords.go`, `load.go`,
   `registry.go`, `fallback.go`.
2. The `packages.tiers.json` row and the archtest row.
3. `cmd/cardc/compile.go` and the `run` dispatch.
4. Run `go run ./cmd/cardc compile -slice fdn` and `-slice legacy`. Every name resolves, and the served count for
   the two slices together is 25.

## Tests

In `engine/cardc/gorgebe`:
- `TestPrintSkeleton`: hand-built IR compared with exact expected text.
  - A normal creature with a cost of generic 2, hybrid W/U, phyrexian G and X; a `*+1`/`*` PT; two keywords;
    `ColorIndicator` blue.
  - A transform card of two faces.
  - The unprintable cases: a split layout, a triggered ability, a `Ward` keyword with a cost, a face with a
    refusal, and too few oracle strings.
- `TestRespell`: every table row, and an unlisted word left unchanged.
- `TestParseScriptRoundTrip`: `ParseScript(Print(c))` of the normal card gives a face whose `Name`, `ManaCost`,
  `Types`, `PT`, `Colors` and `Keywords` are the printed ones, with no diags.
- `TestReadFallbacks`: fixtures written to `t.TempDir()`: one good file, a bad class, a stem that does not match its
  name, and a missing directory.
- `TestBuildOrigins`: on `testutil.CorpusRegistry(t)` (ENV when absent), with hand-built IR for `Grizzly Bears`
  (vanilla) and `Llanowar Elves` (given a triggered ability), and a fallback for `Grizzly Bears` in one sub-test
  and not in the other. Check:
  - the origins and reasons;
  - `Deck` swaps only the served card and keeps a four-of's four copies;
  - an unknown name is the error.
- `TestSkeletonMatchesForge` (corpus; needs both `.cards/mtgish` and the Forge corpus; ENV when either is absent).
  For every proof-slice card (`mtgish.SliceNames` `fdn` plus `legacy`, through the index and `mtgish.Compile`):
  `ParseScript(PrintCharacteristics(c, oracle))`, then compare each face's `Name`, `ManaCost`, `Types`, `PT`,
  `Loyalty`, `Defense`, `Colors` and `Oracle` with the Forge face. Every face must agree (measured: all 413).
- `TestServedCardsMatchForge` (corpus, as above): `Build` over the proof slice. At least 25 entries are
  `OriginCardc`. For each, every face's `Keywords` and its counts of `Abilities`, `Triggers`, `Statics` and `Repls`
  equal the Forge face's, and each `Abilities[i].API` matches. Linking and intrinsics make Prowess's trigger and a
  basic land's mana ability on both sides.

The gorgebe test files may import `engine/cardc/mtgish` and `testkit/testutil`. `TestDependencyOrderHolds` reads
`go list`'s `.Imports` and `.Deps` without `-test` (`arch_test.go:62`), so a test-only edge does not trip the new
row.

In `cmd/cardc`: `TestCompileUsage`, where `run([]string{"compile", "-slice", "nope"}, …)` exits 1, and
`TestReportOnFixture` still passes.

## Done means

- [ ] `make test-one PKG=./engine/cardc/gorgebe` passes (not ENV).
- [ ] `make test-one PKG=./cmd/cardc` passes.
- [ ] `make test-one PKG=./internal/archtest RUN='^(TestDependencyOrderHolds|TestNothingImportsCardc|TestPublicBoundary|TestNoVerbatimForgeLines)$'` passes.
- [ ] `go run ./cmd/pkgmap -check` exits 0.
- [ ] No `make test-targeted` or `make test-smart` per ticket: one `make test-smart BASE=origin/cardc` runs at the
  wave-2 head.

## Out of scope

- Printing any non-keyword ability, keyword argument, or layout beyond normal and transform (CC-21..23).
- Lockstep (CC-31). The static diff (CC-30).
- Any edit to `engine/cards` or any engine package: `cacheVersion` and `CompilerFingerprint` stay as they are
  (PLAN.md section 3).
- Committing any generated script, any Oracle text, or any test fixture copied from Forge or mtgish data. Test
  scripts are synthetic, and the real-card tests read the corpus at run time.
- Anything on GitHub outside gorge. Never open a pull request to i5jb/mtgish.

## Collisions

A hub. It creates `engine/cardc/gorgebe/*` and `cmd/cardc/compile.go`, and edits one line of `cmd/cardc/main.go`,
one row of `packages.tiers.json` and one row of `arch_test.go`. It runs in parallel with CC-15a..16b, which touch
only `engine/cardc/ir` and `engine/cardc/mtgish`. CC-31 depends on it.
