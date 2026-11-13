package kop

// The as-enters chosen player (CR 614.12; The Rack, Black Vise:
// K:ETBReplacement:Other with DB$ ChoosePlayer | Choices$ Player.Opponent).
// gorge records the answer on the entering object (Object.Chosen, cleared
// when it leaves the battlefield); the kernel keeps the seat in the
// permanent's chosen byte (dirac/chosenplayer.go).

// ExtChosenOpp is the enters program's choice of an opponent: the entering
// permanent's chosen byte becomes the one opponent of its controller (a
// two-player game: gorge's KChoose has one option, answered in place). No
// other field.
const ExtChosenOpp ExtOp = 245

func init() { extOpNames[ExtChosenOpp] = "ChosenOpp" }

// RefSrcChosenPl is a player ref: the seat the resolving source permanent
// chose as it entered (gorge Defined$ ChosenPlayer over the source's
// Object.Chosen), read live; none when the source is not on the battlefield.
const RefSrcChosenPl Ref = 39

// WhoseChosenPl: a Phase trigger (printed, battlefield listener only) whose
// ValidPlayer$ is Player.Chosen: the active player is the listener's chosen
// player. An explicit value: kgen's whoChosenPl
// (dirac/kgen/lower_chosenopp.go) must equal it.
const WhoseChosenPl Whose = 201
