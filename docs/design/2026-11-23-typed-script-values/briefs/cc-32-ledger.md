# feat(cardc): CC-32 the divergence ledger, blame and its ratchet

Kind: feature

Target branch: `cardc`. Complexity: medium. Implementer: Sonnet.
Design of record: `docs/design/2026-11-23-typed-script-values/PLAN.md` on `cardc`, section 4 ("Divergence
classes", "Pass bar"). Runs after CC-30 (`engine/cardc/diff`, `engine/cardc/slice`) and CC-31 (`engine/cardc/lockstep`).

## Goal

Hold every live divergence of the proof slice in a committed, per-card ledger, both ways:
1. every static-diff row (CC-30) of a served card;
2. every lockstep game that is not clean, blamed on one served card.

A new divergence, or a ledger row that no longer happens, fails a test. Seed the ledger at this head and triage what
the evidence below supports. Add `cardc ledger`.

## Workspace facts (verified at `de85384ff1` plus CC-24; CC-30's API from its own spec)

- `lockstep.Run(g, dir, swap, newBot)` (`engine/cardc/lockstep/lockstep.go:49`) returns a `Result` with `Outcome`
  `clean`, `diverged`, `rejected` or `unfinished`. `lockstep.FDN(root, forge, from, n)` and `lockstep.Legacy(...)`
  (`slices.go:23`, `:153`) seat the games; `cmd/cardc/lockstep.go` is the caller recipe (bot:
  `botpolicy.NewGameBot(seed)`).
- At CC-24's head, `cardc lockstep -slice fdn -n 1500` gives `games=3000 clean=1953 diverged=1026 rejected=21`, in
  about 60 s on one core (a game plus its replay is about 20 ms). Legacy at 2 and 4 seats: every game diverges.
- `diff.Card(name, cardc, forge) []Row`, `Row.Key()`, `Row.Sig()` and `slice.Build(root, forge, "fdn", "legacy")`
  are CC-30's. Read `engine/cardc/diff` and `engine/cardc/slice` first.
- `gorgebe.Stem(name)` (`engine/cardc/gorgebe/fallback.go`) is the per-card file stem the fallback register uses.
- The compliance mtgish ledger, read-only and outside this repo: `~/projects/gorge-compliance/oracle/mtgish/known/T1.tsv`
  .. `T4.tsv`, columns `card tier field gorge mtgish class note`, classes `forge-bug`, `gorge-compile`,
  `mtgish-bug`, `equivalent`, `digital-variant`. gorge never imports or vendors it; the triage reads it.

## Design

Decided. The seat makes no design calls; anything this ticket does not cover is out of scope.

### 1. Blame (`engine/cardc/lockstep/blame.go`)

```go
// Only restricts swap to one Forge card: every other card keeps its Forge pointer.
func Only(swap Swap, forge *cards.Card) Swap

// Blame names the served card a non-clean result is due to: for each distinct card the swap changes in g, in
// deck order (seat 0 first, then by position), it runs g in dir with Only(swap, card), and returns the name
// (Faces[0].Name) of the first whose result is not clean. "" when none is (the divergence needs two cards).
func Blame(g Game, dir Direction, swap Swap, newBot func(seed uint64) rules.Answerer) string
```

A clean result is never blamed. Blame is deterministic: same game, same answer.

### 2. The ledger (`engine/cardc/lockstep/ledger.go`)

One file per card, `engine/cardc/testdata/ledger/<gorgebe.Stem(name)>.tsv`:

```
# <card name>
source	key	sig	class	issue	note
```

- `source`: `static` or `lockstep`.
- `static` rows: `key` is `diff.Row.Key()`, `sig` is `diff.Row.Sig()`. One row per live diff row.
- `lockstep` rows: `key` is a witness, `<game id>/<direction>` (`fdn/13/forward`, `legacy/4/reverse`), `sig` is
  `-`. A card blamed anywhere needs at least one lockstep row; it may carry more, each a live witness.
- `class`: `mtgish-bug`, `cardc-gap`, `forge-bug` or `equivalent` (PLAN.md section 4).
- `issue`: the upstream mtgish issue number, or `0` (none filed). Only `mtgish-bug` rows may be non-zero.
- `note`: free text of 100 characters or fewer in our own words; never card text, never a script fragment.
- Rows sorted by source then key; no duplicate (source, key). The first line names the card exactly; its stem must
  match the file name.

Two-card divergences (Blame returns "") go to `engine/cardc/testdata/ledger-unattributed.tsv`, columns
`witness class note`, held both ways like the rest.

```go
type Row struct{ Card, Source, Key, Sig, Class string; Issue int; Note string }
func ReadLedger(dir string) ([]Row, error)       // every file; a malformed one is an error naming it
func WriteLedger(dir string, rows []Row) error   // one file per card; removes files of cards with no rows
// Hold returns the live rows with no ledger row (missing) and the ledger rows with no live row (stale), matched on
// (Card, Source, Key) and, for static rows, Sig.
func Hold(live, ledger []Row) (missing, stale []Row)
```

### 3. Counts ratchet

`engine/cardc/testdata/ledger-counts.tsv`: one line per class, `class<TAB>rows<TAB>cards`, all four classes listed.
`TestLedgerCounts` requires the ledger's counts to equal it exactly, so every change to the ledger shows its effect
in review. The pass bar (PLAN.md section 4) is `cardc-gap` and `equivalent` both 0.

### 4. Seeding and triage

`cardc ledger [-write]` (`cmd/cardc/ledger.go`, new; dispatch and usage lines in `main.go`):
1. `slice.Build(root, forge, "fdn", "legacy")`; static rows over every served card;
2. lockstep over all 1,500 FDN pairs and Legacy at 2, 4, 6 and 8 seats, both directions, on 4 workers (results
   gathered in game order, so output never depends on scheduling); each non-clean result is blamed;
3. without `-write`: print `missing` and `stale` rows and a totals line, exit 1 on any; with `-write`: rewrite the
   ledger, **keeping** class, issue and note of every row whose (card, source, key) survives, giving new rows
   class `cardc-gap`, issue `0`, note `untriaged`, keeping at most the first two lockstep witnesses per card (game
   order, forward before reverse), and rewriting `ledger-counts.tsv`.

Then triage the seeded rows, card by card, with only this evidence:
- **`mtgish-bug`**: the compliance ledger has a `mtgish-bug` row for the same card whose note names the same
  symptom (up-to read as exact: mtgish#3; morph face-up trigger lost: mtgish#1; haste dropped: mtgish#2), or you
  read the card's mtgish tree and the IR and can name the wrong node. Issue number when the note gives one, else 0.
- **`forge-bug`**: the compliance ledger has a `forge-bug` row for the card with the same symptom.
- **`equivalent`**: a static row whose two values mean the same in gorge (cite the reader: a `file:line` in
  `engine/effects` or `engine/rules` in the note's place in your hand-back, not in the file).
- Everything else stays `cardc-gap` / `untriaged`. Do not fix cards in this ticket.

### 5. Registry seating for the tests

Tests build the proof registry once per package (`sync.Once`) with `slice.Build`, and the bot is
`botpolicy.NewGameBot(seed)` answering as `cmd/cardc/lockstep.go` does.

## Tests

In `engine/cardc/lockstep` (corpus tests skip through `testutil.CorpusRegistry` and `slice.ErrNoData`, so a missing
corpus is ENV):
- `TestBlame`: CC-31's synthetic Zz Bear game: the 3/3 swap is blamed on `Zz Bear`; the identity swap blames "".
- `TestLedgerFormat` (offline): every ledger file parses; stem matches the name line; classes and issues valid; no
  duplicates; sorted; every card is in the FDN or Legacy slice name lists (`mtgish.SliceNames`).
- `TestLedgerStatic` (corpus): `Hold` over the static rows is empty both ways.
- `TestLedgerLockstepFDN00` .. `TestLedgerLockstepFDN14` (corpus): pairs `[100k, 100k+100)`, both directions, on 4
  goroutines; every non-clean result's blamed card has a lockstep row (any witness), and every "" blame is in the
  unattributed register. Each test well inside one minute; measure and report the slowest.
- `TestLedgerLockstepLegacy` (corpus): the same at 2, 4, 6 and 8 seats (split into two tests if it nears 30 s).
- `TestLedgerWitnessesLive` (corpus): each lockstep row's witness game, run in its direction, is not clean, and
  `Blame` names that row's card; each unattributed witness is not clean and blames "".
- `TestLedgerCounts` (offline): section 3.

In `cmd/cardc`: `TestLedgerUsage`: an unknown flag exits non-zero.

## Done means

- [ ] `make test-one PKG=./engine/cardc/lockstep RUN='^(TestBlame|TestLedgerFormat|TestLedgerStatic|TestLedgerLockstepFDN[0-9]+|TestLedgerLockstepLegacy.*|TestLedgerWitnessesLive|TestLedgerCounts|TestSwapIsNeutral|TestHarnessFindsDivergence|TestFDNSliceLoads)$'` passes (not ENV).
- [ ] `make test-one PKG=./cmd/cardc RUN='^(TestLedgerUsage|TestDiffUsage|TestLockstepUsage)$'` passes.
- [ ] `make test-one PKG=./internal/archtest RUN='^(TestDependencyOrderHolds|TestNothingImportsCardc|TestPublicBoundary|TestNoVerbatimForgeLines)$'` passes.
- [ ] `go run ./cmd/cardc ledger` exits 0 at the ticket head.
- [ ] The hand-back gives `ledger-counts.tsv`, the number of blamed cards, the unattributed count, the slowest
  ledger test's wall time, and for each class other than `cardc-gap` the cards and the evidence used.
- [ ] No `make test-targeted` or `make test-smart` per ticket: one `make test-smart` runs at the wave-4 head.

## Out of scope

- Fixing any card, respelling gorgebe output, or adding canonicalisations (burn-down, CC-34..39).
- Fallback files and fork commits (CC-33a/b). Filing upstream issues (the operator files them, by hand).
- Any edit to `engine/replay`, `engine/rules`, `engine/cards` or an engine package; any edit to `engine/cardc/ir`,
  `mtgish` or `gorgebe` non-test code.
- Committing Oracle text, script text, diff values or compliance rows' token columns.
- Anything on GitHub outside gorge. Never open a pull request to i5jb/mtgish.

## Collisions

New `blame.go`, `ledger.go` and tests in `engine/cardc/lockstep`; new `engine/cardc/testdata/ledger/`,
`ledger-unattributed.tsv`, `ledger-counts.tsv`; new `cmd/cardc/ledger.go`, one dispatch and one usage line in
`main.go`. Runs alone after CC-30; CC-33b follows it.
