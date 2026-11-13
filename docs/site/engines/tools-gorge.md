# Play and fuzz games on gorge

Three tools drive gorge without a server. All read the corpus in `.cards`.

| Tool | Use it to |
|---|---|
| `forgec` | fetch the Forge corpus, compile it to the IR, report which cards are playable |
| `mtgsim` | play seeded games between bots and replay each one to check its event chain |
| `cardfuzz` | play many random and exploring games, record failures, and repeat one with `-repro` |

## forgec

```sh
go run -trimpath ./cmd/forgec fetch      # pinned corpus into .cards
go run -trimpath ./cmd/forgec compile    # write the IR cache
go run -trimpath ./cmd/forgec report     # playable cards and the primitives still missing
```

`forgec coverage` writes `coverage.md` and `summary.md` into `-out`. The card tables are on
[Card support](../support/sets.md).

## mtgsim

```sh
go run -trimpath ./cmd/mtgsim -games 20 -verify -seed 1
go run -trimpath ./cmd/mtgsim -seats 2 -a bot -b lethal-pressure
go run -trimpath ./cmd/mtgsim -seats 4 -format commander -policies az-redeal,lethal-pressure,-,forge-heuristic
```

Game *i* uses seed + *i*. A game that doesn't finish, or doesn't replay under `-verify`, is an error and a non-zero exit.
`-policies` names a policy for every seat (`-` keeps the plain heuristic); without it `-a` and `-b` name seats 0 and 1.
Each game line ends with `policies=`, and a seat that searched gets a `search seat` line with its decisions, simulations
and tree nodes. Seats are asked the way `gorged` asks them: a search policy such as `az-redeal` answers from an honest
root dealt from its own observation feed, and a heuristic seat from the same reused board.
`-sb-agent name=/path` adds a SpellBench v2 stdio agent as a policy.

## cardfuzz

```sh
go run -trimpath ./cmd/cardfuzz -games 1000 -workers 4
go run -trimpath ./cmd/cardfuzz -report -missing          # which cards and abilities were never used
go run -trimpath ./cmd/cardfuzz -repro cardfuzz-failures.jsonl -line 1
```

It keeps its coverage in `-state` and appends failures to `-failures`. Each failure replays with `-repro`.
The flags for `-spellbench` and `-manabrew` run the same games through those wires. See [every flag](flags.md#cardfuzz).
