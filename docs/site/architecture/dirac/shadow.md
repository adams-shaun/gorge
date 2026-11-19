# Staging a state from an observation (the shadow)

A bot attached to an external engine (a SpellBench v2 host, later Forge or XMage) never holds that engine's game. It holds what its own seat sees. To search, it needs a complete world: a guess at every hidden zone. The shadow turns that observation into a dirac `State` the kernel can step, so a dirac policy or rollout search can answer, and the answer can be mapped back onto the external engine's candidates.

Two rules shape everything here.

- **No omniscience, by construction.** The input type has no field that can hold an opponent's hand or a library's identity beyond what the caller deals. The package imports `dirac`, `dirac/kop` and `engine/view` only. It never reads `engine/state`, never uses the search `Clairvoyant` option, and a gorge game enters only in tests, as the verifier.
- **Unstageable is an error, not a guess.** A position outside the supported scope returns a typed `dirac.ErrUnstageable{Reason}`. The caller falls back to another policy and counts the reason. A partial or guessed state is never returned.

## The two layers

| Layer | Where | Does |
|---|---|---|
| `dirac.NewFromWorld(*World)` | `dirac/zone_stage.go` | Builds a `State` from a `World`: a flat list of cards (owner, zone, place in zone, controller, tapped, summoning-sick) plus life, land drop, turn, step and priority holder. Refuses anything it cannot rebuild exactly. |
| `shadow.WorldFromView`, `shadow.StageFromView` | `dirac/shadow` | Builds a `World` from an `engine/view.View` (the seat's own view) plus a caller-supplied `Hidden` (the opponent's hand and each library, as card names). Checks the dealt zones against the counts the view shows. |

`Hidden` is the caller's. A determinization deals it from the seat's own deck list and the counts the view shows; the verifier supplies the true zones. The staging code cannot tell which.

## What stages today (chunks C4, C5 and C5b)

