# Staging a state from an observation (the shadow)

A bot attached to an external engine (a SpellBench v2 host, later Forge or XMage) holds only what its own seat sees.
To search it needs a complete world: a guess at every hidden zone. The shadow turns that observation into a dirac
`State` the kernel can step, so a dirac policy or rollout search can answer, and the answer can be mapped back onto
the external engine's candidates.

Two rules shape it.

- **No omniscience, by construction.** The input has no field that can hold an opponent's hand or a library's identity
  beyond what the caller deals. `dirac/shadow` imports `dirac`, `dirac/kop` and `engine/view` only; it never reads
  `engine/state` or the search `Clairvoyant` option. A gorge game enters only in tests, as the verifier.
- **Unstageable is an error, not a guess.** A position outside the supported scope returns a typed
  `dirac.ErrUnstageable{Reason}`. The caller falls back to another policy and counts the reason.

## The two layers

| Layer | Where | Does |
|---|---|---|
| `dirac.NewFromWorld(*World)` | `dirac/zone_stage.go` | Builds a `State` from a `World`: a flat list of cards (owner, zone, place, controller, tapped, summoning-sick) plus life, land drop, turn, step and priority holder. Refuses anything it cannot rebuild exactly. |
| `shadow.WorldFromView`, `shadow.StageFromView` | `dirac/shadow/stage.go` | Builds a `World` from the seat's own `engine/view.View` plus a caller-supplied `Hidden` (the opponent's hand and each library, as card names). Checks the dealt zones against the counts the view shows. |

`Hidden` is the caller's: a determinization deals it from the seat's own deck list and the view's counts, and the
verifier supplies the true zones. Staging cannot tell which.

## Staging from a seat's own view

`shadow.NewLayout(tbl, lists)` takes the game's open deck lists, one card name per copy and one list per seat (every
seat's list is known, the open-list assumption a determinization deal makes). `(*Layout).Deal(v, cfg, seed)` then
builds both halves of an `Input` from the view and those lists alone: the `Hidden` deal and the object numbering
`Input.ObjOf` exposes. Seat `s`'s cards take `base_s .. base_s+len(lists[s])-1`, the sum of the earlier lists. The
physical cards the view shows take the low indices of that range in a fixed order (every player's battlefield for the
seat's cards, its graveyard, exile, command zone, stack spells, then the viewer's hand); the unseen cards fill the rest
in list order, the first `HandSize` of another seat's becoming its hand and the remainder its library. Current tokens
are compacted after every deck, in first-seen order. A hidden multiset that is not exactly the count the view shows is
`hidden`.

The initial hidden order is deliberately arbitrary. `dirac/search` re-deals every zone a seat cannot see at each
simulation (`redeal`), so the order carries no information and no guess. The deal reads only the view and the lists, so
two `Layout`s over the same lists and views stage identically.

## What stages

Two, three or four seats (`dirac.MaxSeats`), every seat still in the game.

