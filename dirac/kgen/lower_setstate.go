package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// setState lowers api:SetState Mode$ Transform (gorge effSetState, rules
// effects/resolve.go) as OpTransform, on a modal layout or a transforming
// double-faced token: gorge flips any multi-face card to face
// (FaceIdx+1) % faces and restamps the layer timestamp for a transforming
// DFC alone (events foldFlipFace IsTransformingDFC). Defined$ Self is
// accepted; an absent Defined$ is Self only when the node declares no
// ValidTgts$ (gorge context.go DefinedRef's fallback). Every other Mode$
// (Flip, TurnFaceUp, TurnFaceDown, Unspecialize, the default), Optional$,
// RememberChanged$, a target or another Defined$ refuses. Mode$ TurnFaceUp
// is lower_setstateturnup.go's.
func (lc *lowerCtx) setState(ps *params) error {
	m := ps.str("Mode")
	if m == "TurnFaceUp" {
		return lc.setStateTurnUp(ps) // lower_setstateturnup.go
	}
	switch {
	case m == "Flip":
		// CR 710: a flip card flips itself (gorge effSetState's Flip arm,
		// setStateFlipGate: a permanent not yet flipped, one-way); the
		// kernel's OpTransform reads the FaceFlip layout for that gate.
		if lc.out.FaceMode != kop.FaceFlip || lc.out.Face != 0 {
			return refuse("SetState Flip on a layout that is not a flip card's top half (%d)", lc.out.FaceMode)
		}
	case m != "Transform":
		return refuse("SetState Mode$ %q", m)
	case lc.out.FaceMode != kop.FaceModal && lc.out.FaceMode != kop.FaceTransform:
		return refuse("SetState Transform on a layout the kernel does not turn (%d)", lc.out.FaceMode)
	}
	d, hasDef := ps.get("Defined")
	if !hasDef {
		if _, ok := ps.get("ValidTgts"); ok {
			return refuse("SetState Transform with ValidTgts$")
		}
		d = "Self"
	}
	if d != "Self" {
		return refuse("SetState Defined$ %q", d)
	}
	for _, k := range []string{"Optional", "RememberChanged"} {
		if _, ok := ps.m[k]; ok {
			return refuse("SetState %s$", k)
		}
	}
	obj, err := lc.objOrTarget(ps, "Self", zBattlefield)
	if err != nil {
		return err
	}
	if obj.Kind != rSelf {
		return refuse("SetState of %s", obj)
	}
	lc.emit(lInstr{Op: "Transform", Obj: obj})
	return nil
}
