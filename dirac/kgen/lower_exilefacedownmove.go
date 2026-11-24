package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// The blind face-down exile of a ChangeZone or ChangeZoneAll move (Induced
// Amnesia, Triumph of Saint Katherine): ExileFaceDown$ True with no
// WithMayLook$, Foretold$ or DefinedExiler$, so gorge applyMoveFaceDown
// (engine/effects/zone_change.go) stamps the blind exiled_with_face_down
// marker on each moved card (FaceDown, ExiledWith the source, nobody the
// looker). The kernel marks the cards after the move (ExtExileFaceDown,
// dirac/zone_exilefacedownmove.go) from the resolution's remembered set, so
// the move must remember what it moved (RememberChanged$ True) and be the first
// instruction of its program: the set is then exactly the moved cards. Any
// other shape (a non-exile destination, a look, no remember: Inverter of
// Truth's whole-library exile, which would also outgrow the face-down link
// pool) keeps refusing.

func init() { instrOp("ExileFaceDown", (*instrCtx).opExileFaceDown) }

// moveFaceDown reads ExileFaceDown$ on a move to dest; ok reports that the
// caller emits the ExileFaceDown instruction right after the move.
func (lc *lowerCtx) moveFaceDown(ps *params, dest int) (ok bool, err error) {
	if _, has := ps.m["ExileFaceDown"]; !has {
		return false, nil
	}
	switch {
	case ps.peek("ExileFaceDown") != "True", dest != zExile:
		return false, refuse("ExileFaceDown$ %q to %s", ps.peek("ExileFaceDown"), zoneLabel[dest])
	case ps.peek("WithMayLook") != "", ps.peek("Foretold") != "", ps.peek("DefinedExiler") != "", ps.peek("FaceDown") != "":
		return false, refuse("ExileFaceDown$ beside a look, a foretell or an exiler")
	case ps.peek("RememberChanged") != "True":
		return false, refuse("ExileFaceDown$ without RememberChanged$")
	case len(lc.ab.Prog) != 0:
		return false, refuse("ExileFaceDown$ on a move after another instruction")
	}
	ps.consume("ExileFaceDown")
	return true, nil
}

// opExileFaceDown is the kop form: OpExt ExtExileFaceDown.
func (ic *instrCtx) opExileFaceDown() error {
	ic.ki.Op, ic.ki.A = kop.OpExt, kop.Const(int(kop.ExtExileFaceDown))
	return nil
}
