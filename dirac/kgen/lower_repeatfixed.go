package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// repeatFixed lowers an ungated api:Repeat (gorge effRepeat with neither
// RepeatCheckSVar$ nor RepeatDefined$ nor RepeatOptional$): the body
// (RepeatSubAbility$) runs MaxRepeat$ times, read once as the loop starts and
// clamped to gorge's 1000. The kernel's OpRepeat count is the uint8 loopN
// (the gated loop's 255) and does not clamp: a count of 256 or more wraps, so
// the forms in the corpus must stay below it. The body is the chain below the header, any shape the
// chain lowers: OpRepeat jumps back over it, so a body that asks resumes
// inside its pass. A body that declares a target slot would re-target each
// pass in gorge's one resolution context, which the kernel does not model, so
// it refuses.
func (lc *lowerCtx) repeatFixed(ps *params, n lValue) error {
	if n.Kind == vChosenNumber {
		// Trade Secrets' "repeat as many times as they choose": a ChooseNumber
		// answer read as the pass count diverges from gorge in lockstep
		// (the loop's count after the opponent's number ask)
		return refuse("Repeat MaxRepeat$ %q reads a chosen number", ps.str("MaxRepeat"))
	}
	if ps.is("Mandatory", "True") {
		ps.consume("Mandatory") // gorge effRepeat never reads it: a trigger's no-option rider (Oath of Lim-Dul)
	}
	body := cards.ResolveSVar(lc.face.SVars, ps.str("RepeatSubAbility"))
	if body == nil {
		return refuse("Repeat body %q", ps.str("RepeatSubAbility"))
	}
	at, nt := len(lc.ab.Prog), len(lc.ab.Targets)
	lc.emit(lInstr{Op: "Repeat", A: n, Note: "MaxRepeat$ passes of the body"})
	if err := lc.chain(body, nil); err != nil {
		return err
	}
	if len(lc.ab.Targets) != nt {
		return refuse("Repeat body with a target")
	}
	lc.ab.Prog[at].Skip = len(lc.ab.Prog) - at - 1
	return nil
}
