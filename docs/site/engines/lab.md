# Play and experiment on dirac from Go (dirac/lab)

`dirac/lab` plays dirac games and sets up experiments in a few lines, with no other engine running. It builds games from deck lists, steps them by hand, forks a position, plays the forks out with different policies, and runs batches with win rates.

It sits on the setup `dplay` uses (a `cardset` table, `dirac.LockstepConfig`, `State.InitN`), so a game it deals is the game a clean lockstep of the same seed plays. It needs the corpus and tables: `make fetch-cards compile-cards dirac-tables` ([Build, play and check dirac](tools-dirac.md)).

## Fifteen lines

```go
a, _ := lab.ReadDeck("decks/red.json")  // or lab.ParseList("red", "17 Mountain\n23 Goblin Boarders")
b, _ := lab.ReadDeck("decks/green.json")
cfg := lab.Config{Set: "fdn", Decks: []lab.Deck{a, b}, Seed: 7}

// 100 games, hbot against random, four at a time
seeds := make([]uint64, 100)
for i := range seeds { seeds[i] = uint64(i + 1) }
_, sum, _ := lab.Batch(cfg, []lab.Policy{lab.HBot(), lab.Random(1)}, seeds, 4, 20000)
fmt.Printf("hbot wins %.2f [%.2f, %.2f] stalled %d\n", sum.WinRate(0).P, sum.WinRate(0).Lo, sum.WinRate(0).Hi, sum.Stalls)

// step a game by hand, fork the position, play the fork out two ways
g, _ := lab.NewGame(cfg)
for _, o := range g.Actions() { fmt.Println(o.Index, o.Text) }
_ = g.Apply(g.Decision().Options[0])
x := lab.Play(g.Fork(), []lab.Policy{lab.HBot(), lab.HBot()}, 20000)
y := lab.Play(g.Fork(), []lab.Policy{lab.Search(lab.SearchOptions(100)), lab.HBot()}, 20000) // a search seat
fmt.Println(x.Winner, y.Winner, g.Hash()) // g itself never moved
```

The same code on the hand-written vanilla table, needing no corpus, runs in `dirac/lab/example_test.go`.

## What is in it

| | |
|---|---|
| `Config` | a card set and one deck per seat (2 to 4), a seed, `Mulligans`, `Commander`, `Full` (the lockstep-full surface). `Table` and `IDs` take a prebuilt table instead. |
| `ReadDeck`, `ParseDeck`, `ParseList` | the gorge deck JSON `dplay -deck-a` reads, or a plain `COUNT NAME` list |
| `NewGame(cfg)`, `Open(cfg)` then `Setup.Game(seed)` | one game; or one table shared by many games |
| `Game` | `Decision`, `Actions` (each with a readable `Text`), `Apply`, `Done`, `Outcome`, `Winner`, `StallReason`, `Turn`, `Steps`, `Fork`, `Hash` |
| `Policy` | `Pick(*dirac.State, *dirac.Decision) dirac.Action`. `Random(seed)`, `HBot()`, `Search(opts)`; `FromPicker` wraps a `dirac.Picker` |
| `Play(g, policies, maxSteps)` | one game to its end; seat `i` is answered by `policies[i]` |
| `Batch(cfg, policies, seeds, workers, maxSteps)` | one game per seed; results in seed order whatever `workers` is |
| `Summary`, `Wilson`, `WriteJSONL` | wins by seat, draws, stalls (by reason), truncations; 95% Wilson intervals; one JSON line per game with `dplay`'s field names (`pair`, `decks`, `seed`, `winner`, `turns`, `intents`, `stall`, `class`, `error`) plus `outcome` |

Four seats play the same way: pass four decks and four policies. `Commander: true` takes each `Deck.Commanders` (names that appear in its list).

### Policies

A policy takes the `*dirac.State` as well as the decision, because `HBot` and `Search` read the position. `Random` restarts its stream every game from its own seed, the game's seed and its seat, so a game's result depends on those alone. `Search` is `dirac/search`'s PUCT over the heuristic leaf, built as `dirac/oracle`'s search seats build it; `SearchOptions(sims)` is `search.DefaultOptions()` with a simulation budget, and any `search.Options` works. A policy that carries per-game state implements `Starter`; one that a `Batch` must copy per worker implements `Cloner`.

`HBot()` reads facts derived from the card table alone (`hbot.NewFacts`). `dplay`'s hbot reads gorge's compiled cards, which is exact, so games differ between the two. `HBotWith` takes any facts builder.

### Results

A game ends one of these ways, and only the first two are results of play:

| outcome | meaning |
|---|---|
| `Won` | a seat won |
| `Drawn` | the game ended with no winner |
| `Stalled` | dirac stopped at a declared capacity or a rule gap; the position has no result |
| `Truncated` | `maxSteps` ran out first |
| `Failed` | a policy panicked or answered with an action the decision did not offer |

A stall is never a draw. `Summary` counts it apart (`Stalls`, `StallsBy`), and win rates are wins over all games, so a stalled or truncated game counts against every seat rather than for nobody. If your stall count is not zero, look at `StallsBy` before trusting a win rate.

`Hash` covers every seat's view of the position and the pending decision. It does not read library order, so two positions that differ only in a library hash the same.

## What is not here

- No gym or Python binding. There is no `reset`/`step` environment, no observation tensors and no reward. `dirac/feat` encodes a state for a net and `dirac/neteval` runs one; neither is wrapped here.
- No training or self-play loop, no recorded search targets, no replay records for `kfuzz -replay` (`dplay -sample-replay` writes those).
- No pool or pairing handling: you give decks and seeds. `dplay` reads pools and committed slices.
- No `sbt` seat (it needs gorge), no turn cap, and no mulligan policy: with `Mulligans` set the policies answer the mulligan decisions themselves.
- A `Game` is not safe for concurrent use. `Batch` runs one per worker.
