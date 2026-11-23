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

`forgec coverage` writes `coverage.md` and `summary.md` into `-out`; the card tables are on [Card support](../support/sets.md).

## mtgsim

```sh
go run -trimpath ./cmd/mtgsim -games 20 -verify -seed 1
go run -trimpath ./cmd/mtgsim -seats 2 -a bot -b lethal-pressure
go run -trimpath ./cmd/mtgsim -seats 4 -format commander -policies az-redeal,lethal-pressure,-,forge-heuristic
```

Game *i* uses seed + *i*. A game that doesn't finish, or doesn't replay under `-verify`, is a non-zero exit.
`-policies` names a policy per seat (`-` keeps the plain heuristic); without it `-a` and `-b` name seats 0 and 1.
A seat that searched gets a `search seat` line with its decisions, simulations and tree nodes. Seats are asked as `gorged` asks them: a search policy such as `az-redeal` answers from an honest root dealt from its own observations.
`-sb-agent name=/path` adds a SpellBench v2 stdio agent as a policy.

## cardfuzz

```sh
go run -trimpath ./cmd/cardfuzz -games 1000 -workers 4
go run -trimpath ./cmd/cardfuzz -report -missing          # which cards and abilities were never used
go run -trimpath ./cmd/cardfuzz -repro cardfuzz-failures.jsonl -line 1
```

Coverage lives in `-state`; failures append to `-failures` and replay with `-repro`. `-spellbench` and `-manabrew` run the same games through those wires ([every flag](flags.md#cardfuzz)).
