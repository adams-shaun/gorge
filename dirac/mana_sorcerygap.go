package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// sorceryGapSearched reports whether the sorcery-speed activation a (of the
// permanent at sl) is priced by gorge's prefix search although the printed
// census pays it. Under Taps-deferred sources (Manabarbs, Rikala) gorge's
// planner census lists none of the board's lands, a filter land's {C} tap
// included, so the activation's census verdict is insufficient and the play
// takes a prefix search (potentialCensusFallback). A sorcery-speed play is
// not offered while a trigger the line's tap set off is on the stack
// (scriptSearch.waitsForStack), so the depth-first search taps another gap
// before it passes, and with every land a gap the 160-clone budget is spent
// before the line resolves its first trigger: the equip is withheld
// (unsupported/census_incomplete). dirac keeps a filter source's census unit
// through the deferral (scriptTapsDefer), so the printed census pays the
// activation and the filter script, the mirror of that search, must confirm
// it (filtScript's node count and its budget, actScriptPays). A census that
// pays without the filters' units needs no search (below).
func (s *State) sorceryGapSearched(k *prep, sl Slot, ab int, a *kop.Ability, cost *kop.ManaCost, av *avail) bool {
	// actScriptPays's own gates: the script is read only where it can run
	if k.gen == nil || !k.gen.anyFilt || a.Cost.Flags&kop.CostSorcery == 0 ||
		ab >= 255 || a.Kind == kop.AbMana || !plainPricedCost(a) ||
		s.g.availX&availNoActScript != 0 || s.pool != nil && s.pool.inFilt != 0 {
		return false
	}
	p := s.perms[sl].ctrl
	f := s.scriptFilt(k, p)
	if !f.any() || s.filtNetNegative(k, f) != 0 || !s.scriptTapsDefer(k, p).any() {
		return false
	}
	// A census that pays without the filters' units is gorge's census too: a
	// non-land source's tap (Silver Myr's {U} for an Equip {1}) is no
	// Taps-deferred gap, so gorge prices the play from it and runs no search.
	// Only a play the filters' own {C} units alone make payable is the
	// search's (the Equip beside no other source).
	pay := *av
	for b := f; b.any(); b = b.rest() {
		x := b.first()
		s.removeSrc(k, &pay, x, s.manaPCGen(k, x))
	}
	return !pay.affords(cost, pipMaskOf(cost), 0)
}
