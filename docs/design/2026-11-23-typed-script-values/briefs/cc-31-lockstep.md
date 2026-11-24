# feat(cardc): CC-31 the lockstep harness (forward and reverse replay across card sources)

Kind: feature

Target branch: `cardc`. Complexity: medium. Seat: Sonnet.
Design of record: `docs/design/2026-11-23-typed-script-values/PLAN.md` on `cardc`, section 4 ("Lockstep"). Runs
after CC-20 (`cc-20-gorgebe-skeleton`).

## Goal

Create `engine/cardc/lockstep`: play a game on one set of card definitions, replay its recorded intents on another,
and report the first divergence. Prove first that swapping **identical** definitions is neutral (PLAN.md marks that
INFERRED). Add `cardc lockstep` and `make cardc-lockstep`.

## Behaviour

- **Forward:** play on Forge's cards, then `replay.Replay` (`engine/replay/replay.go:62`) the log with every deck
  swapped through a `Swap`.
- **Reverse:** play on the swapped decks, then replay on Forge's.
- A clean replay returns nil. A `*replay.Divergence` (an alias of `eventdiff.Divergence`, whose `Seq` is the first
  differing event, `engine/eventdiff/eventdiff.go:17`) is a divergence at that Seq. Any other error is `rejected`:
  the replayed engine refused a recorded intent.
- `Replay`'s doc asks for "the same *cards.Card pointers" (`replay.go:10`). The swap is outside that contract on
  purpose, and `TestSwapIsNeutral` is what makes it safe.
  - **If `TestSwapIsNeutral` fails, stop and report BLOCKED** with the first divergence (game, direction, Seq,
    Want and Got events).
  - Do not loosen the comparison, filter events, or special-case cards. The plan's lockstep premise would be wrong,
    and that is the operator's call.
- The package seats no bot of its own (engine packages import nothing outside `engine/`). The caller passes the bot
  factory, as `rules.PlayAcceptance` takes it (`engine/rules/acceptance_game.go:97`).

## Design

Decided.

### 1. Package API (`engine/cardc/lockstep/lockstep.go`, new)

```go
type Direction uint8
const (
    Forward Direction = iota + 1 // play on Forge definitions, replay on swapped
    Reverse                      // play on swapped, replay on Forge
)
func (d Direction) String() string // "forward", "reverse"

// Swap maps a Forge-resolved deck to the deck to compare against. gorgebe.Registry.Deck is one.
type Swap func([]*cards.Card) []*cards.Card

// Game is one seated game on Forge definitions.
type Game struct {
    ID  string       // "fdn/<line>" or "legacy/<seats>"
    Cfg rules.Config // Decks hold Forge cards
}

type Result struct {
    ID      string
    Dir     Direction
    Swapped int    // distinct cards whose pointer the swap changed
    Outcome string // "clean", "diverged", "rejected" or "unfinished"
    Seq     uint64 // the first differing event, when diverged
    Detail  string // the error text, when not clean
}

// Run plays g in direction dir with newBot answering, replays the log, and reports.
func Run(g Game, dir Direction, swap Swap, newBot func(seed uint64) rules.Answerer) Result
```

`Run`:
1. Copy `g.Cfg` twice: `forge` as given, and `swapped` with `Decks[i] = swap(Decks[i])`. Count `Swapped` over every
   deck by pointer.
2. Forward plays `forge` and replays on `swapped`; Reverse plays `swapped` and replays on `forge`.
3. Play with `rules.PlayAcceptance(cfg, newBot, nil)` (`acceptance_game.go:97`): the same loop, starting-player ask
   and intent cap as the acceptance games.
   - A play error is `unfinished` with Detail. Swapped decks can stall where Forge's do not, and that is a finding,
     not a harness error.
4. Replay with `replay.Replay(e.L, other)`. Use `errors.As` to read a `*replay.Divergence`, giving `diverged` with
   `Seq`. Any other error is `rejected`.

### 2. Slices (`slices.go`, new)

```go
// FDN seats pairs [from, from+n) of dirac/oracle/testdata/slices/fdn/pairs.tsv.
func FDN(root string, forge *cards.Registry, from, n int) ([]Game, error)
// Legacy seats the acceptance game at seats seats from already-resolved decks.
func Legacy(forge *cards.Registry, names []string, decks [][]*cards.Card) Game
```

**FDN.**
- `pairs.tsv` lines are `deckA<TAB>deckB<TAB>seed` (1,500 lines at this base).
- The decks are `decks/<name>.json` inside `decks.tar.gz` in the same directory. Read them with `archive/tar` and
  `compress/gzip`, then parse with `deck.Parse` and `File.Resolve(forge)` (`engine/deck/deck.go:190`, `:280`):
  main deck only, as PLAN.md's slice counts.
- Files are read by path from `root`. `lockstep` does not import `dirac/...`; nothing in `engine/` may.
- The Config is:

  ```go
  rules.Config{Seed: seed, Names: []string{A, B}, Decks: [][]*cards.Card{a, b}, Tokens: forge.Tokens,
      NameUniverse: forge.Universe(), Mulligans: 1, LoopGuard: rules.WithDefaultObjectCap(nil)}
  ```

  `rules.WithDefaultObjectCap` is at `engine/rules/livelock.go:155`. ID is `fdn/<0-based line>`.
- An out-of-range `from` or `n` is clamped; a missing file is an error naming it.

**Legacy.** `rules.AcceptanceConfig(forge, names, decks)` (`acceptance_game.go:49`), with ID `legacy/<seats>`.
Callers seat the decks with `testutil.AcceptanceDecks(forge, seats)` (`testkit/testutil/decks.go:94`), in tests and
in `cmd/cardc`, never in `lockstep` itself.

