package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A spell whose own Cost$ adds nothing to its printed mana cost but a fixed-count
// tapXType part (Fear of Exposure's "tap two untapped creatures and/or lands",
// Guardian of the Great Door's four) is declined by gorge's cast planner
// ("shape:additional_cost", planDeclinesAddCost), but its potential planner prices
// it from the composed cost (rules potential_plan.go spellExtrasAreFixedTaps,
// planComposedCost): each assignment of candidates is withheld from the mana plan,
// so the play is priced only when the plan leaves N candidates untapped. That
// verdict is a proof, so a board whose granted mana (Enduring Vitality's
// creatures) is a census gap runs the prefix search on it: the granted sources
// are tapped in battlefield order until the composed cost prices (CR 601.2g-h).

// spellFixedTapsOnly mirrors gorge's spellExtrasAreFixedTaps for card c's plain
// cast: the spell's additional cost is one fixed-count tap of other permanents.
func spellFixedTapsOnly(k *prep, c CardID) bool {
	if k.t == nil {
		return false
	}
	cd := &k.t.Cards[c]
	if cd.HasAltAdd || cd.AltAddOne || cd.HasModeCost || cd.Cost.X != 0 || cd.Cost.Hyb[0] != 0 {
		return false
	}
	ac := spellAddCost(k, c)
	return ac.Flags == kop.CostTapX && ac.N > 0 && ac.Mana == (kop.ManaCost{})
}

// fixedTapsCast reports a plain cast a of a spellFixedTapsOnly card.
func (s *State) fixedTapsCast(k *prep, a Action) bool {
	return a.Kind() == AkCast && a.Mode() == 0 && a.Target() == 0 && spellFixedTapsOnly(k, s.objs[a.Source()].card)
}

// affordsUngrantedReserving is affordsUngrantedFor with cost part c's tap or
// sacrifice candidates kept apart from the mana (gorge planReserving): some
// assignment of them leaves a mana plan from the printed sources. src is the
// activation's source (NoSlot for a spell).
func (s *State) affordsUngrantedReserving(k *prep, p uint8, cost *kop.ManaCost, ex Slot, d payDesc, c *kop.Cost, src Slot) bool {
	var av avail
	var ms modalSet
	s.ungrAvail(k, p, ex, &av, &ms)
	pm := pipMaskOf(cost)
	for m := 0; m < 1<<ms.n; m++ {
		b := ms.modes(av, m)
		if b.affords(cost, pm, 0) && s.costPartDisjoint(k, p, src, c, cost, &b, pm) {
			return true
		}
		if k.restrSrc && d.class != pdNone && s.restrSrcAffords(k, p&1, d, cost, &b, ex, 0) {
			// restricted mana the plain avail cannot show: keep the old answer
			return true
		}
	}
	return false
}

// restrTapUnit is the restricted unit p's source at x adds for payment d
// (restrSrcAvail's rkExt arm), when d may spend it.
func (s *State) restrTapUnit(k *prep, p uint8, x Slot, d *payDesc) (*pcard, bool) {
	v, kind := s.restrTapView(k, p, x, d)
	return v, kind == rkExt
}

// restrTapView is the restricted view restrSrcAvail counts for p's source at x
// and payment d, with its kind (v nil when there is none).
func (s *State) restrTapView(k *prep, p uint8, x Slot, d *payDesc) (*pcard, uint8) {
	if d == nil || !k.restrSrc || k.gen == nil || k.gen.flags[s.cardAt(x)]&gfRestrSrc == 0 {
		return nil, 0
	}
	v, kind, _ := s.restrView(k, p, x, s.manaPCGen(k, x), d)
	if v == nil {
		return nil, 0
	}
	return v, kind
}

