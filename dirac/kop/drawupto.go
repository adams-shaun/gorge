package kop

// ExtDrawUpto: a player draws up to B cards (gorge effDraw's Upto$ True: the
// drawer is asked how many, a Min 0 / Max min(B, library size) KChoose over
// the top cards of their own library, and draws that many; Arcane Denial's
// "may draw up to two cards"). Obj is the drawer, which may be a player other
// than the resolving controller. An empty library asks nothing. One
// DkChoose ChDrawUpto pick per card, Done ends the count. dirac/act_drawupto.go.
const ExtDrawUpto ExtOp = 309

func init() { extOpNames[ExtDrawUpto] = "DrawUpto" }

// DelayRemUpkeep (OpDelayedProgram A): the next upkeep of any player, with
// the player the registering instruction's Obj names as the fired program's
// trigger player (RefTriggerPlayer): the countered spell's controller
// (gorge Phase$ Upkeep, ValidPlayer$ Player, RememberObjects$
// RememberedController; Arcane Denial). One-shot, no remembered object.
// dirac/trig_delayremupkeep.go.
const DelayRemUpkeep = 10
