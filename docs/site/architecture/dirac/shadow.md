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

## What stages today (chunks C4, C5, C5b and C6)

- Two seats.
- Lands and creatures with no abilities beyond combat keywords, and creatures whose only ability is "put a +1/+1 counter on this" (`selfCounterAbility`: activated, no target, no once-per-turn state).
- Damage marked on a battlefield creature (C5; a damaged creature is also marked dealt-damage-this-turn, as `dmgPerm` records it) and nonzero +1/+1 counters on a battlefield creature (C5).
- Plain creature tokens on the battlefield (C5b): a token `CardDef` with `TToken`, combat keywords and no ability. A token is a `WorldObj` with `Token` set, placed after the decks' objects in the order it was made, so it gets the object index the game gave it. The shadow finds its table token by the view's name ("Goblin Token"), printed power and toughness, against the table's script names (`r_1_1_goblin`); zero or several matches is `token`.
- Attached Auras and Equipment (C5b), on a creature of either seat: `WorldObj.Attached` and `Host` (the bearer's object index), staged by `attachTo`. The card must be an Aura or Equipment whose abilities are statics that commute (a P/T or keyword grant, can't attack, can't block, no condition) plus the Equip ability. An attachment's layer timestamp is the order the staging attaches in, the battlefield list order, not the order the game did, which an observation does not show; that is exact for the statics admitted and is why only commuting ones are.
- A sorcery or instant whose whole spell ability creates tokens, in any hidden zone, the graveyard or the hand (a card carries no state there).
- Library, hand, graveyard and battlefield order, tapped and summoning-sick marks, life, land drop, turn, step and priority holder.
- The stack (C6), public and shown in `view.View.Stack`: spells with no target, mode, X or kicker (a token-making instant or sorcery, a creature, an Equipment; an Aura targets and is refused) and activated self-counter abilities ("{1}, sacrifice another creature: put a +1/+1 counter on this") whose source is a battlefield creature. A spell is an object in the stack zone (`WorldStack`, `dirac/zone_stagestack.go`); its cost shows in the tapped lands, the sacrificed creature and the graveyard. Priority over a non-empty stack: the holder of the top entry's controller has passed no one (0 passes); any other holder was passed to (1). The active seat holding priority over the other seat's top entry is refused as `priority`: after a pass or after a resolution, and the view cannot say which.
- Quiet priority steps only: upkeep, draw, first main, second main, end (the stack, when non-empty, is one of these steps').

Everything else is refused with a reason. dirac's reasons: `table`, `seats`, `object-layout`, `card`, `zone`, `control`, `place`, `step`, `priority`, `turn`, `life`, `battlefield`, `token`, `attachment`, `stack-entry`. The shadow adds: `game-over`, `seats`, `stack`, `decision`, `pending`, `unknown-card`, `unmapped-object`, `hidden`, `exile`, `counter`, `mark`, `pool`, `step-name`. Each has a test that provokes it (`TestUnstageableReasons`, `TestWorldFromViewReasons`).

Still refused, as typed reasons: a trigger on the stack, a stack entry with a target, mode, X, kick or a copy (`stack`; dirac's `stack-entry` for a card it cannot rebuild as an entry), any trigger waiting to be put on the stack (`pending`), and the active seat's priority over the other's entry (`priority`). Delayed triggers, replacement and prevention shields and per-source remembered memory (`SrcMem`, `RemPersist`, `prevNext`) are not observable and are made only by cards `stageableCard` already refuses (`card`), so no position holding one stages. Widening them needs a card of that kind in the admitted set and a `View` field that shows its state, which the view does not carry today.

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

`TestStageCardsMatchReplay` (`dirac/shadow/stage_cards_test.go`) is the same verifier on a corpus pool from the `mbc` set, every card supported with evidence: Plains and Mountain, eleven creatures with combat keywords, Pacifism (an Aura), Swiftfoot Boots (an Equipment) and Dragon Fodder (two Goblin tokens). A quarter of the games wait for a position holding an attachment and a quarter for one holding a token. Last run (C6): 173 of 200 games staged a position, 76 staged positions held an attachment, 119 a token and 54 a spell on the stack (a token-making instant, a creature, an Equipment), 109,175 continuation decisions, 0 mismatches. The test fails if fewer than 20 positions hold each. The vanilla pool of `TestStageMatchesReplay` stages a Hungry Ghoul ability on the stack: 44 positions, 188 of 200 games staged, 94,709 continuation decisions, 0 mismatches, fails below 20. Games that wait for a stack position may never see a stageable one, so the staged floor of the corpus pool is 160.

`TestStageMutationsAreCaught` drops each of tapped, summoning-sick, damage, +1/+1 counters, life, land drop, library order, graveyard order and turn from the staging in turn, drops a stack ability, and on the corpus pool drops an Equipment's attachment, a stack spell (to the graveyard), moves an Aura to another creature and drops the tokens, and requires the verifier to fail on each. A feature that staging silently dropped would be caught.

## Fork and rollout (chunk C8)

`dirac.Rollout` (`dirac/driver.go`) owns one working `State` on its own `ForkPool`. `Fork(src)` is `State.CopyFrom(src)` into it; `src` is never written. `Run(src, pol, max)` forks and plays the fork with a caller-supplied `Picker` (`Pick(*Decision) Action`; `RandomPolicy` fits) and returns a `RolloutResult`: exactly one of `Over` (with `Winner`, -1 a draw), `Stalled` (with `Reason`) or `Truncated` (the decision cap). A stall is never a draw: `Rollout.Stalls[reason]` counts it and the search must drop that sample. The loop allocates nothing; the working `State` and the decision buffer are allocated once. A `Rollout` is for one goroutine. `dirac/shadow` and a search use it through the public API.

Tests (`dirac/fork_rollout_mp_test.go`): `TestForkLeavesOriginal`, `TestRolloutFourSeatsEnds`, `TestRolloutStallIsCountedNotDraw`, `TestRolloutTruncates`, `TestRolloutDeterministic`, `TestRolloutStepAllocsZero`.

## Extending it

Each later chunk widens the refused set and keeps the same verifier, run on a pool that uses the new feature.

| Chunk | Adds |
|---|---|
| C5 (partly done: damage, +1/+1 counters; C5b: attached Auras and Equipment, plain creature tokens) | other counter kinds, attachments with abilities beyond a static grant, tokens with abilities, copies |
| C6 (partly done: spells and self-counter abilities on the stack) | triggers on the stack and pending, targets and modes on stack entries, delayed triggers, replacement effects, per-source remembered memory |
| C7 | mid-turn positions: declare attackers and blockers, targets |
| C8 | `CopyFrom` fork and rollout loop (landed, below) |
| C13 | four-seat pods |

A new feature needs: a field on `World` (or `WorldObj`), the staging code in `zone_stage.go`, the mapping in `shadow.WorldFromView`, a mutation in `TestStageMutationsAreCaught`, and a refusal reason for the cases it still cannot rebuild. Keep `dirac.State` rules: no references, no allocation in the transition loop, no growth of `stateCore`.

The SpellBench glue (the candidate mapping, the agent) lives in the private companion module, behind a build tag, and imports this package at a pinned gorge commit.