Both directions seat Forge's `Tokens` and `NameUniverse`. Only deck card definitions are swapped.

### 3. `cmd/cardc lockstep` (`cmd/cardc/lockstep.go`, new; one dispatch line in `main.go`'s `run`)

`cardc lockstep -slice fdn|legacy [-from N] [-n M] [-seats S] [-dir forward|reverse|both]`. Defaults: `-from 0
-n 100 -seats 2 -dir both`.
1. Build the cardc registry exactly as `cardc compile` does (CC-20) over the slice's cards.
2. `Swap` is `reg.Deck`. The bot is `botpolicy.NewGameBot(seed)`, answering `b.Answer(e.G, e, d)`, as
   `rules.PlayAcceptance`'s doc shows (`acceptance_game.go:24-29`).
3. Print one TSV line per result, `id<TAB>dir<TAB>swapped<TAB>outcome<TAB>seq<TAB>detail`, then
   `total<TAB>games=<n><TAB>clean=<c><TAB>diverged=<d><TAB>rejected=<r><TAB>unfinished=<u>`.
4. Exit 0 whatever the outcomes. The ledger and its ratchet are CC-32.

`cardc.mk` gains:

```make
SLICE ?= fdn
.PHONY: cardc-lockstep
# Every FDN pair (or the Legacy acceptance seats) through cardc lockstep, in chunks of 100.
cardc-lockstep:
	@if [ "$(SLICE)" = legacy ]; then for s in 2 4 6 8; do go run ./cmd/cardc lockstep -slice legacy -seats $$s || exit 1; done; \
	else for f in $$(seq 0 100 1400); do go run ./cmd/cardc lockstep -slice fdn -from $$f -n 100 || exit 1; done; fi
```

### 4. Classification

`docs/site/architecture/packages.tiers.json`: add `"engine/cardc/lockstep": {"tier": "engine", "status":
"TOOL"}`, sorted among the `engine/cardc/*` rows. The pre-commit hook refuses the commit without it.

## Implementation steps

1. `engine/cardc/lockstep/doc.go`, `lockstep.go`, `slices.go`, and the tiers row.
2. `cmd/cardc/lockstep.go`, the dispatch line, and the `cardc.mk` target.
3. Tests. Run `go run ./cmd/cardc lockstep -slice fdn -n 5` and record its totals line in the hand-back.

## Tests

In `engine/cardc/lockstep`. The tests import `bots/botpolicy`, `testkit/testutil` and `engine/cardc/gorgebe`.
These are test-only edges: `engine/rules/testbot_test.go` imports botpolicy the same way. Every corpus test skips
through `testutil.CorpusRegistry`, so a missing `.cards` is ENV.
- `TestSwapIsNeutral` (corpus): the identical swap. For each distinct card in the decks:
  1. read its script with `os.ReadFile(c.Path)` (`Card.Path` is the absolute script path, `engine/cards/ir.go:620`);
  2. `gorgebe.ParseScript(c.Path, text)`, then `gorgebe.Bind` all of them;
  3. map the Forge pointer to the copy.

  Run Forward and Reverse on FDN pairs 0-4 and on the 2-seat Legacy game. Every result must be `clean` with
  `Swapped > 0`. On failure, stop and report as Behaviour says.
- `TestHarnessFindsDivergence`: a synthetic two-seat game. Each deck is 40 Forests (`forge.Lookup("Forest")`) and
  20 copies of `Zz Bear`, `gorgebe.ParseScript`'d from `Name:Zz Bear\nManaCost:G\nTypes:Creature Bear\nPT:2/2\n`.
  The swap maps `Zz Bear` to a copy printed with `PT:3/3`. Forward is `diverged` with `Seq > 0`, Reverse too, and
  the identity swap is `clean` both ways.
- `TestFDNSliceLoads` (corpus): `FDN(root, forge, 0, 1500)` returns 1,500 games, each with two non-empty decks and
  the pair's seed; `FDN(root, forge, 1490, 100)` returns 10.
- Budget: each test stays well inside one minute. If `TestSwapIsNeutral` nears it, cut to pairs 0-2, never below.

In `cmd/cardc`: `TestLockstepUsage`, where `run([]string{"lockstep", "-slice", "nope"}, …)` exits 1.

## Done means

- [ ] `make test-one PKG=./engine/cardc/lockstep` passes (not ENV), `TestSwapIsNeutral` included.
- [ ] `make test-one PKG=./cmd/cardc` passes.
- [ ] `make test-one PKG=./internal/archtest RUN='^(TestDependencyOrderHolds|TestNothingImportsCardc|TestPublicBoundary)$'` passes.
- [ ] `go run ./cmd/pkgmap -check` exits 0.
- [ ] The hand-back quotes the `cardc lockstep -slice fdn -n 5` totals line.
- [ ] No `make test-targeted` or `make test-smart` per ticket: one `make test-smart BASE=origin/cardc` runs at the
  wave-2 head.

## Out of scope

- The divergence ledger, its classes and the ratchet (CC-32).
- `cmd4`, the four-seat Commander pod (PLAN.md stage 2).
- Any edit to `engine/replay`, `engine/rules` or any engine package.
- Anything on GitHub outside gorge. Never open a pull request to i5jb/mtgish.

## Collisions

Depends on CC-20, whose registry it seats. Its files are new apart from one dispatch line in `cmd/cardc/main.go`
(CC-20 adds the line above it), the `cardc.mk` target and one `packages.tiers.json` row. It runs in parallel with
CC-15a..16b.
