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
  `events.Kind` is append-only, and every kind M2r and M2d added was appended
  after all earlier kinds so ordinals, the hash chain and golden replays stay
  untouched: M2r appended `CastInfo`, `Choose`, `TokenCreate`, `StackCopy`,
  `Attach` and `AbilityPush`; M2d appended `ModeChosen` for mid-resolution
  modal answers.
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

**M2r closed the coverage ratchet.** `rules/acceptance_test.go`'s
`knownUnsupported` (Ruling P12/D2-a) is the empty literal, and
`TestEveryRepoDeckIsFullySupported` asserts the measured gap set equals the
table in both directions (Ruling R-20): a card the build newly cannot fully
support fails and is named together with the primitives it is missing, and a
table entry the build now supports is stale and fails too. Measured against
the corpus pin, the ratchet stands at **0 of 136** -- every one of the 136
distinct cards across the 12 repo decks (`internal/testutil/decks/*.json`)
is fully supported and plays. The 12 decks round-robin across 2/4/6/8 seats
(`TestRepoDecksPlayAtEverySeatCount`), replay byte-identically
(`TestRepoDeckGamesReplayExactly`), and `TestHeads` pins the chain heads as
goldens in `rules/heads_test.go`:

| seats | 2 | 4 | 6 | 8 |
|---|---|---|---|---|
| chain head | `45e0671d07b60d9e` | `795a100313094d6c` | `0311852b655e44d0` | `1216344ec91e5881` |

`make sim` plays 20 verified 4-seat games from the same seed set, every one
replaying byte-identically (20/20 `replay OK`).

Measured at the corpus pin `master @
95f04e8a04c8925fa97cb226fc3341cabcc90a53` (`FORGE_REF` in the Makefile):
`make report` prints `cards: 33667  playable: 19765 (58.7%)` with `tokens:
839`, and the registered primitive set -- `effects.Supported()`, measured
against `.cards/ir.gob.gz` -- stands at 42 `api:`, 31 `kw:`, 8 `stat:`, 8
`trig:`, 1 `repl:` (90 total), every one of them referenced by the corpus.
M1's 37/8/8/8/1 = 62 was the count before M2r registered the keyword,
trigger, static and mid-game primitives the ratchet's card work needed.

A seat's clients must be able to answer every `decision.Kind`, and the set is
now closed: the M1 kinds (`priority`, `target`, `attackers`, `blockers`,
`trigger_order`, `trigger_optional`), M2r's `choose` (an {X} value, delve
exiles, cost sacrifices, "as it enters" name/type/number, miracle-style
yes/no), and the two M2d closures -- `mulligan`, the London keep/mulligan
and bottoming round `Config.Mulligans` runs between the deal and turn 1, and
`modes`, the modal pick and unless-pay ask a mid-resolution answer serves
(the `ModeChosen` event carries the answer into the log). Concede (M2d-3) is
not a kind: it is a `concede` option on every priority decision that emits
the existing `PlayerLost` with Text "conceded". The engine-side defaults
that still stand in for a choice the engine cannot yet ask are listed under
**Known approximations** below.

Acceptance commands:

```sh
go test ./rules/ -run 'TestEveryRepoDeck|TestRepoDecks|TestRepoDeckGames' -v
make sim
```

## Known approximations

Where a registered primitive is implemented with an engine-side default for
a player choice the engine cannot yet ask, or with behaviour narrower than
the card text, it is listed here with the stand-in's location and the
milestone that removes it. Each entry was verified live at `main` `6efda17`;
a stand-in whose code no longer exists must be deleted from this list, never
kept -- a deferral that names a milestone is only as good as the milestone
still owing it.

| Stand-in | Where | Removed by |
|---|---|---|
| "As this enters, choose ..." is asked at cast/play time, so the choice is recorded and visible a resolution early; the mid-resolution machinery that could move it to resolution time exists (M2d-2) but the asks have not migrated | `rules/cast.go:453-630` | M4 |
| `Mana` with `Produced$ Any`/`Combo Any` adds colourless instead of asking (Cavern of Souls, Chromatic Star, Lion's Eye Diamond, Lotus Petal) | `effects/misc.go:289-293` | M4 |
| `RestrictValid$` spend restrictions are never read (Cavern of Souls' second mana ability, Eldrazi Temple's {C}{C}) -- colour and restriction both come out as plain colourless | `effects/misc.go:289-293` | M4 (mana restrictions) |
| A paid copy always copies to the original's target: `MayChooseTarget$`/`UnlessSwitched$` are not read and a Note records it (Chain Lightning's pay-copy clause; the unless-pay ask itself is real since M2d-2) | `effects/copy.go:39-116` | M4 |
| `Discard` discards from the front of the hand, never asking which card (Cabal Therapy, Duress, Thoughtseize) | `effects/cardflow.go:66-79` | M4 |
| `Sacrifice` with a player target is skipped and `SacValid$` is never read (Gatekeeper of Malakir's kicked ETB) | `effects/zone.go:151-160` | M4 |
| `Counter` is unconditional: `UnlessCost$` is never read, so "counter unless its controller pays {N}" is not offered (Mana Leak, Spell Pierce, Daze, Mausoleum Wanderer; M2d-2's unless-pay ask serves only `CopySpellAbility`) | `effects/misc.go:103-119` | M4 |
| `Effect` (a continuous effect from `StaticAbilities$`/`Triggers$` for `Duration$`) records a Note only (Palace Jailer's "until it leaves", Vines of Vastwood's can't-be-targeted) | `effects/misc.go:49-56` | M4 |
| `Regenerate` grants a Shield counter the state-based actions never consume (Experiment One still dies) | `effects/counters.go:80-90` | M4 (CR 701.16) |
| `DelayedTrigger` records a registration Note and never fires (Flickerwisp's return at the next end step) | `effects/misc.go:124-127` | M4 |
| `Vote` gives every voter the first `Choices$` entry and records one Note per vote (Council's Judgment) | `effects/misc.go:239-247` | M4 |
| `BecomeMonarch` records a Note only; the monarch's end-step draw does not exist (Palace Jailer) | `effects/misc.go:251-257` | M4 |
| `RearrangeTopOfLibrary` looks at the top N but keeps the order unchanged -- the reorder choice is never asked (Ponder) | `effects/cardflow.go:210-229` | M4 |
| A host that cannot answer a decision gets the deterministic fallback: `Charm` takes its first mode with a Note (the modes ask itself is a real KModes decision since M2d-2) | `effects/misc.go:171-233` | none -- the no-ask host is the fuzz/test degradation contract |

## Host behaviour notes (embedder observer hooks, D15)

`OnBurst` errors crash the match like a persist failure (D15): the table
halts and the chain does not continue. `OnMatchEnd` errors are discarded
because the outcome is already recorded and an error cannot un-record it, so
an embedder that persists through `OnMatchEnd` must handle its own
persistence failures inside the callback.
