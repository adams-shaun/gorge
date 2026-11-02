package kgen

// api Phases (CR 702.26, Spectral Adversary). gorge effPhases phases
// out each affected permanent (the targets of ValidTgts$, or the source by
// default), and every permanent attached to it indirectly with it (702.26g).
// The kernel's Ext Phases does the same for the permanent Obj names; a target
// slot of several objects runs it once per object, in pick order.
//
// Read exactly: ValidTgts$ with TargetMin$/TargetMax$ and no Defined$, or no
// ValidTgts$ and Defined$ Self (or absent). The toggle (PhaseInOrOut$),
// AllValid$ sweeps, the phase-in override Phaseout$, RememberAffected$,
// WontPhaseInNormal$ and Tapped$ are refused, as is K:Phasing (the keyword
// toggle) elsewhere.
func (lc *lowerCtx) phases(ps *params) error {
	for _, k := range []string{"PhaseInOrOut", "AllValid", "Phaseout", "RememberAffected", "WontPhaseInNormal", "Tapped"} {
		if _, ok := ps.get(k); ok {
			return refuse("Phases %s$", k)
		}
	}
	if _, ok := ps.get("ValidTgts"); !ok {
		if d, ok := ps.get("Defined"); ok && d != "Self" {
			return refuse("Phases Defined$ %q", d)
		}
	}
	obj, err := lc.objOrTarget(ps, "Self", zBattlefield)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "Phases", Obj: obj})
	return nil
}
