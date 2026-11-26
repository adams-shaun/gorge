package kgen

// modeCyclingAlt lowers an AlternativeCost static over the controller's
// cycling activations: New Perspectives' "As long as you have seven or more
// cards in hand, you may pay {0} rather than pay cycling costs" (ValidSA$
// Activated.Cycling | ValidPlayer$ You | Cost$ Discard<1/CARDNAME> |
// CheckSVar$ X | SVarCompare$ GE7, CR 118.9).
//
// gorge (activate_static_altcost.go abilityAlternateCostFor) replaces a
// cycling or typecycling activation's whole printed cost with the static's
// Cost$ and gates the option on the static's CheckSVar$ (staticCond lowers it
// onto the static's Cond2 beforehand). The one cost shape the kernel's twin
// (build_cyclingalt.go) charges is the discard alone, so any other Cost$ is
// refused. A ValidPlayer$ other than You, a ValidCard$ or an EffectZone$ is a
// scope gorge reads differently (it scopes by the static's controller and the
// battlefield) and keeps the shape refusal.
func (sc *staticCtx) modeCyclingAlt() ([]lStatic, bool, error) {
	sp := sc.sp
	if sp.str("ValidPlayer") != "You" || sp.str("Cost") != "Discard<1/CARDNAME>" {
		return nil, true, refuse("AlternativeCost shape")
	}
	if _, ok := sp.get("ValidCard"); ok {
		return nil, true, refuse("AlternativeCost shape")
	}
	if _, ok := sp.get("EffectZone"); ok {
		return nil, true, refuse("AlternativeCost shape")
	}
	sp.consume("CostDesc")
	sc.st.Kind = "CyclingAlt"
	return nil, false, nil
}
