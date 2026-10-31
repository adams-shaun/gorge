# A4 deck-shape posture: aggro / midrange / control from the own-deck manifest

Date: 2026-10-20.
Outcome: **implemented and armed only on the experimental `forge-heuristic`
policy.** The default `bot` is unchanged (`TestHeads` untouched) and no win
rate is claimed.

## Forge behaviour (clean-room)

Forge fixes a binary posture per game: "non-aggro" when the deck name contains
"Control" or the average mana value over distinct nonland main-deck entries is
greater than 3; otherwise aggro (the default). It reads the posture in exactly
one place, the attacker's "not needed as blockers" pass, and only when some
potential attacker could block the opponent's next-turn attackers:

- aggro keeps a creature home only when the predicted crack-back without it
  would kill us;
- non-aggro applies a stricter "danger" test (the crack-back would leave us
  below `AI_IN_DANGER_THRESHOLD = 4`) and releases blockers strongest-first
  only when the extra life lost without one is at most its own attack damage.

The aggression ladder, blocking, casting and holding mana are profile
properties, not posture, so Forge has no counterpart to R4 below. References
read for behaviour only: `PlayerControllerAi.java:75-77,1465`;
`Deck.java:804-824`; `AiAttackController.java:339,393-450,862`;
`ComputerUtil.java:3207-3260`; `ComputerUtilCombat.java:392,468`;
`Default.ai:7,78`.

## R1: deck shape

`botpolicy.ShapeOf(*deck.Manifest)` classifies the deciding seat's own
manifest. Copies are weighted (Forge's distinct entries are not), the deck
NAME is never read, and there are three bands instead of two so Midrange is
exactly inert:

- `avg100 = 100*S/N` (integer floor), where `S = Σ Curve[i].CMC*Curve[i].Count`
  (lands sit in the 0 bucket) and `N = Σ Main[i].Count − Σ{Main[i].Count :
  Main[i].Land}`.
- `ShapeAggro` when `avg100 <= 225` (average nonland card at most 2.25).
- `ShapeControl` when `avg100 >= 301` (Forge's ">3" on the exact mean).
- `ShapeMidrange` otherwise, and whenever `m == nil`, `len(m.Curve) == 0`,
  `N <= 0`, or `Σ Curve counts != Σ Main counts`. An absent or inconsistent
  curve never reads as Aggro.

Informational (INFERRED from the repo's deck lists): the `uw-control` deck
averages about 2.64, so it classifies Midrange. The thresholds were not tuned
to move it.

## R2: aggro releases the AR4 blocker (manual path)

AR4 holds back the best blockable attacker whenever the declaration leaves no
untapped blocker and any opponent has a threat. With `Posture.Attack` armed,
an aggro shape skips that hold-back exactly when our life total is strictly
greater than the unblocked crack-back `crack`: the sum of `Power` over every
creature not controlled by us with positive power and no Defender, tapped or
not, doubled for Double Strike. AR4 fires only when the attack leaves no
untapped blocker, so the whole crack-back is unblocked -- Forge's "serious
danger" reading. A missing life fact never skips. Midrange and Control leave
AR4 unchanged.

R2 is AND-ed into AR4's trigger; the AR5/AR7/AR8 exemptions and the
MaxAttackers cut are unchanged. The attack simulation's base answer
(`chooseAttackersMode`) therefore sees R2 too, as intended.

## R3: attack-sim life weights (attack-sim path only)

`chooseAttackersSim` scores each candidate with `scorePosture`:

- Aggro: own life 100%, opponent life 150%.
- Control: own life 150%, opponent life 100%.
- Midrange or unarmed: both 100%, which is the unchanged `score`.

`scorePosture` adds the weighted life deltas to the unchanged `score`; it
returns `score`'s win/loss sentinel when either side is dead. `score` itself
is untouched. On a 45,000-board synthetic grid, the 150% weight changed the
simulated answer on 5072 boards for Aggro and 2062 for Control: firing
evidence, not a frequency in real play.

The simulated plies build Boards without `Posture`/`OwnDeck`, so predictions
stay posture-free by design.

## R4: reserve preference (cast; gorge extension)

In `chooseCast`, C7's `res * ReserveScale` uses `b.reserveScale(w)` when
`Posture.Reserve` is armed:

- Aggro: `w.ReserveScale * 0` (taps out);
- Control: `w.ReserveScale * 2` (keeps instant mana up);
- Midrange: `w.ReserveScale` (unchanged).

Only C7's bonus size moves; A1, A3, C8, C9 and CR1 are untouched, as are the
tap gate and `ReserveScale` itself.

## Data flow and arming

No new board fact. `Board.OwnDeck` is already filled by both adapters
(`BoardFromGame` and `BoardFromView`) and pinned `reflect.DeepEqual` over a
whole game; `ShapeOf` is pure, so adapter parity follows. `Board.Posture` is
policy configuration like `Targets`/`Removal`/`AIHints`, filled only by the
arming constructor. `heuristic.Bot.EnablePostureRules`/`PostureRules()` carry
it, and `forgeheuristic.New` arms `{Attack: true, Reserve: true}`. The default
`bot` never sets it.

`forge-heuristic` is seated as a manual `heuristic.NewBot` in a bench runner,
which builds no attack simulation, so R2 and R4 fire on that path
and R3 does not. R3 fires for the policy only on an attack-sim seat, which
the hosted auto-pay tables get after F0.

## Tests

`bots/botpolicy/posture_test.go` pins the three bands and both boundaries,
R2's release and lethal-crack-back hold, all four R3 sim rows, and the four R4
reserve rows. `bots/heuristic/posture_rules_test.go` pins that the rules
travel on the Board for both the manual and attack-sim bots, and the
`posture` entry in `TestBotAdaptersAgreeOverWholeGame` pins adapter parity
over a whole game.