- **Pods.** The same features stage at `NSeat` 2..4. A round starts at the active seat with no pass and each pass moves
  to the next seat, so the pass count is `(Prio + NSeat - Active) % NSeat` (`World.roundPasses`); a pose leaves the holder
  at the seat before the active one with `NSeat` passes. A blocker's controller must be the seat its attacker attacks
  (CR 509.1a). The declare blockers pose stages only while no block is declared yet, with `World.BlkDef` the defender asked
  (dirac's `blkDef`, read only in a pod); a later defender's pose after another's blocks is `combat`. An observer at the
  declare blockers step in a pod is `decision` even with blocks shown, since another defender may still be to declare.
  The declare blockers step with no attacker on the board is `combat` (every attacker left; a blocker of a departed
  attacker keeps blocking, CR 506.4, and the view now shows it as `Blocking`, but only the damage steps and end of combat
  place it).
  A seat that has left the game is `seat-left` (a finished game stays `game-over`); Commander stays refused (`exile`).
  Status of the multiplayer shadow (C13): pods of every seat in the game are done; departed seats and Commander remain.

- **Permanents.** Lands; creatures with combat keywords only, or whose only ability is a no-target, no-once-per-turn
  "put a +1/+1 counter on this" (`selfCounterAbility`); damage marked on a creature (also marked dealt-damage-this-turn,
  as `dmgPerm` records it); +1/+1 counters.
- **Tokens.** Plain creature tokens (`TToken`, combat keywords, no ability), placed after the decks' objects in creation
  order. The table token is found by the view's name, printed power and toughness against the table's script names
  (`r_1_1_goblin`); zero or several matches is `token`.
- **Attachments.** Auras and Equipment on a creature of either seat, when their abilities are commuting statics (a P/T or
  keyword grant, can't attack, can't block, no condition) plus Equip. Layer timestamps follow the battlefield list order,
  which is exact only because the admitted statics commute.
- **Spells in hidden zones.** A sorcery or instant whose whole spell ability creates tokens, in the graveyard or hand.
- **Order and marks.** Library, hand, graveyard and battlefield order, tapped and summoning-sick marks, life, land drop,
  turn, step, priority holder.
- **The stack** (`dirac/zone_stagestack.go`). Spells with no target, mode, X or kicker, and "{1}, sacrifice another
  creature: put a +1/+1 counter on this" abilities whose source is a battlefield creature. Priority over a non-empty
  stack: a stack whose top entry is not the active seat's is refused as `priority`, whoever holds priority. Such a round
  may have started at that entry's controller after a cast or activation (no pass) or at the active seat after a
  resolution (also no pass), and the view cannot say which; the two differ by one pass once the active seat passes. When
  the top entry is the active seat's, the round is settled: the active seat holds it with 0 passes, any other holder
  with 1.
- **Combat** (`dirac/zone_stagecombat.go`, `dirac/shadow/stage_combat.go`). A priority round at upkeep,
  draw, first main, beginning of combat, declare attackers, declare blockers, first-strike damage, damage, end of combat,
  second main and end; and the attackers and
  blockers decisions at their pose with no pick made (`PoseAttackers`, `PoseBlockers`). The view shows a block only on
  the attacker (`BlockedBy`, in block order) and an attack only on the attacker (`AttackingPlayer`), so staging sets the
  attacking, blocking and blocked marks, the declared-attackers set, the attackers-this-turn count and the first-strike
  flag from those. gorge runs one `combat-damage` step with a priority round after the first-strike pass and another after
  the regular pass; dirac has two steps. `view.View.DamageStep` (`"first-strike"` or `"regular"`, set from
  `combatRound` through `view.DamageStepReader`) picks `StepFirstStrikeDamage` or `StepDamage`, and
  `View.FirstStrikeStep` (`combatRound.hasFirst`) sets `World.FirstStrike`, which sets dirac's first-strike flag at those
  two steps (a first striker that died still made the step). A combat damage view with no `DamageStep` is `step-name`.
  After damage a blocker whose attacker left is `CardView.Blocking` with no attacker listing it: `Blocking` with
  `Blocks == dirac.NoObj`; an attacker whose blockers all left stays `Blocked` with none (CR 509.1h). Both are legal only
  from the damage steps on (`combat` before). The first-strike damage step marks the creatures that dealt damage in it
  (`pfDealtFirst`, as `markFogFirst`) so the regular step skips them. At end of combat gorge has reset `combatRound`, so
  `FirstStrike` is false there and dirac keeps `anyFirstStrike`; nothing reads the flag after the damage steps. A pose leaves the priority holder dirac projects (the seat before the active one; at two seats the other seat), not the view's
  `Priority`, which names the deciding seat.

A view shows its decision only to the deciding seat. An observer at declare attackers with no attacker, or at declare
blockers with no block, cannot tell the pose from the priority round after an empty declaration and is refused as
`decision`.

## What is refused

Everything else is refused with a reason. dirac's reasons are `dirac.UnstageableReasons` (`table`, `seats`,
`object-layout`, `card`, `zone`, `control`, `place`, `step`, `priority`, `turn`, `life`, `battlefield`, `token`,
`attachment`, `stack-entry`, `combat`). `shadow` adds the consts in `dirac/shadow/stage.go` (`game-over`, `seats`,
`seat-left`, `stack`, `pending`, `decision`, `unknown-card`, `unmapped-object`, `hidden`, `exile`, `counter`, `mark`, `pool`,
`step-name`). `TestUnstageableReasons` (`dirac`) and `TestWorldFromViewReasons` (`dirac/shadow`) provoke them.