- Two seats.
- Lands and creatures with no abilities beyond combat keywords, and creatures whose only ability is "put a +1/+1 counter on this" (`selfCounterAbility`: activated, no target, no once-per-turn state).
- Damage marked on a battlefield creature (C5; a damaged creature is also marked dealt-damage-this-turn, as `dmgPerm` records it) and nonzero +1/+1 counters on a battlefield creature (C5).
- Plain creature tokens on the battlefield (C5b): a token `CardDef` with `TToken`, combat keywords and no ability. A token is a `WorldObj` with `Token` set, placed after the decks' objects in the order it was made, so it gets the object index the game gave it. The shadow finds its table token by the view's name ("Goblin Token"), printed power and toughness, against the table's script names (`r_1_1_goblin`); zero or several matches is `token`.
- Attached Auras and Equipment (C5b), on a creature of either seat: `WorldObj.Attached` and `Host` (the bearer's object index), staged by `attachTo`. The card must be an Aura or Equipment whose abilities are statics that commute (a P/T or keyword grant, can't attack, can't block, no condition) plus the Equip ability. An attachment's layer timestamp is the order the staging attaches in, the battlefield list order, not the order the game did, which an observation does not show; that is exact for the statics admitted and is why only commuting ones are.
- A sorcery or instant whose whole spell ability creates tokens, in any hidden zone, the graveyard or the hand (a card carries no state there).
- Library, hand, graveyard and battlefield order, tapped and summoning-sick marks, life, land drop, turn, step and priority holder.
- Quiet priority steps only: upkeep, draw, first main, second main, end.

Everything else is refused with a reason. dirac's reasons: `table`, `seats`, `object-layout`, `card`, `zone`, `control`, `place`, `step`, `priority`, `turn`, `life`, `battlefield`, `token`, `attachment`. The shadow adds: `game-over`, `seats`, `stack`, `decision`, `unknown-card`, `unmapped-object`, `hidden`, `exile`, `counter`, `mark`, `pool`, `step-name`. Each has a test that provokes it (`TestUnstageableReasons`, `TestWorldFromViewReasons`).

Still refused: copies (`token`), any other counter kind, a zero count, or a counter or damage off the battlefield (`counter`, `mark`), an Aura or Equipment of another shape, attached to a non-creature or a player, or an Aura on the battlefield attached to nothing (`attachment`), a token that is not a plain creature token or that the table cannot name (`token`). Both new reasons are dirac's (`dirac.ReasonToken`, `dirac.ReasonAttach`); the shadow reuses them.

A token that has left the battlefield leaves its object index behind in the game that made it (dirac reuses an index only when the table is full). The view cannot show it, so a gap before a shown token is refused as `token`; a token that left after the last shown one leaves no gap, and the staged game simply holds fewer objects than the one that made it. The verifier skips those positions and counts them.

Not carried: the RNG draw count (the verifier masks `Projection.Draws`) and the per-card in-hand history.

## How it is proven

`TestStageMatchesReplay` (`dirac/shadow/stage_test.go`):

1. Play 200 lockstep games on a pool of vanilla lands, 14 vanilla or keyword creatures and Hungry Ghoul. Odd-numbered games wait for a position where a battlefield creature carries damage or a counter.
2. At a random step, build the seat's `view.View` from gorge's game and stage a `State` from it, with the true hidden zones as `Hidden`.
3. Require `oracle.Project` equality with the dirac state replayed from the same prefix.
4. Continue both on one shared policy to game end and require zero mismatches.

Last run (C5): 197 of 200 games staged a position (719 tried; 476 refused as a non-quiet step, 46 for a non-empty stack), 40 staged positions held damage and 71 held +1/+1 counters, 103,742 continuation decisions, 0 mismatches. The test fails if fewer than 20 staged positions hold each mark.

`TestStageCardsMatchReplay` (`dirac/shadow/stage_cards_test.go`) is the same verifier on a corpus pool from the `mbc` set, every card supported with evidence: Plains and Mountain, eleven creatures with combat keywords, Pacifism (an Aura), Swiftfoot Boots (an Equipment) and Dragon Fodder (two Goblin tokens). A quarter of the games wait for a position holding an attachment and a quarter for one holding a token. Last run (C5b): 181 of 200 games staged a position, 85 staged positions held an attachment and 117 a token, 114,034 continuation decisions, 0 mismatches. The test fails if fewer than 20 positions hold each.

`TestStageMutationsAreCaught` drops each of tapped, summoning-sick, damage, +1/+1 counters, life, land drop, library order, graveyard order and turn from the staging in turn, and on the corpus pool drops an Equipment's attachment, moves an Aura to another creature and drops the tokens, and requires the verifier to fail on each. A feature that staging silently dropped would be caught.

## Fork and rollout (chunk C8)

`dirac.Rollout` (`dirac/driver.go`) owns one working `State` on its own `ForkPool`. `Fork(src)` is `State.CopyFrom(src)` into it; `src` is never written. `Run(src, pol, max)` forks and plays the fork with a caller-supplied `Picker` (`Pick(*Decision) Action`; `RandomPolicy` fits) and returns a `RolloutResult`: exactly one of `Over` (with `Winner`, -1 a draw), `Stalled` (with `Reason`) or `Truncated` (the decision cap). A stall is never a draw: `Rollout.Stalls[reason]` counts it and the search must drop that sample. The loop allocates nothing; the working `State` and the decision buffer are allocated once. A `Rollout` is for one goroutine. `dirac/shadow` and a search use it through the public API.

Tests (`dirac/fork_rollout_mp_test.go`): `TestForkLeavesOriginal`, `TestRolloutFourSeatsEnds`, `TestRolloutStallIsCountedNotDraw`, `TestRolloutTruncates`, `TestRolloutDeterministic`, `TestRolloutStepAllocsZero`.

## Extending it

Each later chunk widens the refused set and keeps the same verifier, run on a pool that uses the new feature.

| Chunk | Adds |
|---|---|
| C5 (partly done: damage, +1/+1 counters; C5b: attached Auras and Equipment, plain creature tokens) | other counter kinds, attachments with abilities beyond a static grant, tokens with abilities, copies |
| C6 | stack, triggers, replacement effects, per-source remembered memory |
| C7 | mid-turn positions: declare attackers and blockers, targets |
| C8 | `CopyFrom` fork and rollout loop (landed, below) |
| C13 | four-seat pods |

A new feature needs: a field on `World` (or `WorldObj`), the staging code in `zone_stage.go`, the mapping in `shadow.WorldFromView`, a mutation in `TestStageMutationsAreCaught`, and a refusal reason for the cases it still cannot rebuild. Keep `dirac.State` rules: no references, no allocation in the transition loop, no growth of `stateCore`.

The SpellBench glue (the candidate mapping, the agent) lives in the private companion module, behind a build tag, and imports this package at a pinned gorge commit.
