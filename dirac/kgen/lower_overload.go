package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// K:Overload:<cost> (CR 702.96a; dirac/kw_overload.go).
//
// An overloaded cast replaces every instance of "target" in the spell's text
// with "each". The kernel has no target-swap pass: kgen emits a second
// program, OverloadProg, whose targeted instructions are rewritten to the
// equivalent untargeted sweep (DestroyAll, DamageAll, ChangeZoneAll,
// PumpAll, OverrideAll, TapAll, PutCounterAll) over the same target filter.
// The overloaded cast runs that program; the printed program is unchanged.
//
// The rewrite is deliberately bounded. An instruction with no sweep
// equivalent, or a later instruction that reads a target slot (a subability's
// "Defined$ Targeted"/"its controller"), refuses the card by name rather
// than silently dropping half of the effect.

// refuseOverloadCombos refuses overload beside any other cast cost route:
// the kernel's overloaded cast pays one plain alternative mana cost of a
// single-face spell (the refuseDashCombos shape).
func (lc *lowerCtx) refuseOverloadCombos() bool {
	out := lc.out
	if out.Overload == nil {
		return false
	}
	if out.Kicker != nil || out.Warp != nil || out.Plot != nil || out.Flashback != nil || out.SpellCost != nil || out.AltAddCost != nil || out.Convoke || out.Improvise ||
		out.Offspring != nil || out.Gift || out.MorphCost != nil || out.Sneak != nil || out.Bestow != nil || out.Impending != nil || out.Mayhem != nil ||
		out.Harmonize != nil || out.Madness != nil || out.Casualty != 0 || out.Bargain || out.OptCost != nil || out.Teamwork != 0 || out.Spree || out.WebSling != nil ||
		out.Dash != nil || out.Evoke != nil || out.MayFlash != nil || out.Delve || out.Storm || out.SpellWaterbend != 0 || out.BeholdN != 0 || out.BeholdExile || out.Miracle != nil {
		lc.refuseNode("K:Overload", "Overload beside an alternative or additional cost")
		return true
	}
	if len(lc.card.Faces) != 1 || lc.card.AlternateMode != "" {
		lc.refuseNode("K:Overload", "Overload on a multi-face card")
		return true
	}
	return false
}

// lowerOverload builds the overloaded cast's program from the printed spell
// ability after the card's lines are lowered (lowerTail).
func (lc *lowerCtx) lowerOverload() {
	if lc.out.Overload == nil || lc.refuseOverloadCombos() {
		return
	}
	ab := lc.spellAbility()
	if ab == nil || len(ab.Prog) == 0 {
		// The printed spell is already refused (or has no effect to rewrite);
		// do not stack a second refusal on the same card.
		return
	}
	prog, err := overloadProg(ab)
	if err != nil {
		lc.refuseNode("K:Overload", "%v", err)
		return
	}
	lc.out.OverloadProg = prog
}

// spellAbility is the card's printed spell ability (the first non-latent
// spell), or nil.
func (lc *lowerCtx) spellAbility() *lAbility {
	for _, a := range lc.out.Abilities {
		if a.Kind == akSpell && !a.Latent {
			return a
		}
	}
	return nil
}

// overloadProg rewrites ab's program to its "each" variant. Any target read
// the rewrite cannot express refuses.
func overloadProg(ab *lAbility) ([]lInstr, error) {
	if len(ab.Prog) == 0 {
		return nil, refuse("Overload with an empty spell program")
	}
	out := make([]lInstr, 0, len(ab.Prog))
	for _, in := range ab.Prog {
		if in.Obj2.Kind != rNone || valueTargets(in.A) || valueTargets(in.B) || condTargets(in.Cond) {
			return nil, refuse("Overload effect %s reads a target outside its subject", in.Op)
		}
		if in.Obj.Kind != rTarget {
			if refTargets(in.Obj) {
				return nil, refuse("Overload effect %s reads %s", in.Op, in.Obj.String())
			}
			out = append(out, in)
			continue
		}
		if in.Obj.N < 0 || in.Obj.N >= len(ab.Targets) {
			return nil, refuse("Overload effect %s names target slot %d", in.Op, in.Obj.N)
		}
		ni, err := overloadInstr(in, ab.Targets[in.Obj.N])
		if err != nil {
			return nil, err
		}
		out = append(out, ni)
	}
	return out, nil
}

