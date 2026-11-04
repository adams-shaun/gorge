package kgen

// api Phases (CR 702.26, Spectral Adversary). gorge effPhases phases
// out each affected permanent (the targets of ValidTgts$, or the source by
// default), and every permanent attached to it indirectly with it (702.26g).
// The kernel's Ext Phases does the same for the permanent Obj names; a target
// slot of several objects runs it once per object, in pick order.
//
// Read exactly: ValidTgts$ with TargetMin$/TargetMax$ and no Defined$, or no
// ValidTgts$ and Defined$ Self (or absent). Defined$ "Self & Targeted" beside
// ValidTgts$ (U.S.S. Defiant, Mirror Migrant: "it and up to one target
// creature you control phase out") is gorge's Defined list, the source then
// the targets: the source phases out first, then each target; one that has
// already phased out (the source chosen as its own target) is skipped. The toggle (PhaseInOrOut$),
// AllValid$ sweeps, the phase-in override Phaseout$, RememberAffected$,
// WontPhaseInNormal$ and Tapped$ are refused, as is K:Phasing (the keyword
// toggle) elsewhere.
func (lc *lowerCtx) phases(ps *params) error {
	for _, k := range []string{"PhaseInOrOut", "AllValid", "Phaseout", "RememberAffected", "WontPhaseInNormal", "Tapped"} {
		if _, ok := ps.get(k); ok {
			return refuse("Phases %s$", k)
		}
	}
	_, tgts := ps.get("ValidTgts")
	if d, ok := ps.get("Defined"); ok {
		switch {
		case !tgts && d != "Self":
			return refuse("Phases Defined$ %q", d)
		case tgts && d == "Self & Targeted":
			lc.emit(lInstr{Op: "Phases", Obj: lRef{Kind: rSelf}})
		case tgts && d != "Targeted":
			return refuse("Phases Defined$ %q beside ValidTgts$", d)
		}
	}
	tps := ps
	if ps.str("Defined") == "Self & Targeted" {
		// the target slot is the subject of the second Phases
		m := make(map[string]string, len(ps.m))
		for k, v := range ps.m {
			if k != "Defined" {
				m[k] = v
			}
		}
		tps = &params{m: m, used: ps.used}
	}
	obj, err := lc.objOrTarget(tps, "Self", zBattlefield)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "Phases", Obj: obj})
	return nil
}
