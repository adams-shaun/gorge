package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

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
//
// Mode$ Transform with Choices$ is a pick, not a Self transform, so the
// card's own layout is not read: setStateTransformPick.
func (lc *lowerCtx) setState(ps *params) error {
	m := ps.str("Mode")
	if m == "TurnFaceUp" {
		return lc.setStateTurnUp(ps) // lower_setstateturnup.go
	}
	if m != "Transform" {
		return refuse("SetState Mode$ %q", m)
	}
	if ch, ok := ps.get("Choices"); ok {
		return lc.setStateTransformPick(ps, ch)
	}
	if lc.out.FaceMode != kop.FaceModal && lc.out.FaceMode != kop.FaceTransform {
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

// setStateTransformPick lowers Mode$ Transform with Choices$ and Mandatory$
// True (Sunder the Gateway's "transform an Incubator token you control"):
// gorge setStateChoose's "setstate_pick", one mandatory KChoose (Min = Max =
// 1, posed with one candidate too, none with no candidate) over the
// battlefield permanents matching Choices$ that would transform (another
// face, not day/nightbound), then the pick transforms. It rides ExtTurnFaceUp
// C 2 (dirac/face_turnupeffect.go), the same ask as the TurnFaceUp pick.
// Without Mandatory$ gorge's pick is Min 0 and refuses here, as do
// Amount$/MinAmount$ (a multi-pick), Optional$, Defined$, ValidTgts$ and
// RememberChanged$.
func (lc *lowerCtx) setStateTransformPick(ps *params, ch string) error {
	for _, k := range []string{"Amount", "MinAmount", "Optional", "Defined", "ValidTgts", "RememberChanged"} {
		if _, ok := ps.get(k); ok {
			return refuse("SetState Transform Choices$ with %s$", k)
		}
	}
	if v := ps.str("Mandatory"); v != "True" {
		return refuse("SetState Transform Choices$ with Mandatory$ %q", v)
	}
	ps.consume("ChoiceTitle")
	f, err := lc.filter(strings.TrimSpace(ch))
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "TurnFaceUp", F: f, C: 2})
	return nil
}