| Refused | Reason |
|---|---|
| a trigger on the stack, or an entry with a target, mode, X, kicker or copy | `stack` (dirac: `stack-entry`) |
| priority over a stack whose top entry is not the active seat's, whoever holds it (the round may have started there or after a resolution) | `priority` |
| any trigger waiting to be put on the stack | `pending` |
| a combat damage view with no `DamageStep` (an engine that is not a `DamageStepReader`) | `step-name` |
| a first-strike damage round of a combat with no first-strike pass; a blocker of nothing, or an attacker blocked with no blocker, before the damage steps | `combat` |
| a creature attacking a planeswalker or battle | `mark` |
| a blocker under two attackers or off the battlefield; an attacker blocked with no listed blocker before the damage steps; combat marks off a combat step or on a non-creature | `combat` |
| a declaration partly made (dirac asks one pick at a time; gorge's view poses the whole declaration), a target decision | `decision` |
| copies; a plain-token mismatch; a gap before a shown token (a token that left the battlefield leaves its index behind and the view cannot show it) | `token` |
| any other counter kind, a zero count, a counter or damage off the battlefield | `counter`, `mark` |
| an Aura or Equipment of another shape, on a non-creature or a player, or an Aura attached to nothing | `attachment` |

Delayed triggers, replacement and prevention shields and per-source remembered memory (`SrcMem`, `RemPersist`,
`prevNext`) are not observable and are created only by cards `stageableCard` already refuses, so no position holding one
stages. Widening this needs a card of that kind in the admitted set and a `View` field that shows its state.

Not carried: the RNG draw count (the verifier masks `Projection.Draws`) and the per-card in-hand history.

## How it is proven

Each verifier plays lockstep games, stages a position from the seat's `view.View` built from gorge's game (true hidden
zones as `Hidden`), requires `oracle.Project` equality with the dirac state replayed from the same prefix, then plays both
on one policy to the end and requires zero mismatches. Each logs its staged, refused and continuation counts and fails
below a floor of staged positions per feature.

| Test (`dirac/shadow`) | Pool and focus |
|---|---|
| `TestStageMatchesReplay` | vanilla lands, vanilla or keyword creatures and Hungry Ghoul; damage, counters, a Ghoul ability on the stack |
| `TestStageCardsMatchReplay` | the `mbc` corpus pool (needs the corpus): Aura, Equipment, tokens, stack spells |
| `TestStageCombatMatchesReplay` | vanilla pool; eleven kinds of combat and turn-boundary position, among them the first-strike and regular damage rounds and end of combat (400 games: 387 staged, 24 first-strike rounds, 36 regular, 36 end of combat) |
| `TestStagePodsMatchReplay` | the vanilla pool at four seats; marks, a stack entry, an attack on a non-adjacent defender (160 games: 160 staged, 50 with marks, 43 with a stack entry, 41 with a non-adjacent attack) |
| `TestStageCombatPodsMatchReplay` | the combat kinds at four seats plus the blockers pose of a later defender (240 games: 239 staged, 45 later-defender poses) |
| `TestStagePodMutationsAreCaught` | seat 3 life, seat 2 library, the attacked seat, the asked defender and the priority holder each bent in turn |
| `TestStageMutationsAreCaught` | drops each staged feature in turn (tapped, damage, counters, life, library order, the stack, combat marks, attachments, tokens) and requires the verifier to fail |

## Fork and rollout

`dirac.Rollout` (`dirac/driver.go`) owns one working `State` on its own `ForkPool`. `Fork(src)` is `CopyFrom` into it;
`src` is never written. `Run(src, pol, max)` forks and plays with a caller-supplied `Picker` (`NewRandomPolicy` fits)
and returns a `RolloutResult`: exactly one of `Over` (`Winner`, -1 a draw), `Stalled` (`Reason`) or `Truncated` (the
decision cap). A stall is never a draw: `Rollout.Stalls[reason]` counts it and a search must drop that sample. The loop
allocates nothing, and a `Rollout` is for one goroutine. Tests are in `dirac/fork_rollout_mp_test.go`.

## The shadow seat

`dirac/shadowseat` is the player over this staging: a gorge `seat.Seat` that searches on dirac. At each priority,
declare-attackers and declare-blockers decision it stages the view it was handed (`Layout.Deal`, then `StageFromView`),
steps the staged `State` to its decision (the config is `LockstepConfig(SurfaceFull)` with `NoElide`, so dirac poses
gorge's decisions one for one), and runs `search.Searcher.SeatSearch`: the heuristic leaf and hbot priors, PIMC over
`Worlds` cached deals. The staged root holds only the Layout's open-list deal, so every world the search deals is a
function of the view and the deck lists; the seat never sets the clairvoyant option and cannot read gorge's hidden zones.

The chosen dirac actions meet gorge's options at the canonical action (`dirac/canon`): pass, land play, plain activation,
plain cast (a planned cast through `Decision.PaymentActions`, which wins over the plain option), attacker, blocker and
target. A cast or activation keeps the staged `State` with the answer applied as a plan, and the target decision that
follows is searched from it. Anything else, and any position the seat cannot stage or map, goes to the fallback seat
(the heuristic bot in the harness). `Stats.Reasons` counts why: `stage/<reason>`, `kind/<kind>`, `continuation/<kind>`,
`decision` and `unmapped/<action>`; `Stats.CardRefusals` keeps the card details of `stage/card`. gorge is the host, so
its own `Submit` checks every answer. The tests are `TestShadowSeatMapsEveryOption`, `TestShadowSeatPlaysHeuristic`,
`TestShadowSeatContinuesTargets`, `TestShadowSeatDeterministic` and `TestShadowSeatImports`.

A closed list (an unknown opponent deck, `ClosedList` with a `search.Belief`) and pods are out of scope: a pod stages
as `stage/seats` and falls back.

## Extending it

A new feature needs a field on `World` or `WorldObj`, the staging code in `zone_stage.go`, the mapping in
`shadow.WorldFromView`, a mutation in `TestStageMutationsAreCaught`, and a refusal reason for the cases it still cannot
rebuild. `dirac.State` rules hold: no references, no allocation in the transition loop, no growth of `stateCore`.

The SpellBench glue (candidate mapping, the agent) lives in a private companion module behind a build tag, which imports
this package at a pinned gorge commit.
