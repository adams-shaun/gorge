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
in list order, the first `HandSize` of another seat's becoming its hand and the remainder its library. Tokens
take the indices after every deck in first-seen order and keep them for the rest of the game: one no view shows any
more is dealt as a ceased token (`Hidden.Ceased`) at its index, with the table name and owner it had when last shown. A hidden multiset that is not exactly the count the view shows is
`hidden`.

The initial hidden order is deliberately arbitrary. `dirac/search` re-deals every zone a seat cannot see at each
simulation (`redeal`), so the order carries no information and no guess. The deal reads only the view and the lists, so
two `Layout`s over the same lists and views stage identically.

## What stages

Two, three or four seats (`dirac.MaxSeats`), every seat still in the game.

- **Pods.** The same features stage at `NSeat` 2..4. A round starts at the active seat with no pass and each pass moves
  to the next seat still in the game, so the pass count is the number of seats not lost from Active up to Prio
  (`World.roundPasses`, `(Prio + NSeat - Active) % NSeat` when none has left); a pose leaves the holder at the last seat
  in the game before the active one with one pass per seat in the game. A blocker's controller must be the seat its attacker attacks
  (CR 509.1a). The declare blockers pose stages only while no block is declared yet, with `World.BlkDef` the defender asked
  (dirac's `blkDef`, read only in a pod); a later defender's pose after another's blocks is `combat`. An observer at the
  declare blockers step in a pod is `decision` even with blocks shown, since another defender may still be to declare.
  The declare blockers step with no attacker on the board is `combat` (every attacker left, and a blocker of a departed
  attacker keeps blocking with no mark a view shows).
  A seat that has left the game stages as `World.Lost`: its view shows no card and no hand or library size (else
  `seat-left`), `Hidden.Gone[p]` holds every card of its deck, and those cards are `ZoneNone` objects that keep their
  indices (CR 800.4a). At least two seats stay, and the active seat, the priority holder, every attacker's target and the
  blockers pose's defender must be among them (else `seats`); a lost seat's ceased token stays `token` until
  gshadow-w1-ceased-tokens. A finished game stays `game-over`.
  Status of the multiplayer shadow (C13): pods of every seat in the game, four-seat Commander pods and departed seats
  are done.

- **Commander (CR 903).** A view whose players list a commander stages as `World.Commander`: one commander a seat
  (`PlayerView.Commanders[0]` -> `World.Cmd`), in the command zone or any zone staging admits, its command-zone casts
  (`CommanderCasts[0]` -> `CmdCasts`, the CR 903.8 tax) and each player's damage from each seat's commander
  (`PlayerView.CmdDamage`, read by the commander's id -> `CmdDamage[damaged][commander's seat]`, CR 903.10a), at 40 life.
  `stageableCard` admits a legendary creature with combat keywords (the legend rule is a state-based action staging never
  performs). `initFromWorld` seats the commanders through `Config.Commanders` (so `InitN` runs `cmdGenesis`), places each
  one where the World says and sets the cast count and tallies; a commander in a graveyard has its CR 903.9a ask marked
  answered (`cmdAsk` = its `ver`), as no quiet position holds the ask pending. A partner pair is `commander`; a second
  card in a command zone is `commander` in dirac and `exile` in the view (a command zone holds its commander only);
  a commander in exile stays `exile`.

- **Permanents.** Lands; creatures with combat keywords only, or whose only ability is a no-target, no-once-per-turn
  "put a +1/+1 counter on this" (`selfCounterAbility`); damage marked on a creature (also marked dealt-damage-this-turn,
  as `dmgPerm` records it); +1/+1 counters.
- **Tokens.** Plain creature tokens (`TToken`, combat keywords, no ability), placed after the decks' objects in creation
  order. The table token is found by the view's name, printed power and toughness against the table's script names
  (`r_1_1_goblin`); zero or several matches is `token`. A token that has ceased to exist (CR 111.7, 704.5d) keeps its
  object index and holds `ZoneNone` (`WorldObj{Token: true, Zone: kop.ZoneNone}`, no mark); the caller names them in
  `Hidden.Ceased` (`CeasedToken{Obj, Name, Owner}`, the name the table token's script name) and they fill the gaps
  among the token indices. A token in a zone other than the battlefield and none stays out of scope.
- **Attachments.** Auras and Equipment on a creature of either seat, when their abilities are commuting statics (a P/T or
  keyword grant, can't attack, can't block, no condition) plus Equip. Layer timestamps follow the battlefield list order,
  which is exact only because the admitted statics commute.
- **Spells in hidden zones.** A sorcery or instant whose whole spell ability creates tokens, in the graveyard or hand.
- **Order and marks.** Library, hand, graveyard and battlefield order, tapped and summoning-sick marks, life, land drop,
  turn, step, priority holder.
- **The stack** (`dirac/zone_stagestack.go`). Spells with no target, mode, X or kicker, an Aura spell of the shape
  `stageableAttachment` admits with its one target (a creature on a battlefield), "{1}, sacrifice another creature: put
  a +1/+1 counter on this" abilities whose source is a battlefield creature, and the Equip ability of an Equipment of
  that shape with its one target (a creature its controller controls). A target is legal at resolution exactly while the
  target object's `ver` is the one the entry carries, so it fizzles when the target has changed zones since; the entry's
  crime bits stay zero, and a table with the crime mechanic refuses a targeted entry (the ledger is not in a view).
  Priority over a non-empty stack: a stack whose top entry is not the active seat's is refused as `priority`, whoever holds priority. Such a round
  may have started at that entry's controller after a cast or activation (no pass) or at the active seat after a
  resolution (also no pass), and the view cannot say which; the two differ by one pass once the active seat passes. When
  the top entry is the active seat's, the round is settled: the active seat holds it with 0 passes, any other holder
  with 1.
- **Combat before damage** (`dirac/zone_stagecombat.go`, `dirac/shadow/stage_combat.go`). A priority round at upkeep,
  draw, first main, beginning of combat, declare attackers, declare blockers, second main and end; and the attackers and
  blockers decisions at their pose with no pick made (`PoseAttackers`, `PoseBlockers`). The view shows a block only on
  the attacker (`BlockedBy`, in block order) and an attack only on the attacker (`AttackingPlayer`), so staging sets the
  attacking, blocking and blocked marks, the declared-attackers set, the attackers-this-turn count and the first-strike
  flag from those. A pose leaves the priority holder dirac projects (the seat before the active one; at two seats the other seat), not the view's
  `Priority`, which names the deciding seat.

A view shows its decision only to the deciding seat. An observer at declare attackers with no attacker, or at declare
blockers with no block, cannot tell the pose from the priority round after an empty declaration and is refused as
`decision`.

## What is refused

Everything else is refused with a reason. dirac's reasons are `dirac.UnstageableReasons` (`table`, `seats`,
`object-layout`, `card`, `zone`, `control`, `place`, `step`, `priority`, `turn`, `life`, `battlefield`, `token`,
`attachment`, `stack-entry`, `combat`, `commander`). `shadow` adds the consts in `dirac/shadow/stage.go` (`game-over`, `seats`,
`seat-left`, `stack`, `pending`, `decision`, `unknown-card`, `unmapped-object`, `hidden`, `exile`, `counter`, `mark`, `pool`,
`step-name`). `TestUnstageableReasons` (`dirac`) and `TestWorldFromViewReasons` (`dirac/shadow`) provoke them.

| Refused | Reason |
|---|---|
| a trigger on the stack, an entry with a mode, X, kicker or copy, a target that is not an object on a shown battlefield (it left; the entry would fizzle) or is a player; and on a table with the crime mechanic, any targeted entry | `stack` (dirac: `stack-entry`) |
| priority over a stack whose top entry is not the active seat's, whoever holds it (the round may have started there or after a resolution) | `priority` |
| any trigger waiting to be put on the stack | `pending` |
| combat damage and end of combat steps (the view's `combat-damage` is both dirac steps, and a blocker whose attacker left still counts as blocking, CR 506.4) | `step` |
| a creature attacking a planeswalker or battle | `mark` |
| a blocker under two attackers, off the battlefield, or an attacker blocked with no listed blocker; combat marks off a combat step or on a non-creature | `combat` |
| a declaration partly made (dirac asks one pick at a time; gorge's view poses the whole declaration), a target decision | `decision` |
| copies; a plain-token mismatch; a gap before a token that no ceased token (`Hidden.Ceased`) fills | `token` |
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
| `TestStageCardsMatchReplay` | the `mbc` corpus pool (needs the corpus): Aura, Equipment, tokens (24 of 200 staged positions hold a ceased one; the `token` refusal count is 0), stack spells, a targeted stack entry (an Aura spell or an Equip activation; 200 games: 168 staged, 28 with a targeted entry) |
| `TestStageCombatMatchesReplay` | vanilla pool; eight kinds of combat and turn-boundary position |
| `TestStagePodsMatchReplay` | the vanilla pool at four seats; marks, a stack entry, an attack on a non-adjacent defender (160 games: 160 staged, 50 with marks, 43 with a stack entry, 41 with a non-adjacent attack) |
| `TestStageCombatPodsMatchReplay` | the combat kinds at four seats plus the blockers pose of a later defender (240 games: 239 staged, 45 later-defender poses) |
| `TestStagePodMutationsAreCaught` | seat 3 life, seat 2 library, the attacked seat, the asked defender and the priority holder each bent in turn |
| `TestStageCommanderPodsMatchReplay` | the `cmd5` pool at four seats, each deck Terrian, World Tyrant (the commander), seven creatures and 32 Forests (120 pods: 114 staged, 107 with a commander in the command zone, 68 on the battlefield, 84 cast, 12 taxed, 36 with commander damage) |
| `TestStageCommanderMutationsAreCaught` | commander damage, cast counts and a command-zone commander moved to its library each bent in turn |
| `TestStageMutationsAreCaught` | drops each staged feature in turn (tapped, damage, counters, life, library order, the stack, combat marks, attachments, tokens, a ceased token's card or revival) and moves or drops a stack entry's target, and requires the verifier to fail |

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

### Measuring it: `shadowbench`

`go run ./dirac/cmd/shadowbench [-pool vanilla|mbc-stage|set:<name>] [-games 100] [-sims 64] [-worlds 4] [-seed 1]
[-max-turns 60]` plays the shadow seat against `heuristic.NewBot` with gorge as the host (`testkit/bench.PlayGame`),
the seat alternating by game, and prints one JSON line: `shadow_wins`, `heuristic_wins`, `draws`, `stalls`,
`win_rate` over the decided games (a stall is outside the denominator) with its 95% Wilson interval, the seat's
`decisions`, `searched`, `continued` and `fallbacks`, the `reasons` census, and the 20 most frequent `card_refusals`.
A game is `seed+g`; the same flags give the same line. A shadow intent gorge's `Submit` refuses is counted in
`refused` and makes the exit status 1. The `set:<name>` pool draws realistic decks from a generated set (two colours
per seat, 17 basics, 23 supported non-land cards of those colours, at most 4 of a name); most of those positions
fall outside the staging scope, so its `reasons` and `card_refusals` rank what to widen next.

First measured lines (100 games, `-sims 64 -worlds 4`, seed 1):

```
{"pool":"vanilla","games":100,"shadow_wins":68,"heuristic_wins":32,"draws":0,"stalls":0,"win_rate":0.68,"wilson_lo":0.5833719774632082,"wilson_hi":0.7633098918454312,"decisions":21546,"searched":17812,"continued":0,"fallbacks":3734,"refused":0,"reasons":{"continuation/choose":102,"kind/choose":111,"stage/combat":3,"stage/pool":534,"stage/priority":96,"stage/stack-entry":5,"stage/step":2883},"card_refusals":[]}
{"pool":"set:fdn","games":100,"shadow_wins":47,"heuristic_wins":53,"draws":0,"stalls":0,"win_rate":0.47,"wilson_lo":0.3751065198210374,"wilson_hi":0.5671131686275227,"decisions":23266,"searched":0,"continued":0,"fallbacks":23266,"refused":0,"reasons":{"kind/arrange":33,"kind/choose":166,"kind/modes":81,"kind/target":297,"kind/trigger_optional":27,"kind/trigger_order":28,"stage/card":9026,"stage/counter":1442,"stage/exile":2697,"stage/hidden":648,"stage/mark":566,"stage/pool":3082,"stage/stack":533,"stage/step":2402,"stage/token":2238},"card_refusals":[["Adventuring Gear: ability",475],["Bloom Tender: ability",349],["Gleaming Barrier: type",280],["Leyline Axe: keyword",220],["Campus Guide: type",191],["Inspiring Paladin: ability",188],["Mild-Mannered Librarian: ability",182],["Goldvein Pick: ability",173],["Slumbering Cerberus: ability",172],["Gnarlid Colony: keyword",163],["Brineborn Cutthroat: keyword",158],["Infestation Sage: ability",146],["Sun-Blessed Healer: keyword",135],["Hare Apparent: ability",131],["Authority of the Consuls: type",126],["Burnished Hart: type",125],["Heraldic Banner: type",125],["Cathar Commando: keyword",123],["Giada, Font of Hope: type",120],["Sire of Seven Deaths: keyword",118]]}
```

On `set:fdn` the seat searched none of 23266 decisions, so its 47 wins are the fallback heuristic playing itself (the
interval holds 50%); the census above is the result.

A closed list (an unknown opponent deck, `ClosedList` with a `search.Belief`) and pods are out of scope: a pod stages
as `stage/seats` and falls back.

## Extending it

A new feature needs a field on `World` or `WorldObj`, the staging code in `zone_stage.go`, the mapping in
`shadow.WorldFromView`, a mutation in `TestStageMutationsAreCaught`, and a refusal reason for the cases it still cannot
rebuild. `dirac.State` rules hold: no references, no allocation in the transition loop, no growth of `stateCore`.

The SpellBench glue (candidate mapping, the agent) lives in a private companion module behind a build tag, which imports
this package at a pinned gorge commit.
