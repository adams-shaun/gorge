package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DrawnThisTurn (gorge's DrawnThisTurn predicate, engine/effects/filter.go):
// Object.DrawnTurn == Game.Turn, stamped by events.Apply's Draw fold and
// cleared by every other move but one onto the stack. The kernel keeps the
// stamp as one object-width arena mask, mechDrawn (0 B in State), laid out
// only for a table whose filter programs read FwDrawnThisTurn (drawnUses):
// drawExcl sets the drawn card's bit, placeFromK clears it on any other move
// (CR 400.7: a new object nobody drew), and the turn change clears every bit
// (newTurnGen: gorge compares the stamp against Game.Turn). Sylvan Library's
// hand ChooseCard reads it.

// DebugDrawnAnyHand is the mutation switch (dirac/oracle
// TestSylvanLibraryLockstep): FwDrawnThisTurn admits every hand card, so a
// Sylvan Library choice offers cards gorge does not and the lockstep must
// diverge.
var DebugDrawnAnyHand bool

// drawnUses marks mechDrawn used when a filter program of t reads
// FwDrawnThisTurn.
func drawnUses(t *kop.Table, u *[numMech]bool) {
	for i := range t.FilterProgs {
		for _, l := range t.FilterProgs[i].Lits {
			if l.W == kop.FwDrawnThisTurn {
				u[mechDrawn] = true
				return
			}
		}
	}
}

// fwDrawnThisTurn: FwDrawnThisTurn. The card matcher reads hand cards only:
// a drawn card that moved anywhere else but the stack lost the stamp in gorge
// too, and the kernel's stack spell reads no card matcher.
//
//kmanifest:fword FwDrawnThisTurn
func (s *State) fwDrawnThisTurn(c *fctx, _ *kop.FLit) bool {
	if !c.card() || c.o == NoObj || c.zone != kop.ZoneHand {
		return false
	}
	return DebugDrawnAnyHand || s.mObj(c.k, mechDrawn, c.o)
}
