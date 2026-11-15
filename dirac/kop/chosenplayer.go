package kop

// The as-enters chosen opponent (The Rack, Black Vise: K:ETBReplacement
// whose body is `ChoosePlayer | Defined$ You | Choices$ Player.Opponent`).
// gorge runs the body through effChoosePlayer while the permanent enters: a
// KChoose of one "player" option per opponent in AliveFrom(controller) order,
// asked only with more than one (a single one is answered in place, the
// oracle's forcedPlayerChoice), and the answer is the permanent's
// event-backed Object.Chosen, cleared when it leaves the battlefield (CR
// 400.7). The kernel keeps the seat in the permanent's chosen byte.

// ExtChoosePlayer: an as-enters opponent choice (CR 614.12), the first
// instruction of an enters program. The entering permanent's controller
// chooses one of their opponents (a ChGenPlayer decision, mode = the seat),
// kept in the permanent's chosen byte; RefChosenPlayer and WhoseChosen read
// it. dirac/repl_chooseplayer.go.
const ExtChoosePlayer ExtOp = 299

func init() { extOpNames[ExtChoosePlayer] = "ChoosePlayer" }

// RefChosenPlayer is a player ref: the opponent the resolution chose
// mid-resolution (ExtChoosePlayerMid; the seat the resolution holds), else
// the one the resolving entry's source permanent chose as it entered (gorge
// Defined$ ChosenPlayer over the source's Object.Chosen). None once the
// source is no longer that permanent (CR 400.7: gorge clears the choice when
// it leaves the battlefield).
const RefChosenPlayer Ref = 39

// ExtChoosePlayerMid: a mid-resolution opponent choice (Skullwinder: a
// battlefield trigger's DB chain `ChoosePlayer | Defined$ You | Choices$
// Player.Opponent` whose later links read it, gorge effChoosePlayer run
// mid-resolution). The resolving controller chooses one of their opponents
// (ChGenPlayer, mode = the seat, AliveFrom order), asked only with more than
// one (one is recorded unasked, the oracle's forcedPlayerChoice); the seat
// stays in the resolution (res.chPl, seat + 1) and RefChosenPlayer reads it
// while the resolution holds it. dirac/repl_chooseplayer.go.
const ExtChoosePlayerMid ExtOp = 290

func init() { extOpNames[ExtChoosePlayerMid] = "ChoosePlayerMid" }

// WhoseChosen restricts a printed battlefield Phase trigger to the turn of
// the player its source chose as it entered (gorge ValidPlayer$
// Player.Chosen). Read with the listener's slot; elsewhere it fails closed.
const WhoseChosen Whose = 201
