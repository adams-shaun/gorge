# dirac

**dirac** is a second Magic rules engine, written to be fast enough to sit
inside a search loop. A whole game is one `dirac.State`, copied with `CopyFrom`, allocating nothing per
transition. It is a `Step`/`Apply` state machine over a `kop.Table` of cards, for two to four seats,
and it is judged against gorge, the rules engine in [`engine/`](../engine): gorge leads and dirac follows,
decision by decision (see [verification](docs/verification.md)). That speed is the point: self-play,
search and policy training run millions of transitions.

It is **not** a card-database front end or a replacement for gorge. One build
holds four seat slots (`MaxSeats`) and plays a two-seat game or a pod of three
or four seats from a fixed card table, supports the mechanics the generated
tables cover, and leaves everything else to gorge. `NewConfig` and `Init` start
two seats; `InitN` with `cardset`'s `MatchupN` starts a pod. Pods, Commander
pods included, play in lockstep with gorge like two-seat games
([verification](docs/verification.md#four-seat-pods)). A card whose lowering
reads the one opponent of a two-seat game (`kop.CardTwoSeat`) makes dirac refuse
a game of three or more seats at its start (`StallTwoSeat`): a declared limit,
counted, not a divergence. A server reaches dirac through `dirac/adapter`, which hosts
two seats ([serving](docs/serving.md)).

## Quick start

Start a game from a generated card set, then answer decisions until the game is
over. This is the loop from [`dirac/doc.go`](doc.go), with the first option of
every decision as the policy:

```go
import (
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

set, _ := cardset.ByName("fdn")
deck := []cardset.Entry{{Name: "Plains", Count: 20}}
tbl, deckA, deckB, _ := set.Matchup(deck, deck)
s := dirac.NewConfig(tbl, deckA, deckB, 1, dirac.Config{Mulligans: 2})
for {
	d := s.Step()            // or s.StepInto(&d): no Decision copy
	if d.Kind == dirac.DkNone {
		break
	}
	s.Apply(d.Options[0])    // one of d.Options[:d.N]
}
over, winner := s.Terminal()
```

`cardset.ByName` reads the binary card tables `make dirac-tables` writes under the
corpus, and returns an error naming that command when they are missing.

`dirac/example_test.go` runs this loop to the end on the hand-written
`dirac/vanilla` table, which needs no tables, and prints the winner as a checked
`Example`. `doc.go` has the full godoc: the API, projection layout and
the gorge rules dirac follows.

## Concepts

| Name | What it is |
|---|---|
| `State` | The whole game. `dst.CopyFrom(src)` clones a position; `New` and `NewConfig` return a pointer, and `Init` starts a game in an existing State. `go vet` rejects an assignment. |
| `Decision` | One ask from the engine. `Step()` returns a `Decision`; `StepInto(&d)` fills a caller's value without the copy. `DkNone` means the game is over. `Options[:d.N]` are the legal answers; `d.Player` is who answers and `d.Sub` says what a `DkChoose` chooses. |
| `Action` | One packed `uint32` answer: `kind<<26 | source<<17 | target<<8 | mode`, with nine-bit source and target codes (`seats.go`). See `types.go` for `ActionKind` and each kind's fields; `MakeAction`, `Kind()`, `Source()`, `Target()` and `Mode()` encode and decode it. |
| `Obj` | A card instance index. Objects are laid out deck by deck in seat order (seat 0's deck in deck order, then seat 1's, and so on), then the sideboards, then tokens in creation order, up to `MaxObj` (`seats.go`). An object keeps its index across zones. |
| `Projection` | `Project(&p)` writes a comparable, fixed-array view: objects by `Obj` index, stack, life, turn, step, active player, mana pools, zone sizes, game over and winner. It is the oracle's comparison surface and a convenient read-only view for tools. |
| `Config` | `NewConfig(tbl, deckA, deckB, seed, Config{...})`. `Mulligans` is the London mulligan limit (0 skips it); `NoElide` poses single-option decisions instead of answering them in place. The other fields are oracle/search knobs documented in `state.go`. |
| `Terminal` | `Terminal() (over bool, winner int8)`. `winner` is the winning seat, or -1 for a draw. |
| Stall | A capacity stop: a pool filled or a bound was hit, so the game ends as a draw with a reason instead of corrupting state. `StallReason()` returns the reason byte and `StallName(reason)` its short name. The capacity reasons are `Stall*` constants in `gen.go`; per-mechanic ones live beside their mechanic. `stallNames` is the name table in `gen.go`, filled by per-reason `init` functions. A caller sees a stall as `Terminal() == (true, -1)` plus a non-zero `StallReason()`. |

Other useful entry points: `Turn()`, `Life(p)`, `ActivePlayer()`, `StepNow()`,
`ObjCard/ObjZone/ObjOwner/ObjToken`, and the epoch-based `ProjectDelta` for
incremental views. Search and bots use the same API.

### Answering a decision

A useful policy switches on `d.Kind`, not just the option order:

| `d.Kind` | Answers |
|---|---|
| `DkPriority` | `AkPass`, `AkPlayLand`, `AkCast`, `AkActivate` (and the special actions: `AkTurnFaceUp`, `AkPlot`, ...). |
| `DkAttackers` | `AkAttack` per attacker/defender pair, then `AkDone`. |
| `DkBlockers` | `AkBlock` per blocker/attacker pair, then `AkDone`. |
| `DkTarget` | `AkTarget` for one object or player; `AkDone` closes an optional up-to target list. |
| `DkModes` | `AkMode` with the chosen mode index. |
| `DkChoose` | `AkChoose`; `d.Sub` says whether the mode is a number/colour/yes-no and whether `source` names a card. |
| `DkTriggerOrder` | `AkOrder` for one pending trigger. |
| `DkMulligan` | `AkKeep` or `AkMulligan`. |

With the default `Config`, decisions that have exactly one option are answered
in place (counted in `State.Skipped`) and never returned by `Step`. Set
`Config.NoElide` when an observer must see every one, as the oracle does.

## Rules for code that touches `State`

These are invariants, not style preferences:

- **`State`'s core is a plain value.** `stateCore` holds no pointers, maps,
  slices, strings or interfaces; a field that needs a collection is a fixed array
  plus a count, and rarely used arrays sit in the cold buffer, sized to the table:
  the feature's own file registers each one with `newCold` and reads it with
  `.of(s)`, never by editing `statecopy.go` or `arena.go`.
  Clone with `CopyFrom`, and run a rules probe on `s.fork()`, paired with
  `s.unfork()`.
- **No allocation in the transition loop.** `Step`/`Apply` and the interpreter
  must not allocate per action; scratch space lives in `State` or a caller's
  reused value.
- **Determinism.** No wall clock and no ambient randomness. Inside the engine,
  the only RNG is `State.rng` (`rng.go`), seeded from `NewConfig`; draw counts
  are part of the projection. Same seed and decisions = the same game, byte for byte.
- **No map iteration whose order reaches an action, option list, record or
  output file.** Fixed arrays, sorted keys or explicit order only.
- **Fixed capacities.** `MaxDeck`, `MaxObj`, `MaxPerm`, `MaxStack`,
  `MaxPending`, `MaxHand`, `MaxDelayed` and `MaxOptions` bound the value; a
  full pool is a stall, never a silent drop.
- **Objects keep their `Obj` index across zones.** A zone change bumps a
  private version, so a stack target that left and came back is illegal
  (CR 400.7).

## Import boundary

`dirac` and its fast packages must not import gorge's packages. Only the oracle seam, the generator and
the SpellBench v2 engine may.

| Package | May import gorge's packages? |
|---|---|
| `dirac`, `dirac/kop`, `dirac/cardset`, `dirac/vanilla`, `dirac/search/...`, `dirac/feat` | No. `dirac/cardset` imports only the module root's table key, which imports the standard library. |
| `dirac/neteval` | Only `bots/netsimd`, `bots/gonet` and `bots/policynet`, the networks and their input types; never the engine. |
| `dirac/oracle` | Yes: it runs gorge as the oracle. |
| `dirac/kgen` | Yes: it lowers gorge's card IR into `kop` tables. |
| `dirac/adapter` | Yes, gorge's view, decision and card types, and never `engine/rules` itself, `engine/replay` or `dirac/oracle`: it serves a game with no rules engine built. |
| `dirac/cmd/sbv2dirac` | Yes: it runs gorge as the lockstep shadow. |
| `dirac/cmd/dplay` | Yes: it reads gorge's deck files, and its bots read gorge's compiled cards through `dirac/oracle`. |
| `dirac/cmd/kmanifest` | Only the module's `cli` package: it reads the manifest and writes source. |

A new import in the no column is a review failure even if the tests pass.

## Package map

| Path | What it is |
|---|---|
| `dirac` | The engine: state, `Step`/`Apply`, combat, stack, mulligans, projection, and the hand-written interpreter for the `kop` ops. Start at `doc.go`. |
| `dirac/kop` | The card-effect vocabulary: `Table`, `CardDef`, `Ability`, `Instr` and the op enum that card tables and the interpreter share. |
| `dirac/cardset` | The engine-neutral `Set` interface and the loader of binary card tables (`ByName`, `Names`). It imports `dirac/kop` and the root package's table key. |
| `dirac/cardset/pools` | One directory per card pool, with its committed inputs, `cards.txt` and `ids.tsv`. `fdn`, `blb` and `pauper` are full-set pools; the pools under `fixtures/` exercise one mechanic at a time. |
| `dirac/oracle` | The engine-neutral seam to gorge: `Side`, `GorgeSide`, `RealKernel`/`FDNSide`, `Lockstep`, `Fuzz`, `Replay`, `Repro` and the policies. "Lockstep" means both engines get the same decks and seed and every decision and projection is compared. Its commands are `dirac/oracle/cmd/kfuzz` (lockstep fuzzer), `dirac/oracle/cmd/khbot` (dirac heuristic-bot parity) and `dirac/oracle/cmd/ksbt` (dirac sb-tactical parity). |
| `dirac/adapter` | A dirac game behind `engine/game`: what `gorged -engine dirac` serves, with a log and views derived from positions. See [serving](docs/serving.md). |
| `dirac/canon` | The canonical vocabulary the oracle and the adapter share, and dirac's translation into it. |
| `dirac/support` | Which cards are supported: the `evidence` and `refused` lists. |
| `dirac/search` | gorge's `bots/azmcts` PUCT search ported onto `dirac.State`, with its self-play, instant-timing, land, belief and per-creature options, each off by default. Start at `doc.go`. |
| `dirac/neteval` | The search's network `Evaluator`: `dirac/feat` feeding gorge's `bots/netsimd` (or `bots/gonet`), with an exact evaluation cache. |
| `dirac/oracle/searchtest` | Tests only: the lockstep checks that need `dirac/search` or `dirac/neteval`, which `dirac/oracle` may not import. |
| `dirac/search/hbot` | The native port of gorge's auto-pay heuristic bot. |
| `dirac/search/mull` | gorge's London-mulligan policies, as pure functions over a hand. |
| `dirac/feat` | The network feature encoder: dirac position to the same `policynet` inputs gorge produces. |
| `dirac/vanilla` | A hand-written table of basic lands, vanilla creatures and simple spells, with four synthetic 40-card decks. The small-table test bed. |
| `dirac/crcoverage` | Test-only CR coverage audit (`README.md` plus one file per CR chapter); `TestCRCoverageIsCurrent` checks every row against the code. |
| `dirac/<topic>_<mechanic>.go` | One file per mechanic. `doc.go` lists the topics. A contributor adds a new file instead of appending to a shared one. |

Commands: `dirac/cmd/kgen` writes the binary tables; `dirac/cmd/dplay` plays games on dirac alone;
`dirac/cmd/sbv2dirac` serves dirac as a SpellBench v2 engine. See [running](docs/running.md).

## Where next

- [Cards](docs/cards.md): how card tables are generated, what is
  supported, and how to check a card.
- [Verification](docs/verification.md): lockstep, `kfuzz`, the parity
  commands, CR coverage and following a gorge change.
- [Running](docs/running.md): `kfuzz`, the Go API, bots, the SpellBench v2
  engine and the benchmarks.
- [Serving](docs/serving.md): `gorged -engine dirac`, what it plays and refuses.
- [Support](docs/support.md): how many cards are supported, refused or unverified,
  per Standard set.

Deeper reference: [`dirac/doc.go`](doc.go) (the full API and gorge-rule
notes), [`DEVIATIONS.md`](docs/DEVIATIONS.md) and
[`GORGE-FINDINGS.md`](docs/GORGE-FINDINGS.md) (differences from gorge),
[`docs/kgen-params.md`](docs/kgen-params.md) (the lowering census) and
[`crcoverage/README.md`](crcoverage/README.md) (CR rows).

## Registering an op, stall or ext sub-op (contributors)

Add to a new file plus at most one line per table; never append to a shared tail.
- A vocabulary code (a counter kind, keyword `KwID`, ext sub-op, `Count$` head `ExprKind`, trigger, static or replacement mode, zone, State `key`) comes from the namespace manifest, `dirac/manifest/<Family>/<Name>`, one file per code ([`manifest/doc.go`](manifest/doc.go)). Most are pre-mapped: `go run ./dirac/cmd/kmanifest show TICKET` finds the entry and its constant, which already exists (`kop/manifest_gen.go`). Use it, and change the entry's `status: reserved` line to `status: active`, nothing else. A code the manifest lacks is a new entry with `value: next` (landing assigns the number), then `go run ./dirac/cmd/kmanifest gen` and `KOP_ORDINALS_REGEN=1 go test -run TestKopOrdinalsUnchanged ./dirac/kop`. `TestManifestStatusMatchesCode` fails on an active entry the kernel never references, or a reserved one it does; kgen refuses a card that carries an inactive code (`manifest:<status>:<Family>/<Name>`). Never add a hand constant to a manifest family, and never append to an iota list: each one ends at a frozen sentinel (`OpCount`, `TrigEventCount`, `ReplCount`, `StatCount`, `ExCount_`, `CondCount_`, `CtrM2M1`/`NumCounterKinds3`).
- Stall reason: `const StallX = N` (N unused, literal) and `func init() { stallNames[StallX] = "x" }` in your mechanic's file. `NumStall` is a fixed capacity; do not edit it.
- Ext sub-op (`kop`): a manifest entry `dirac/manifest/ExtOp/ExtX` (the generator writes the constant and `extOpNames[ExtX] = "X"`). Its handler, a `func (s *State) x(k *prep, in *kop.Instr) (ok, done bool)` in your feature file, carries `//kmanifest:ext ExtX` in its doc comment; `go run ./dirac/cmd/kmanifest gen` dispatches it from `execExt`'s default arm (`execExtGen`, `manifest_gen.go`). Do not add a case to `execExt`.
- Table-prep fact (a feature's "this table uses X" flag): `var useX = newGenUse(func(t *kop.Table) bool {...})` in your file, read as `k.gen.uses.has(useX)`; or `func init() { genNeed(func(t *kop.Table) bool {...}) }` when the table only needs a `genPrep` (`genneed.go`). Do not add a term to `prepareGen`'s gate or a bool to `genPrep`.
- Arena mechanic (a per-slot or per-object mark few pools use): a `dirac/manifest/mech/mechX` entry (`value: next`, `state: slotbit 16/16` or `objbit 32/64`, `status: active`), then `go run ./dirac/cmd/kmanifest gen` (the constant, `mechWidth`, `mechName` and `numMech` are generated into `manifest_gen.go`), and `func init() { arenaDetect(mechX, func(t *kop.Table, in *kop.Instr) bool {...}) }` in your file. Do not edit `arena.go`.
- `TestRegistryNamesComplete` (missing name) and `TestEnumValuesUnique` (shared value) in ./dirac and ./dirac/kop fail loudly on a mistake. Never renumber.
- Op: take an Ext sub-op instead. The free ops [158,249] are kept for hot-path ops, each by the operator's approval: a `dirac/manifest/Op/OpX` entry with `value: next` and an `approval:` line naming it.
- kgen: a new file `dirac/kgen/names_<mechanic>.go` with `func init() { enum[kop.T](...) }` (names) and/or `opsdoc_<mechanic>.go` with `func init() { opsEntries = append(opsEntries, opsDoc{...}) }` (OPS.md rows, appended after the existing ones); do not edit `names.go`/`opsdoc.go`. A new LIR op's kop form is a `func (ic *instrCtx) opX() error` handler in an `instr_<family>.go` plus one `instrOp("X", (*instrCtx).opX)` line in that file's `init()`; never a case in a shared switch. New `kop` constants outside the manifest's families (an op's own untyped operand codes) go in your own kop file.
- `canon.ChooseKinds` (`dirac/canon/canon.go`) stays one shared list: ActChoose+i is the recorded action kind, so an init-append form would make the ordinal depend on file-name init order (replay-visible, unstable across merges). Add one line at its tail; a conflict there is a re-append.
