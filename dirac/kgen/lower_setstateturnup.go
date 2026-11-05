package kgen

import "strings"

// setStateTurnUp lowers api:SetState Mode$ TurnFaceUp (gorge effSetState's
// turnUp arm, effects/resolve.go) as OpExt ExtTurnFaceUp: no cost is paid,
// the face-down battlefield permanent turns face up (events.TurnFaceUp, so
// its turned-face-up triggers fire) and one that is no longer face down, or
// whose printed face is an instant or sorcery, is left alone.
//
// Two shapes lower:
//   - ValidTgts$ alone: one battlefield target (Expose the Culprit's first
//     mode), C 0.
//   - Choices$ (with Optional$ True or without it, gorge's pick is optional
//     unless Mandatory$): C 1, the resolving controller picks up to one
//     permanent matching the filter that would turn up (gorge setStateChoose's
//     "setstate_pick", Min 0 Max 1; Hustle // Bustle).
//
// Defined$, Amount$, MinAmount$, Mandatory$, TargetMin$/TargetMax$,
// RememberChanged$, RevealFirst$, ValidNewFace$ and Optional$ beside a
// target (gorge's yes/no ask) refuse.
func (lc *lowerCtx) setStateTurnUp(ps *params) error {
	for _, k := range []string{"Defined", "Amount", "MinAmount", "Mandatory", "TargetMin", "TargetMax", "RememberChanged", "RevealFirst", "ValidNewFace"} {
		if _, ok := ps.get(k); ok {
			return refuse("SetState TurnFaceUp %s$", k)
		}
	}
	ps.consume("ChoiceTitle")
	opt, hasOpt := ps.get("Optional")
	if ch, ok := ps.get("Choices"); ok {
		if _, tg := ps.get("ValidTgts"); tg {
			return refuse("SetState TurnFaceUp Choices$ with ValidTgts$")
		}
		if hasOpt && strings.TrimSpace(opt) != "True" {
			return refuse("SetState TurnFaceUp Optional$ %q", opt)
		}
		f, err := lc.filter(strings.TrimSpace(ch))
		if err != nil {
			return err
		}
		lc.emit(lInstr{Op: "TurnFaceUp", F: f, C: 1})
		return nil
	}
	if hasOpt {
		return refuse("SetState TurnFaceUp Optional$ without Choices$")
	}
	if _, ok := ps.get("ValidTgts"); !ok {
		return refuse("SetState TurnFaceUp without ValidTgts$ or Choices$")
	}
	obj, err := lc.objOrTarget(ps, "", zBattlefield)
	if err != nil {
		return err
	}
	if obj.Kind != rTarget {
		return refuse("SetState TurnFaceUp of %s", obj)
	}
	lc.emit(lInstr{Op: "TurnFaceUp", Obj: obj})
	return nil
}
