package kop

// ExtEnterCtrPick: an as-enters "enters with your choice of two different
// counters on it from among X, Y and Z" (CR 614.1c, 614.12; gorge's ETB
// PutCounter with ChooseDifferent$ 2 over a three-kind CounterType$ list:
// Grimdancer's menace, deathtouch and lifelink). The entering permanent's
// controller picks two of the three kinds, one pick per decision (gorge's
// two-pick "counter_kinds" KChoose, a ChRemCtrKind decision, mode = the
// kernel counter kind: every kind first, then the two left), asked once the
// permanent is on the battlefield; the permanent then has one counter of each
// picked kind. The kinds are B (the first), C's low byte (the second) and C's
// high byte (the third), in the card's printed order. Only the lone
// instruction of an enters program. dirac/repl_enterctrpick.go.
const ExtEnterCtrPick ExtOp = 288

func init() { extOpNames[ExtEnterCtrPick] = "EnterCtrPick" }
