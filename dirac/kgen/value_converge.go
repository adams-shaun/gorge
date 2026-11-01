package kgen

import "fmt"

// Count$Converge, the number of colours of mana spent to cast
// the spell (gorge effects/count_body_paid.go: ConvergeColours of
// c.Source, the cast spell, kept as it moves stack->battlefield and
// cleared when it leaves the battlefield). Lowered only where the kernel
// reads the cast exactly:
//
//   - a spell's own chain (Arcane Omens): the resolving spell entry;
//   - a self ETB replacement (K:etbCounter, Rancorous Archaic; its
//     CheckSVar$ condition, Steel Exemplar): the resolving spell entering;
//   - a self-ETB trigger body (Sundering Archaic, Transcendent Archaic):
//     bound as it triggers (Tf3ConvergeBound), read while the source is
//     still on the battlefield.
//
// Anything else refuses.
func (lc *lowerCtx) convergeValue() (lValue, error) {
	if lc.replSelf && !lc.cloneRepl {
		return lValue{Kind: vConverge}, nil // its CheckSVar$ condition (Steel Exemplar) or its body
	}
	if lc.ab == nil {
		return lValue{}, fmt.Errorf("Count$Converge outside an ability")
	}
	if lc.ab.Kind == akSpell && !lc.ab.Latent {
		return lValue{Kind: vConverge}, nil
	}
	tr := lc.ab.Trigger
	if tr == nil || !lc.trigBody || lc.ab.Latent {
		return lValue{}, fmt.Errorf("Count$Converge outside a spell, its enters replacement or its ETB trigger")
	}
	self := tr.F != nil && tr.F.Players == 0 && len(tr.F.Clauses) == 1 && tr.F.Clauses[0].Flags&fSelf != 0
	if tr.Event != "ETB" || tr.Batch || !self {
		return lValue{}, fmt.Errorf("Count$Converge in a %s trigger", tr.Event)
	}
	if tr.XBound || tr.SpentBound {
		return lValue{}, fmt.Errorf("Count$Converge beside a bound X or spend")
	}
	tr.ConvergeBound = true
	return lValue{Kind: vConverge}, nil
}
