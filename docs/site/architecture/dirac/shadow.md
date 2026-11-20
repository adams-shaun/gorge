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

Two seats only.

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
  stack: the top entry's controller has passed no one (0 passes); any other holder was passed to (1). The active seat
  holding priority over the other seat's top entry is refused as `priority`, because the view cannot say whether that is
  after a pass or a resolution.
- **Combat before damage** (`dirac/zone_stagecombat.go`, `dirac/shadow/stage_combat.go`). A priority round at upkeep,
  draw, first main, beginning of combat, declare attackers, declare blockers, second main and end; and the attackers and
  blockers decisions at their pose with no pick made (`PoseAttackers`, `PoseBlockers`). The view shows a block only on
  the attacker (`BlockedBy`, in block order) and an attack only on the attacker (`AttackingPlayer`), so staging sets the
  attacking, blocking and blocked marks, the declared-attackers set, the attackers-this-turn count and the first-strike
  flag from those. A pose leaves the priority holder dirac projects (the seat after the active one), not the view's
  `Priority`, which names the deciding seat.

A view shows its decision only to the deciding seat. An observer at declare attackers with no attacker, or at declare
blockers with no block, cannot tell the pose from the priority round after an empty declaration and is refused as
`decision`.

## What is refused

Everything else is refused with a reason. dirac's reasons are `dirac.UnstageableReasons` (`table`, `seats`,
`object-layout`, `card`, `zone`, `control`, `place`, `step`, `priority`, `turn`, `life`, `battlefield`, `token`,
`attachment`, `stack-entry`, `combat`). `shadow` adds the consts in `dirac/shadow/stage.go` (`game-over`, `seats`,
`stack`, `pending`, `decision`, `unknown-card`, `unmapped-object`, `hidden`, `exile`, `counter`, `mark`, `pool`,
`step-name`). `TestUnstageableReasons` (`dirac`) and `TestWorldFromViewReasons` (`dirac/shadow`) provoke them.

| Refused | Reason |
|---|---|
| a trigger on the stack, or an entry with a target, mode, X, kicker or copy | `stack` (dirac: `stack-entry`) |
| any trigger waiting to be put on the stack | `pending` |
| combat damage and end of combat steps (the view's `combat-damage` is both dirac steps, and a blocker whose attacker left still counts as blocking, CR 506.4) | `step` |
| a creature attacking a planeswalker or battle | `mark` |
| a blocker under two attackers, off the battlefield, or an attacker blocked with no listed blocker; combat marks off a combat step or on a non-creature | `combat` |
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
| `TestStageCombatMatchesReplay` | vanilla pool; eight kinds of combat and turn-boundary position |
| `TestStageMutationsAreCaught` | drops each staged feature in turn (tapped, damage, counters, life, library order, the stack, combat marks, attachments, tokens) and requires the verifier to fail |

## Fork and rollout

`dirac.Rollout` (`dirac/driver.go`) owns one working `State` on its own `ForkPool`. `Fork(src)` is `CopyFrom` into it;
`src` is never written. `Run(src, pol, max)` forks and plays with a caller-supplied `Picker` (`NewRandomPolicy` fits)
and returns a `RolloutResult`: exactly one of `Over` (`Winner`, -1 a draw), `Stalled` (`Reason`) or `Truncated` (the
decision cap). A stall is never a draw: `Rollout.Stalls[reason]` counts it and a search must drop that sample. The loop
allocates nothing, and a `Rollout` is for one goroutine. Tests are in `dirac/fork_rollout_mp_test.go`.

## Extending it

A new feature needs a field on `World` or `WorldObj`, the staging code in `zone_stage.go`, the mapping in
`shadow.WorldFromView`, a mutation in `TestStageMutationsAreCaught`, and a refusal reason for the cases it still cannot
rebuild. `dirac.State` rules hold: no references, no allocation in the transition loop, no growth of `stateCore`.

The SpellBench glue (candidate mapping, the agent) lives in a private companion module behind a build tag, which imports
this package at a pinned gorge commit.
