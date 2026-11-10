package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// An all-at-once filter (kop.FilterEachManaAb): Skycloud Expanse's "{1}, {T}:
// Add {W}{U}", the card's first and only mana ability. gorge's planner census
// skips its paid cost, as Prophetic Prism's (mana_filterfirst.go); its relaxed
// alternative is one fixed production of every colour it makes (rules
// paymentPlanRelaxedAlternatives, !CountsAny: no multiset), the fee charged as
// generic and the {T} making it once only. It differs from Prism's in one way:
// activating it poses no colour ask (a fixed production; rules activateManaFor
// resolves it directly), so the search tries it once, no ask node, and every
// activation (search, window, dry run) adds the whole vector whatever colour
// the caller named.

// filtEachVec is the mana an all-at-once filter a adds, by colour.
func filtEachVec(a *kop.Ability) (v [kop.NumManaKinds]uint8) {
	n := max(a.ManaN, 1)
	for c := 0; c < 5; c++ {
		if a.ManaColors&(1<<c) != 0 {
			v[c] = n
		}
	}
	return v
}

// addFiltUnits adds filter a's production to t: units of its colour mask (one
// any-colour unit, a Combo's, or a dynamic amount of one colour), or each colour
// of an all-at-once one.
func addFiltUnits(t *avail, a *kop.Ability, units uint8) {
	if a.ManaEach {
		for c, n := range filtEachVec(a) {
			if n != 0 {
				t.addUnits(1<<c, n)
			}
		}
		return
	}
	t.addUnits(uint8(a.ManaColors), units)
}

// filtEachDo adds an all-at-once filter's vector at sl to p's pool, each colour
// tagged as the tap's (an artifact's mana is typed: mana_artifact.go), with the
// TapsForMana extra its {T} brings (mana_tapextra.go filtExtra).
func (s *State) filtEachDo(k *prep, p uint8, sl Slot) {
	for c, n := range filtEachVec(s.filtAb(k, sl)) {
		if n != 0 {
			s.poolAdd(p, c, int(n))
			s.artTag(k, p, sl, c)
		}
	}
	xv := s.filtExtra(k, sl)
	s.addXv(p, &xv)
}
