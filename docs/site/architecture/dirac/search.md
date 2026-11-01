# Search and bots

dirac exists so that searches and self-play can step games cheaply. This page says what it offers a search, for bot authors
and for readers comparing engines. It reports no playing strength. Speed per simulation is on
[Bot speed](../../performance/bots.md).

## What dirac offers a search

| Need | Operation | Cost |
|---|---|---|
| copy a position | `dst.CopyFrom(root)` | about 41 ns, no allocation ([State](state.md)) |
| list a decision | `StepInto(&d)`: packed `Action`s in canonical order | no allocation |
| apply a move | `Apply(a)` | no allocation |
| hide what a seat can't see | `Hidden(o, seat)`, `HiddenCards`, `DealHidden` (re-deal hidden cards from a pool), `RedealOwnLibrary`, `ReseedChance` | in place, on the copy |
| read a position | `ObjCard`, `ObjZone`, `Controller`, `HandSize`, `LibrarySize`, `StepNow` and others | no allocation |
| recompute only what changed | `SetEpoch` and the [change marks](state.md#change-marks), `ObjHash`, `ProjectDelta` | proportional to what changed |

Everything here has to keep the rule the whole engine keeps: no allocation per transition, no map iteration whose order could
reach an action, and the same seed and answers give the same game, byte for byte.

## How a simulation uses them

The shape below is that of [`bots/azmcts`](https://github.com/adams-shaun/gorge/tree/main/bots/azmcts), gorge's PUCT
tree search over information sets. The maintainers run a port of it on dirac, in their private companion module.

1. Copy the root with `CopyFrom`.
2. Re-deal what the seat can't see (its own library order, the opponent's hand and library, from a card pool if one is given)
   and re-seed chance, from a per-decision seed.
3. Walk: play the tree's edges as packed actions, and answer every other decision with a heuristic bot until the next searched point.
4. Expand one node, value the leaf once with an evaluator (no rollouts), and back the value up.

Almost all of a simulation is the engine stepping along that line, which is why the transition and copy numbers decide what a
search can afford. A rules probe inside `Step` uses a fork from the `State`'s [fork pool](state.md#hot-and-cold), so nothing
allocates at a warmed depth.

## The ports

| Package | Entry points | What it gives a bot |
|---|---|---|
| `dirac/search/hbot` | `NewFacts(*kop.Table)`, `New(*Facts)`, `(*Bot).EnableHoldInstant()` | a native port of gorge's auto-pay heuristic bot, held to it decision by decision by `khbot` |
| `dirac/search/mull` | `KeepLands`, `KeepConstructed`, `KeepNever`, `CoinMulls` | gorge's London-mulligan policies, as pure functions over a hand |
| `dirac/feat` | `NewPool(tbl, specs)`, `NewEncoder(pool)`, `EncodeState`, `EncodeOptions` | the network input encoding, below |

These import nothing from gorge. `oracle.HBotFacts` and `oracle.FeatPool` build their tables from a gorge registry, and dirac-only
code passes a `kop.Table` and pre-derived specs instead.

## Network inputs

`dirac/feat` turns a position into the same inputs gorge's `bots/policynet` produces from the deciding seat's redacted
view: dense scalars, a sorted hashed sparse bag, one entity record per visible card, and one encoding per option. Lockstep can assert
that byte for byte at every decision, so a network trained on one engine's games reads the other's identically.

dirac has no strings, so every string-derived fact the encoder reads (names, type lines, costs, keyword spellings) is computed
per `CardID` into a `Pool` when the pool loads. The hot path streams FNV-1a over those rows and dirac's own numbers, and an encode
allocates nothing.