// overloadInstr maps one targeted instruction to its sweep form. A filter
// with a player alternative needs the player-sweep cases (Discard); every
// other object sweep refuses it.
func overloadInstr(in lInstr, t lTarget) (lInstr, error) {
	if t.F != nil && t.F.Players != 0 {
		switch in.Op {
		case "Discard":
			// "target player discards" -> "each player discards"
			// (Mind Rake; RefEachPlayer/RefEachOpp order the kernel's
			// player walk like gorge's AliveFrom(controller)).
			in.Obj = lRef{Kind: rEachPlayer}
			if t.F.Players == plOpp {
				in.Obj.Kind = rEachOpp
			}
			in.F = nil
			in.Note = ""
			return in, nil
		default:
			return in, refuse("Overload of %s with a player target", in.Op)
		}
	}
	if t.F == nil || len(t.F.Clauses) == 0 {
		return in, refuse("Overload of %s with no object filter", in.Op)
	}
	in.Obj = lRef{}
	switch in.Op {
	case "Destroy":
		if in.C&^1 != 0 {
			return in, refuse("Overload of Destroy with flags %#x", in.C)
		}
		in.Op = "DestroyAll"
		in.F = t.F
		in.C = (in.C & 1) << 1 // DestroyAll bit 1 is NoRegen (bit 0 RememberDestroyed)
	case "DealDamage":
		if in.C != 0 {
			return in, refuse("Overload of DealDamage with flags %#x", in.C)
		}
		in.Op = "DamageAll"
		in.F = t.F
	case "Bounce":
		if in.C != 0 || t.Zone != zBattlefield && t.Zone != zNone {
			return in, refuse("Overload of Bounce from %s", zoneLabel[t.Zone])
		}
		in.Op = "ChangeZoneAll"
		in.F = t.F
		in.C = zHand
	case "Exile":
		if in.C != 0 {
			return in, refuse("Overload of Exile with flags %#x", in.C)
		}
		if t.Zone != zBattlefield && t.Zone != zNone {
			return in, refuse("Overload of Exile from %s", zoneLabel[t.Zone])
		}
		in.Op = "ChangeZoneAll"
		in.F = t.F
		in.C = zExile
	case "PumpUntilEOT":
		in.Op = "PumpAll"
		in.F = t.F
	case "GrantKeyword":
		in.Op = "PumpAll"
		in.F = t.F
		in.A, in.B = lValue{}, lValue{}
	case "Override":
		if in.B.Kind != vNone {
			return in, refuse("Overload of Animate with a chosen colour")
		}
		in.Op = "OverrideAll"
		in.F = t.F
	case "Tap":
		in.Op = "TapAll"
		in.F = t.F
	case "PutCounter":
		if in.B.Kind == vConst && in.B.N == 2 {
			// RememberPut$: PutCounterAll's own remember arm
			// (kop.PutCtrRememberPut)
			in.B = cst(kop.PutCtrRememberPut)
		} else if in.B.Kind != vNone {
			return in, refuse("Overload of PutCounter with flags %s", in.B.String())
		}
		in.Op = "PutCounterAll"
		in.F = t.F
	case "Goad":
		in.F = t.F
	default:
		return in, refuse("Overload of %s has no each-variant", in.Op)
	}
	in.Note = ""
	return in, nil
}

// refTargets reports whether r names a target slot or a target's property.
func refTargets(r lRef) bool {
	switch r.Kind {
	case rTarget, rTargetController, rTargetOwner:
		return true
	case rTargetUnion:
		return r.N != 0
	}
	return false
}

// valueTargets reports whether v reads a target ref.
func valueTargets(v lValue) bool {
	if refTargets(v.Ref) {
		return true
	}
	for _, s := range v.Sub {
		if valueTargets(s) {
			return true
		}
	}
	return false
}

// condTargets reports whether c reads a target ref.
func condTargets(c lCond) bool {
	if refTargets(c.Ref) || valueTargets(c.A) || valueTargets(c.B) {
		return true
	}
	for _, s := range c.Sub {
		if condTargets(s) {
			return true
		}
	}
	return false
}
