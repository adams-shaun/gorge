package kgen

import "strings"

// handMoveEachOpp lowers a hand move whose owners are each opponent
// (DefinedPlayer$ Player.Other / Player.Opponent / Opponent: Hunted Wumpus).
// gorge effChangeZoneHandOwners walks the owners in AliveFrom(controller)
// order, each the chooser of their own hand's pick, each pick settled before
// the next owner is asked (handMoveOwnersWalk). The kernel's RepeatEach over
// the opponents (controller first, C 1, as repeatEachPlayers emits it) runs
// the one-owner ChooseMove with the repeated player as its owner (RefEach),
// the shape Worlds Within Worlds lowers to. Only the plain riders lower: a
// remembered set, a chooser other than the owner, a counted take, an
// attacking entry or an Aura's entry election (Charmed Griffin: the parked
// one-card Aura move is not walked per owner yet) refuses.
func (lc *lowerCtx) handMoveEachOpp(in lInstr, counted bool) error {
	if counted {
		return refuse("hand move of each opponent over a counted take")
	}
	for _, fl := range strings.Split(in.CS, "|") {
		switch fl {
		case "from=Hand", "Tapped", "Mandatory", "Confirm":
		default:
			return refuse("hand move of each opponent with %s", fl)
		}
	}
	at := len(lc.ab.Prog)
	lc.emit(lInstr{Op: "RepeatEach", Obj: lRef{Kind: rEachOpp}, C: 1})
	in.Obj = lRef{Kind: rEach}
	lc.emit(in)
	lc.ab.Prog[at].Skip = len(lc.ab.Prog) - at - 1
	return nil
}
