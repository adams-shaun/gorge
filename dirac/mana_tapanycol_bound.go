package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// potentialTapAnyColExtra folds into the potential over-bound pool the
// any-colour StatTapExtra statics (StTapAnyCol: Fertile Ground's one mana of
// any color, Dawn's Reflection's two) a tap of sl brings. gorge's planner
// never folds such a body into the source's production (the source stays
// deferred), but its potential bound reads it: a literal Amount$ of Produced$
// Any or Combo Any is that many mana of each of the five colours, never
// colourless (rules payment_plan_interference.go addPotentialOpenTapMana,
// potential.go potentialTapExtra; CR 605.3b). Without it a Forest under
// Fertile Ground beside two more lands bounds at three mana, and a {3}{G}
// cast the prefix search pays falls out of the offer. The per-tap census
// keeps leaving it out (the trigger joins no census unit), as gorge's does.
func (s *State) potentialTapAnyColExtra(k *prep, sl Slot, fold func(Slot, uint8, uint8)) {
	if k.gen == nil || !k.gen.anyTapExtra || DebugTapAnyColOff || s.pool != nil && s.pool.census {
		return
	}
	ctl := s.perms[sl].ctrl
	s.eachStatic(k, func(st statSrc) bool {
		a := &st.a.Static
		if st.a.Kind != kop.AbStatic || a.Kind != kop.StatTapExtra || st.sl == NoSlot ||
			a.StFlags&kop.StTapAnyCol == 0 || a.Cond2 != 0 ||
			a.StFlags&(kop.StTapReflect|kop.StTapOnC) != 0 ||
			st.ctrl != ctl && a.StFlags&kop.StTapAnyPlayer == 0 {
			return true
		}
		if !s.matchPermGen(k, a.Filter, sl, st.ctrl, st.sl, layerFull) {
			return true
		}
		for c := 0; c < 5; c++ {
			fold(sl, 1<<c, uint8(a.B))
		}
		return true
	})
}

// tapsDeferAnyColUnits adds the any-colour StatTapExtra bound to a
// Taps-deferred gap's census unit: gorge's relaxed alternative of the gap
// carries potentialTapExtra on every production (potential_plan.go
// paymentPlanRelaxedAlternatives' add), and that extra now holds the open
// trigger's mana in each of the five colours (potentialTapAnyColExtra), so the
// relaxed Forest under Fertile Ground over-produces {G} plus one of each
// colour and the prefix search runs for a {3}{G} play.
func (s *State) tapsDeferAnyColUnits(k *prep, sl Slot, av *avail) {
	s.potentialTapAnyColExtra(k, sl, func(_ Slot, m, n uint8) { av.addUnits(m, n) })
}
