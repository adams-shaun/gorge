package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The census total an {X} witness divides by X (rules potentialModeCastPlan:
// hi = paymentPlanCensusTotal / base.X) counts every census source's largest
// alternative, whether or not the payment may spend it: Heartwood Crafter's
// "{T}: Add {C}. This mana can't be spent to cast spells from your hand" is a
// unit of the census beside five lands, so a hand cast of {X}{X}{R} under a
// reduction of five is searched up to 6/2 = 3, not the 5/2 = 2 of the mana it
// can spend. The reduced cost is the one the search prices (the binary search
// asks the planner at each X), so a bound that is one mana short stops the
// witness a value early and taps one source fewer than gorge floats.

// restrWithheldMana is the mana of p's restricted sources of their own (rkExt,
// rkDyn: a unit with no census ability to widen) that payment d may not spend
// but gorge's census still holds: restrSrcAvail counts what d may spend, this
// counts the rest. A widened source's larger alternative is not counted.
func (s *State) restrWithheldMana(k *prep, p uint8, d payDesc) int {
	g := k.gen
	deferSkip := s.deferLive(k) && s.g.availX&(availWalk|availHyp) == 0
	n := 0
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		c := s.cardAt(sl)
		if g.flags[c]&gfRestrSrc == 0 || deferSkip && s.planDeferred(k, p, sl) || !s.restrUsable(k, sl) {
			continue
		}
		if v, _, _ := s.restrView(k, p, sl, s.manaPCGen(k, sl), &d); v != nil {
			continue // a unit of the payment's own (restrSrcAvail)
		}
		best := 0
		for i := range g.rs.recs {
			rec := &g.rs.recs[i]
			if rec.card != c || rec.kind == rkWiden {
				continue
			}
			amt := int(g.rs.views[rec.view].manaN)
			if rec.kind == rkDyn {
				amt = min(s.dynManaN(k, &k.t.Cards[c].Abilities[rec.ab], sl), maxDynMana)
			}
			best = max(best, amt)
		}
		n += best
	}
	return n
}

// restrWitnessXPlus is restrWitnessX with withheld more mana in the census
// total X divides into.
func restrWitnessXPlus(x int, t avail, multi []uint8, cost *kop.ManaCost, withheld int) int {
	tot := t.total + withheld
	for _, n := range multi {
		tot += int(n)
	}
	return min(max(x, t.maxXMulti(cost, pipMaskOf(cost), multi)), tot/int(cost.X))
}
