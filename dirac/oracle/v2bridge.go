package oracle

import "github.com/adams-shaun/gorge/engine/decision"

// v2bridge.go: the reverse of a frame's canonicalisation, for an environment
// that poses gorge's decision to an agent in another vocabulary (dirac/cmd/sbv2dirac
// builds SpellBench v2 candidates from the shadow's native decision) and must
// hand the agent's pick to the kernel as the canonical action it stands for.
// Each method answers against the frame the last Step posed; ok false means
// the frame offers no canonical action for that native answer.

// CanonOfOption is the canonical action that answers the posed decision with
// native option index idx: the next pick of a multi-pick decision, the whole
// answer of a single-pick one, or the priority entry built from that option
// (including a planned cast that replaced it).
func (g *GorgeSide) CanonOfOption(idx int) (Canon, bool) {
	fr := g.fr
	if fr == nil {
		return 0, false
	}
	if fr.dk == DPriority {
		for _, pe := range fr.prio {
			if pe.opt == idx && (pe.kind == prOption || pe.kind == prPlan && pe.offered) {
				return pe.a, true
			}
		}
		return 0, false
	}
	for i, at := range fr.curAt {
		if at == idx {
			return fr.cur.Options[i], true
		}
	}
	return 0, false
}

// CanonOfPayment is the priority entry that casts through the planner's
// payment action actionID (Intent.Payment.ActionID).
func (g *GorgeSide) CanonOfPayment(actionID string) (Canon, bool) {
	if g.fr == nil || g.fr.dk != DPriority {
		return 0, false
	}
	for _, pe := range g.fr.prio {
		if pe.kind == prPlan && pe.pay.ID == actionID {
			return pe.a, true
		}
	}
	return 0, false
}

// CanonDone is the Done that closes the posed multi-pick decision, when its
// answer so far is whole.
func (g *GorgeSide) CanonDone() (Canon, bool) {
	if g.fr == nil || !g.fr.multi {
		return 0, false
	}
	for i, at := range g.fr.curAt {
		if at == -1 {
			return g.fr.cur.Options[i], true
		}
	}
	return 0, false
}

// CanonOfIntent is the canonical action for a native intent answering the
// posed decision: a priority option or payment, or the single option of a
// single-pick decision. A multi-pick answer is walked pick by pick through
// CanonOfOption and CanonDone instead.
func (g *GorgeSide) CanonOfIntent(in decision.Intent) (Canon, bool) {
	switch {
	case in.Payment != nil:
		return g.CanonOfPayment(in.Payment.ActionID)
	case len(in.Choices) == 1:
		return g.CanonOfOption(in.Choices[0])
	}
	return 0, false
}

// Multi reports whether the posed decision is answered pick by pick and
// closed by Done.
func (g *GorgeSide) Multi() bool { return g.fr != nil && g.fr.multi }

// MidPick reports whether a multi-pick answer is under way, so the current
// canonical decision is not the pending decision's first.
func (g *GorgeSide) MidPick() bool { return g.fr != nil && len(g.fr.chosen) > 0 }
