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
| chain head | `0876361619998e2a` | `d74b8a889f09be48` | `ea3d87a74c4c954d` | `5e573c76021a419f` |

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
| `Discard` implements exactly two corpus shapes. `Mode$ RevealYouChoose` (Thoughtseize, Duress) is a real mid-resolution KModes ask: the caster chooses from the DiscardValid$-filtered hand, and the answered choice re-enters to discard exactly those cards (a chained `SubAbility$` such as Thoughtseize's `DBLoseLife` fires exactly once, once the choice is in). `Mode$ RevealDiscardAll` (Cabal Therapy) is a filter, not a choice: it discards every matching card and asks nothing. **Everything else still takes the front card deterministically.** A Discard with no `Mode$` (the cleanup-step and Delve-style shape) or with any OTHER `Mode$` value falls to `effDiscard`'s default `hand[0]` take — `TgtChoose` is 804 corpus SAs, `Hand` 108, `Random` 58, `Defined` 26, `LookYouChoose` 7, `YouChoose` 2, `RevealTgtChoose` 1, for **998 of the 1099 corpus Discard SAs** (Faithless Looting, Thirst for Knowledge, Riddlesmith, Hypnotic Specter, Avaricious Dragon, Dragon Mage, Magus of the Wheel and Reforge the Soul all sit in the repo decks), and a no-ask host on a `RevealYouChoose` also falls back to `hand[0]`. The CR 514.1 cleanup-step discard is NOT this primitive at all — it is a real `KChoose` ask in `rules/combat.go`'s `cleanupStep`, and `effDiscard` is not on that path. Multi-target `RevealYouChoose` still abandons targets 2..n (the first `h.Ask` suspends and the re-entry sees `Ctx.Discard != nil` for every target) | `effects/cardflow.go` (`effDiscard`) | M4 |
| `Sacrifice` with a player target is skipped and `SacValid$` is never read (Gatekeeper of Malakir's kicked ETB) | `effects/zone.go:151-160` | M4 |
| `Counter` is unconditional: `UnlessCost$` is never read, so "counter unless its controller pays {N}" is not offered (Mana Leak, Spell Pierce, Daze, Mausoleum Wanderer; M2d-2's unless-pay ask serves only `CopySpellAbility`) | `effects/misc.go:103-119` | M4 |
| `Effect` (a continuous effect from `StaticAbilities$`/`Triggers$` for `Duration$`) records a Note only (Palace Jailer's "until it leaves", Vines of Vastwood's can't-be-targeted) | `effects/misc.go:49-56` | M4 |
| `Regenerate` grants a Shield counter the state-based actions never consume (Experiment One still dies) | `effects/counters.go:80-90` | M4 (CR 701.16) |
| `DelayedTrigger` records a registration Note and never fires (Flickerwisp's return at the next end step) | `effects/misc.go:124-127` | M4 |
| `Vote` gives every voter the first `Choices$` entry and records one Note per vote (Council's Judgment) | `effects/misc.go:239-247` | M4 |
| `BecomeMonarch` records a Note only; the monarch's end-step draw does not exist (Palace Jailer) | `effects/misc.go:251-257` | M4 |
| `RearrangeTopOfLibrary` looks at the top N but keeps the order unchanged -- the reorder choice is never asked (Ponder) | `effects/cardflow.go:210-229` | M4 |
| The tap gate is colour-aware for plain `Produced$` strings, but `Any`/`Combo Any` still resolve to colourless, and `Combo X Y`/`Chosen` retain the executor's degenerate rune output rather than a selectable colour; land drops remain colour-blind and prefer basics only | `effects/misc.go:289-293` (`effMana`); `botpolicy/cast.go` (`chooseLand`) | M4 mana-choice decisions and a land-entry/production-aware `chooseLand` |
| The tap gate's production collector reads a non-literal `Amount$` as **1** where the executor's `Num` reads it as `X`-or-0, so a source is recorded as producing mana the pool never receives. 122 corpus mana abilities are affected (109 `Amount$ X`, 7 `Amount$ Y`, 3 `UrzaAmount`, and `Sacrificed$CardPower`/`IncubationAmount`/`Count$Valid Goblin.YouCtrl`); 50 of them pair it with a plain coloured `Produced$`, which makes `ProducesColour` true and lets `chooseTap` actively PREFER a source that adds nothing. Only zero-vs-nonzero reaches the decision -- the magnitude never does -- and all real production still comes from `effMana` via `events.Apply`, so this is a bot-quality defect, not a state-correctness one | `cards/mana_production.go` (`manaAbilityAmount`) vs `effects/count.go:43-46` (`Num`) | M4 mana-choice decisions |
| A host that cannot answer a decision gets the deterministic fallback: `Charm` takes its first mode with a Note (the modes ask itself is a real KModes decision since M2d-2) | `effects/misc.go:171-233` | none -- the no-ask host is the fuzz/test degradation contract |

## Host behaviour notes (embedder observer hooks, D15)

`OnBurst` errors crash the match like a persist failure (D15): the table
halts and the chain does not continue. `OnMatchEnd` errors are discarded
because the outcome is already recorded and an error cannot un-record it, so
an embedder that persists through `OnMatchEnd` must handle its own
persistence failures inside the callback.

## Running a gorged server while you work

Two orchestrator sessions share this box, and one of them serves a live demo
that is redeployed automatically after every merge into `main`. So ports are
allocated, not first-come:

| range | who |
|---|---|
| 8080-8081 | the demo. **Never bind these**, and never run `make deploy-demo` |
| 8082-8089 | the bot-policy / botbench workstream |
| 8090-8099 | task agents on the engine side — pick one of these |

Put persistence under `/tmp/gorge-<something-unique>`, never the repo-root
default `gorged-data`: several servers sharing one persistence directory
silently corrupt each other, and a resumed directory written by an older binary
comes back with the fields that binary lacked set to their zero values
(`Format`'s zero is `constructed`, a real value, so `-format` is ignored with
nothing in the output to say so).

**Stopping a server: never `pkill -f` or a bare `pgrep -f`.** The pattern is
matched against every process's `/proc/<pid>/cmdline` including your own shell's,
and it has killed a session here. Find the server by its listening socket
(`ss -lptn`), confirm the pid, then signal that pid. Note that `/proc/<pid>/comm`
is the BINARY's name -- if you built `gorged-after`, its `comm` is
`gorged-after`, not `gorged` -- so match on the port rather than the name.

`scripts/fleet.sh ports` prints the current allocation, and `scripts/fleet.sh
port` prints a free one in your range.

## Working in a task worktree

Task worktrees are created with `scripts/agent-worktree.sh <id> [base] [--web]`,
never with a bare `git worktree add`. A worktree carries only tracked files, and
this repo needs one untracked thing to test honestly: the `.cards` corpus. Without
it `internal/testutil`'s `CorpusRegistry` calls `t.Skip`, so every
corpus-dependent test SKIPS instead of running, the package still prints `ok` --
in about 2ms -- and the run reads green to you, to the review pre-filter and to
the gate while having executed almost nothing. If you find yourself in a worktree
with no `.cards`, stop and say so rather than reporting a green suite.

Which agent seats exist, what they cost and when to escalate between them is
recorded in `docs/superpowers/agent-seats.md`.