// takeRestrUnit takes out of r the units restrSrcAvail added for p's source at
// x (a reserved tap candidate's mana): an rkExt source's restricted unit, or an
// rkWiden source's widened unit, which replaced its census one (Secluded
// Courtyard's {C} beside its restricted {W}). It reports the view's kind, 0 when
// the source has none; an evaluated rkDyn amount is never in r.
func (s *State) takeRestrUnit(k *prep, p uint8, x Slot, d *payDesc, r *avail) uint8 {
	v, kind := s.restrTapView(k, p, x, d)
	switch kind {
	case rkWiden:
		if v.wideN != 0 && DebugRestrictedSource != 4 {
			// remove drops the one-colour entry restrSrcAvail added, and
			// only for ColAny: a fixed colour adds none
			r.remove(uint8(v.wideCol), v.wideN)
			return kind
		}
		r.remove(s.srcMaskAt(v, x), v.manaN)
	case rkExt:
		r.remove(srcMask(v), v.manaN)
	default:
		return kind
	}
	for col, n := range v.manaXv {
		if n > 0 {
			r.remove(1<<col, n)
		}
	}
	return kind
}

// spellPartDisjoint is costPartDisjoint for p's cast of card c (object o) at
// cost, whose additional cost c's part taps or sacrifices permanents: the
// restricted sources the spell may spend join the mana as restrSrcAvail counts
// them, each a candidate whose tap takes that mana away (Creeping Peeper's
// enchantment-only {U} paying Fear of Exposure is not also one of the two
// permanents it taps; CR 601.2g-h, gorge planReserving).
func (s *State) spellPartDisjoint(k *prep, p uint8, c CardID, o Obj, part *kop.Cost, cost *kop.ManaCost) bool {
	if part.Flags&kop.CostTapX != 0 && s.plainPlanLeavesTaps(k, p, part, cost) {
		return true
	}
	var av avail
	s.availFor(k, p, &av)
	if k.restrSrc && k.gen != nil && part.Flags&kop.CostTapX != 0 {
		d := payDesc{class: pdSpell, card: c, src: NoSlot, og: s.restrOriginOfObj(o)}
		// an evaluated-amount source (rkDyn, Helga) is no unit of t, as it is
		// none of av: the restricted units t does hold stay reserved
		t, _, _ := s.restrSrcAvail(k, p, d, &av, NoSlot)
		return s.costPartDisjointD(k, p, NoSlot, part, cost, &t, pipMaskOf(cost), &d)
	}
	return s.costPartDisjoint(k, p, NoSlot, part, cost, &av, pipMaskOf(cost))
}

// plainPlanLeavesTaps is gorge planReserving's first step for a spell's tap part:
// the mana plan made with nothing reserved stands when it leaves N candidates
// untapped (planLeavesCostPayable), before any assignment is tried. Nine lands
// pay Guardian of the Great Door's {W}{W} from the Plains and a Restless
// Anchorage and leave seven to tap, however far down the reservation order the
// assignments that keep both {W} sources sit (CR 601.2g-h).
func (s *State) plainPlanLeavesTaps(k *prep, p uint8, part *kop.Cost, cost *kop.ManaCost) bool {
	pk := planner{dry: true}
	s.tapForPK(k, p, cost, 0, NoObj, false, false, &pk)
	if !pk.found {
		return false
	}
	left := 0
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		x := m.first()
		if s.perms[x].flags&pfTapped == 0 && !pk.planUses(x) && s.matchPerm(k, part.Filter, x, p, NoSlot) {
			left++
		}
	}
	return left >= int(part.N)
}

// altAddOneCast reports a plain hand cast a of a card whose additional cost is
// a one-part AlternateAdditionalCost (Dusk Rose Reliquary's sacrifice): gorge's
// cast planner declines it ("shape:optional_cost"), so the potential planner
// prices the printed mana from its census and the prefix search taps a granted
// source the census misses (CR 601.2g-h).
func (s *State) altAddOneCast(k *prep, a Action) bool {
	if a.Kind() != AkCast || a.Mode() != 0 || a.Target() != 0 {
		return false
	}
	o := s.objs[a.Source()]
	cd := &k.t.Cards[o.card]
	return cd.AltAddOne && !cd.HasAltAdd && o.zone == kop.ZoneHand && cd.Cost.X == 0 && cd.Cost.Hyb[0] == 0
}
