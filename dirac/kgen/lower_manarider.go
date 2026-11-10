package kgen

// A mana ability whose rider puts counters on its own source (Twitching
// Doll: "{T}: Add one mana of any color. Put a nest counter on this").
// CR 605.3b: the rider resolves with the mana, at activation. gorge's
// payment planner defers such a source (pay.PaymentPlanAbilityTier: a
// SubAbility$ chain on a tap-only cost that is neither a damage nor a
// self-return rider classifies "source:rider"), so autopay never taps it;
// its census gap is relaxable (one mana per colour, once), and only a
// potential play's prefix search or the manual window activates it. The
// kernel models exactly that for a plain one-mana-of-any-colour {T} source
// (the Taps-deferred route, dirac/mana_tapsdefer.go), and runs the rider
// wherever it taps the source for mana (dirac/mana_rider.go).

// ctrRiderMana reports whether a is that shape: a plain unrestricted {T}
// ability adding one mana of any colour, its program exactly "put N >= 1
// counters of one kind on this permanent", with no condition.
func ctrRiderMana(a *lAbility) bool {
	if a.Kind != akMana || len(a.Prog) != 1 || a.Mana == nil {
		return false
	}
	in := &a.Prog[0]
	if in.Op != "PutCounter" || in.Obj.Kind != rSelf || !in.A.isConst() || in.A.N < 1 || in.B.Kind != vNone ||
		in.CS == "" || in.F != nil || in.Cond.Kind != cNone || in.Skip != 0 {
		return false
	}
	c := *a
	c.Prog = nil
	return plainTapMana(&c, false) && unrestrictedOne(&c) && c.Mana.Colors == "WUBRG" && !c.Mana.Each
}

// refuseCtrRiderBeside refuses a counter-rider mana ability beside another
// mana ability: the kernel defers the source as one unit, which a second
// ability gorge prices normally would not be.
func (lc *lowerCtx) refuseCtrRiderBeside(ms []*lAbility) bool {
	if len(ms) < 2 {
		return false
	}
	for _, a := range ms {
		if ctrRiderMana(a) {
			lc.refuseNode("card", "mana ability with a counter rider beside another mana ability")
			return true
		}
	}
	return false
}
