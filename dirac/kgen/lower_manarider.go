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

// Rider kinds stored in lMana.Rider and emitted as kop.Ability.ManaRider.
const (
	riderNone = iota
	riderDeplete
	riderOppGainLife
)

// riderMana reads a mana ability's lowered rider program: the bounded shapes
// the kernel runs at activation (CR 605.3b).
//   - a depletion sacrifice (Peat Bog, Gemstone Mine): SkipIf the source's own
//     counter count is not 0, then Sacrifice the source;
//   - each opponent gains N life (Grove of the Burnwillows).
//
// The counter-on-self rider stays ctrRiderMana's (Twitching Doll); a
// conditional life rider (Avid Reclaimer's Nissa clause) is not this shape.
func riderMana(ab *lAbility) (kind int, ctr string, n int, ok bool) {
	if ab.Kind != akMana {
		return 0, "", 0, false
	}
	switch len(ab.Prog) {
	case 1:
		in := &ab.Prog[0]
		switch {
		case in.Op == "GainLife" && in.Obj.Kind == rEachOpp && in.A.isConst() && in.A.N >= 1:
			return riderOppGainLife, "", in.A.N, true
		}
	case 2:
		skip, sac := &ab.Prog[0], &ab.Prog[1]
		if skip.Op == "SkipIf" && skip.Skip == 1 && sac.Op == "Sacrifice" &&
			sac.Obj.Kind == rYou && sac.A.isConst() && sac.A.N == 1 &&
			sac.F != nil && len(sac.F.Clauses) == 1 && sac.F.Clauses[0].Flags&fSelf != 0 {
			c := skip.Cond
			if c.Kind == cPresent && c.F != nil && len(c.F.Clauses) == 1 && c.Op == opGE && c.A.isConst() && c.A.N == 1 {
				cl := &c.F.Clauses[0]
				if cl.CmpStat == cmpCounters && cl.CmpN == 0 && cl.CmpCounter != "" {
					return riderDeplete, cl.CmpCounter, 1, true
				}
			}
		}
	}
	return 0, "", 0, false
}
