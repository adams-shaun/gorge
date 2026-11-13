package kop

// ExtChooseCardEnter: an as-enters choice of another permanent (CR 614.12;
// gorge's ETB ChooseCard replacement, Defined$ You, Choices$
// Creature.YouCtrl+Other, Mandatory$ True: Dauntless Bodyguard). The entering
// permanent's controller chooses one battlefield object matching F other than
// the entrant (a ChGenChooseCard decision, source and mode = the object),
// asked once the permanent is on the battlefield, as gorge asks it; no ask is
// posed when nothing matches. The answer is kept in the permanent's
// chosen byte as that Obj, and an ability of the card reads it as the
// resolution's chosen set (RefChosen, F2ChosenCard), from the source's last known
// information once a Sac<1/CARDNAME> cost moved it. Only the first
// instruction of an enters program. dirac/repl_choosecard.go.
const ExtChooseCardEnter ExtOp = 297

func init() { extOpNames[ExtChooseCardEnter] = "ChooseCardEnter" }
