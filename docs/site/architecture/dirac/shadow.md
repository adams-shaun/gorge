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

## What stages today (chunks C4, C5, C5b, C6 and C7)

- Two seats.
- Lands and creatures with no abilities beyond combat keywords, and creatures whose only ability is "put a +1/+1 counter on this" (`selfCounterAbility`: activated, no target, no once-per-turn state).
- Damage marked on a battlefield creature (C5; a damaged creature is also marked dealt-damage-this-turn, as `dmgPerm` records it) and nonzero +1/+1 counters on a battlefield creature (C5).
- Plain creature tokens on the battlefield (C5b): a token `CardDef` with `TToken`, combat keywords and no ability. A token is a `WorldObj` with `Token` set, placed after the decks' objects in the order it was made, so it gets the object index the game gave it. The shadow finds its table token by the view's name ("Goblin Token"), printed power and toughness, against the table's script names (`r_1_1_goblin`); zero or several matches is `token`.
- Attached Auras and Equipment (C5b), on a creature of either seat: `WorldObj.Attached` and `Host` (the bearer's object index), staged by `attachTo`. The card must be an Aura or Equipment whose abilities are statics that commute (a P/T or keyword grant, can't attack, can't block, no condition) plus the Equip ability. An attachment's layer timestamp is the order the staging attaches in, the battlefield list order, not the order the game did, which an observation does not show; that is exact for the statics admitted and is why only commuting ones are.
- A sorcery or instant whose whole spell ability creates tokens, in any hidden zone, the graveyard or the hand (a card carries no state there).
- Library, hand, graveyard and battlefield order, tapped and summoning-sick marks, life, land drop, turn, step and priority holder.
- The stack (C6), public and shown in `view.View.Stack`: spells with no target, mode, X or kicker (a token-making instant or sorcery, a creature, an Equipment; an Aura targets and is refused) and activated self-counter abilities ("{1}, sacrifice another creature: put a +1/+1 counter on this") whose source is a battlefield creature. A spell is an object in the stack zone (`WorldStack`, `dirac/zone_stagestack.go`); its cost shows in the tapped lands, the sacrificed creature and the graveyard. Priority over a non-empty stack: a stack whose top entry is not the active seat's is refused as `priority`, whoever holds priority. Such a round may have started at that entry's controller after a cast or activation (no pass) or at the active seat after a resolution (also no pass), and the view cannot say which; the two differ by one pass once the active seat passes. When the top entry is the active seat's, the round is settled: the active seat holds it with 0 passes, any other holder with 1.
- Combat before damage (C7, `dirac/zone_stagecombat.go`, `dirac/shadow/stage_combat.go`): a priority round at upkeep, draw, first main, beginning of combat, declare attackers (attackers declared, each with the player it attacks), declare blockers (blocks declared: each blocker with its attacker and its place in that attacker's block order, an attacker blocked exactly when it has blockers), second main and end; and the two combat decisions at their pose with no pick made, `Pose` `PoseAttackers` (the view's decision is `attackers`) and `PoseBlockers` (`blockers`). The view shows a block only on the attacker (`BlockedBy`, in block order) and an attack only on the attacker (`AttackingPlayer`), so those are what is staged: `WorldObj.Attacking`, `AtkSeat`, `Blocked`, `Blocking`, `Blocks`, `BlockOrd`. Staging sets the attackers' attacking mark and target, the blockers' blocking mark and attacker, the blocked mark, the declared-attackers set, the attackers-this-turn count and the first-strike flag as the declaration left them. A pose leaves the priority holder dirac projects (the seat after the active one, who passed last), not the view's `Priority`, which names the deciding seat.
- A view shows its decision only to the seat that decides it. An observer at the declare attackers step with no attacker, or at the declare blockers step with no block, cannot tell the pose from the priority round after a declaration that made nothing, and is refused as `decision`; a deciding seat sees `attackers` or `blockers` (or `priority`) and is never ambiguous.

Everything else is refused with a reason. dirac's reasons: `table`, `seats`, `object-layout`, `card`, `zone`, `control`, `place`, `step`, `priority`, `turn`, `life`, `battlefield`, `token`, `attachment`, `stack-entry`, `combat`. The shadow adds: `game-over`, `seats`, `stack`, `decision`, `pending`, `unknown-card`, `unmapped-object`, `hidden`, `exile`, `counter`, `mark`, `pool`, `step-name`. Each has a test that provokes it (`TestUnstageableReasons`, `TestWorldFromViewReasons`).

Still refused, as typed reasons: a trigger on the stack, a stack entry with a target, mode, X, kick or a copy (`stack`; dirac's `stack-entry` for a card it cannot rebuild as an entry), any trigger waiting to be put on the stack (`pending`), and priority over a stack whose top entry is not the active seat's (`priority`). Delayed triggers, replacement and prevention shields and per-source remembered memory (`SrcMem`, `RemPersist`, `prevNext`) are not observable and are made only by cards `stageableCard` already refuses (`card`), so no position holding one stages. Widening them needs a card of that kind in the admitted set and a `View` field that shows its state, which the view does not carry today.

Still refused after C7: the combat damage and end of combat steps (`step`; the view's `combat-damage` is both dirac's first-strike and regular damage steps, and which one it is leaves no trace when a first striker has already died; a blocker whose attacker has left combat still counts as blocking, CR 506.4, and the view names blocks only on the attacker, so it shows nothing); a creature attacking a planeswalker or a battle (`mark`); a blocker listed under two attackers, one off the battlefield, or an attacker blocked with no blocker listed (`combat`); combat marks off a combat step or on a non-creature (`combat`); a declaration partly made (dirac asks attackers and blockers one pick at a time, gorge's view poses the whole declaration); a target decision (the target of a stack entry is `stack`). `combat` is dirac's, new in C7.

Still refused: copies (`token`), any other counter kind, a zero count, or a counter or damage off the battlefield (`counter`, `mark`), an Aura or Equipment of another shape, attached to a non-creature or a player, or an Aura on the battlefield attached to nothing (`attachment`), a token that is not a plain creature token or that the table cannot name (`token`). Both new reasons are dirac's (`dirac.ReasonToken`, `dirac.ReasonAttach`); the shadow reuses them.

A token that has left the battlefield leaves its object index behind in the game that made it (dirac reuses an index only when the table is full). The view cannot show it, so a gap before a shown token is refused as `token`; a token that left after the last shown one leaves no gap, and the staged game simply holds fewer objects than the one that made it. The verifier skips those positions and counts them.

Not carried: the RNG draw count (the verifier masks `Projection.Draws`) and the per-card in-hand history.

## How it is proven

`TestStageMatchesReplay` (`dirac/shadow/stage_test.go`):

1. Play 200 lockstep games on a pool of vanilla lands, 14 vanilla or keyword creatures and Hungry Ghoul. Odd-numbered games wait for a position where a battlefield creature carries damage or a counter.
2. At a random step, build the seat's `view.View` from gorge's game and stage a `State` from it, with the true hidden zones as `Hidden`.
3. Require `oracle.Project` equality with the dirac state replayed from the same prefix.
4. Continue both on one shared policy to game end and require zero mismatches.

Last run (C5): 190 of 200 games staged a position (426 tried; 219 refused at a non-quiet step, 8 as priority, 7 as decision and 2 as stack-entry), 41 staged positions held damage and 66 held +1/+1 counters, 95,060 continuation decisions, 0 mismatches. The test fails if fewer than 20 staged positions hold each mark.

`TestStageCardsMatchReplay` (`dirac/shadow/stage_cards_test.go`) is the same verifier on a corpus pool from the `mbc` set, every card supported with evidence: Plains and Mountain, eleven creatures with combat keywords, Pacifism (an Aura), Swiftfoot Boots (an Equipment) and Dragon Fodder (two Goblin tokens). A quarter of the games wait for a position holding an attachment and a quarter for one holding a token. Last run (C6): 176 of 200 games staged a position, 78 staged positions held an attachment, 122 a token and 54 a spell on the stack (a token-making instant, a creature, an Equipment), 111,088 continuation decisions, 0 mismatches. The test fails if fewer than 20 positions hold each. The vanilla pool of `TestStageMatchesReplay` stages a Hungry Ghoul ability on the stack: 48 positions, 190 of 200 games staged, 95,060 continuation decisions, 0 mismatches, fails below 20. Games that wait for a stack position may never see a stageable one, so the staged floor of the corpus pool is 160.

`TestStageMutationsAreCaught` drops each of tapped, summoning-sick, damage, +1/+1 counters, life, land drop, library order, graveyard order and turn from the staging in turn, drops a stack ability, on the vanilla pool drops the attackers, the blocks, a block order and the pose and moves the step (`combatMutations`), and on the corpus pool drops an Equipment's attachment, a stack spell (to the graveyard), moves an Aura to another creature and drops the tokens, and requires the verifier to fail on each. A feature that staging silently dropped would be caught.

`TestStageCombatMatchesReplay` (`dirac/shadow/stage_combat_test.go`) is the C7 verifier on the vanilla pool: 320 games, each waiting from a random decision for one of eight kinds of position (priority at the beginning of combat, with attackers declared, with blocks declared, with or without blocks at declare blockers, the declare attackers decision, the declare blockers decision, the first decision of a turn and the last). The deciding seat's view is staged (a priority position is staged from a random seat's) and held to the replayed state, then both play to the end. Last run: 342 positions tried, 318 staged (40 each of the declare attackers decision, the declare blockers decision, priority at the beginning of combat, with attackers declared, at the first decision of a turn and at its last; 78 at declare blockers), 24 refused as `decision` (an observer at the declare blockers step with no block), 164,396 continuation decisions, 0 mismatches. The test fails below 20 staged positions of any kind. The first-decision-of-a-turn position is upkeep priority of turn 2 or later; the last is the end step's second pass.

## Fork and rollout (chunk C8)

`dirac.Rollout` (`dirac/driver.go`) owns one working `State` on its own `ForkPool`. `Fork(src)` is `State.CopyFrom(src)` into it; `src` is never written. `Run(src, pol, max)` forks and plays the fork with a caller-supplied `Picker` (`Pick(*Decision) Action`; `RandomPolicy` fits) and returns a `RolloutResult`: exactly one of `Over` (with `Winner`, -1 a draw), `Stalled` (with `Reason`) or `Truncated` (the decision cap). A stall is never a draw: `Rollout.Stalls[reason]` counts it and the search must drop that sample. The loop allocates nothing; the working `State` and the decision buffer are allocated once. A `Rollout` is for one goroutine. `dirac/shadow` and a search use it through the public API.

Tests (`dirac/fork_rollout_mp_test.go`): `TestForkLeavesOriginal`, `TestRolloutFourSeatsEnds`, `TestRolloutStallIsCountedNotDraw`, `TestRolloutTruncates`, `TestRolloutDeterministic`, `TestRolloutStepAllocsZero`.

## Extending it

Each later chunk widens the refused set and keeps the same verifier, run on a pool that uses the new feature.

| Chunk | Adds |
|---|---|
| C5 (partly done: damage, +1/+1 counters; C5b: attached Auras and Equipment, plain creature tokens) | other counter kinds, attachments with abilities beyond a static grant, tokens with abilities, copies |
| C6 (partly done: spells and self-counter abilities on the stack) | triggers on the stack and pending, targets and modes on stack entries, delayed triggers, replacement effects, per-source remembered memory |
| C7 (partly done: priority in combat up to the declare blockers step, the attackers and blockers decisions at their pose) | combat damage and end of combat priority (needs the view to show blocks from the blocker's side and the first-strike step), a declaration partly made, target decisions |
| C8 | `CopyFrom` fork and rollout loop (landed, below) |
| C13 | four-seat pods |

A new feature needs: a field on `World` (or `WorldObj`), the staging code in `zone_stage.go`, the mapping in `shadow.WorldFromView`, a mutation in `TestStageMutationsAreCaught`, and a refusal reason for the cases it still cannot rebuild. Keep `dirac.State` rules: no references, no allocation in the transition loop, no growth of `stateCore`.

The SpellBench glue (the candidate mapping, the agent) lives in the private companion module, behind a build tag, and imports this package at a pinned gorge commit.
