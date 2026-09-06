# AGENTS.md — gorge

Pure-Go Magic rules engine.

## What this is

The rules engine that will replace `mtgplay`'s XMage bridge. Card behaviour is
compiled from Forge card scripts into an IR; the engine implements the
primitives that IR references. See
`docs/superpowers/specs/2026-09-03-mtgcore-go-engine-design.md`.

## Hard rules

- **Never commit Forge card scripts.** They are GPL-3.0; gorge is Apache-2.0.
  `forgec fetch` pulls the card corpus and token scripts into `.cards/`,
  pinned to a commit SHA (`FORGE_REF` in the Makefile), which is gitignored.
  `cards/boundary_test.go` fails the build if any are tracked.
- **No cgo, no third-party deps** in the card pipeline and rules core.
  `wazero` arrives with the plugin tier in M3.
- **All state mutation goes through `events.Apply`.** If you are writing to a
  `state.Game` field outside `events`, you are introducing a replay bug.
- **No nondeterminism.** No wall clock, no ambient randomness, no `map` range
  where iteration order can reach an event.
- **This engine never imports anything from the mtgbld/mtgserve application.**

## Build / run / test

```sh
make fetch-cards          # one-time; ~25 MB, pinned commit from Card-Forge/forge
make compile-cards        # parse into the IR cache
make report               # card coverage against implemented primitives
make sim                  # build mtgsim and play 20 verified 4-seat games
make test lint
```

## Status

**M1 acceptance** (Task 26): the 12 repo decks (`internal/testutil/decks/*.json`)
load through the compiled corpus, play headless with `seat.Bot`/`rules`'
own `testBot`, and are wired into the coverage report truthfully (Ruling
W1 -- `cmd/forgec` now imports `rules` so its `effects.Supported()` sees
every registered family, not API primitives alone).

Registered primitives, measured against `.cards/ir.gob.gz` (corpus
`master @ 95f04e8a04c8925fa97cb226fc3341cabcc90a53`): 37 `api:`, 8 `kw:`,
8 `stat:`, 8 `trig:`, 1 `repl:` (62 total) -- `cards: 33667  playable: 15265
(45.3%)` per `make report`, up from 20.8% before the W1 import fixed the
undercount.

The coverage RATCHET (`rules/acceptance_test.go`'s `knownUnsupported`,
Ruling P12/D2-a) pins 35 of the 136 distinct cards across the 12 decks as
needing at least one unimplemented primitive -- almost entirely individual
keywords (kw:Equip, kw:Flash, kw:Kicker, kw:Delve, kw:Undying, kw:etbCounter,
kw:Storm, ...), which is M4's worklist; every such card is still shuffled
into its deck and simply inert (Ruling U13).

**Resolved concern** (Task 26 found it, Task 29 -- `831d8a0` -- fixed it):
`rules/trigger.go`'s `applyReplacements` did not honour Forge's
`ReplacementResult$` parameter -- every matching replacement discarded the
original event (correct only for `ReplacementResult$ Replaced`), so a
permanent whose `R:Event$ Moved` replacement used `ReplacementResult$
Updated` (the "enters the battlefield modified" idiom, e.g. Geralf's
Messenger's and Hallowed Fountain's "enters tapped") never actually moved
off the stack: the same top-of-stack object kept re-resolving and the game
never reached completion on its own. It did not hang the process, though --
Task 26's 400000-intent budget caught it cleanly every time, exhausting the
budget in roughly ten seconds and reporting a failure with exit 1 rather
than blocking forever, exactly as the budget guard is meant to. Confirmed
via `mono-black-aggro` (Geralf's Messenger) and `uw-control` (Hallowed
Fountain, Celestial Colonnade); see
`docs/superpowers/plans/2026-09-03-mtgcore-m0-m1/task-26-report.md` for the
full repro. `TestRepoDecksPlayAtEverySeatCount` and
`TestRepoDeckGamesReplayExactly` now pass unmodified at 2/4/6/8 seats and
all five seeds, and `make sim` goes 20/20, on top of Task 29's fix.

Acceptance commands:

```sh
go test ./rules/ -run 'TestEveryRepoDeck|TestRepoDecks|TestRepoDeckGames' -v
make sim
```

## Host behaviour notes (embedder observer hooks, D15)

`OnBurst` errors crash the match like a persist failure (D15): the table
halts and the chain does not continue. `OnMatchEnd` errors are discarded
because the outcome is already recorded and an error cannot un-record it, so
an embedder that persists through `OnMatchEnd` must handle its own
persistence failures inside the callback.
