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

## What stages today (chunk C4)

- Two seats.
- Lands and creatures with no abilities beyond combat keywords.
- Library, hand, graveyard and battlefield order, tapped and summoning-sick marks, life, land drop, turn, step and priority holder.
- Quiet priority steps only: upkeep, draw, first main, second main, end.

Everything else is refused with a reason. dirac's reasons: `table`, `seats`, `object-layout`, `card`, `zone`, `control`, `place`, `step`, `priority`, `turn`, `life`, `battlefield`. The shadow adds: `game-over`, `seats`, `stack`, `decision`, `unknown-card`, `unmapped-object`, `hidden`, `exile`, `token`, `mark`, `pool`, `step-name`. Each has a test that provokes it (`TestUnstageableReasons`, `TestWorldFromViewReasons`).

Not carried: the RNG draw count (the verifier masks `Projection.Draws`) and the per-card in-hand history.

## How it is proven

`TestStageMatchesReplay` (`dirac/shadow/stage_test.go`):

1. Play 200 lockstep games on a pool of vanilla lands and 14 vanilla or keyword creatures.
2. At a random step, build the seat's `view.View` from gorge's game and stage a `State` from it, with the true hidden zones as `Hidden`.
3. Require `oracle.Project` equality with the dirac state replayed from the same prefix.
4. Continue both on one shared policy to game end and require zero mismatches.

Last run: 200 of 200 positions staged (462 tried; 243 refused as a non-quiet step, 19 for a non-empty stack), 105,166 continuation decisions, 0 mismatches.

`TestStageMutationsAreCaught` drops each of tapped, summoning-sick, life, land drop, library order, graveyard order and turn from the staging in turn, and requires the verifier to fail on each. A feature that staging silently dropped would be caught.

## Extending it

Each later chunk widens the refused set and keeps the same verifier, run on a pool that uses the new feature.

| Chunk | Adds |
|---|---|
| C5 | counters, damage, auras, equipment, tokens |
| C6 | stack, triggers, replacement effects, per-source remembered memory |
| C7 | mid-turn positions: declare attackers and blockers, targets |
| C8 | `CopyFrom` fork and rollout loop |
| C13 | four-seat pods |

A new feature needs: a field on `World` (or `WorldObj`), the staging code in `zone_stage.go`, the mapping in `shadow.WorldFromView`, a mutation in `TestStageMutationsAreCaught`, and a refusal reason for the cases it still cannot rebuild. Keep `dirac.State` rules: no references, no allocation in the transition loop, no growth of `stateCore`.

The SpellBench glue (the candidate mapping, the agent) lives in the private companion module, behind a build tag, and imports this package at a pinned gorge commit.
