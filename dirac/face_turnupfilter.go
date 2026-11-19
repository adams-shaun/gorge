package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// turnUpFilterPays: gorge's turn-up witness is the payment planner's, whose
// census prices a first-and-only filter mana ability (Gravestone Strider's
// "{1}: Add one mana of any color", Prophetic Prism) as a paid gap beside
// the plain sources (potential_plan.go), so a keyword turn-up whose cost the
// sources pay only through that filter is a potential play (CR 605.3a,
// 106.1). A second-ability filter beside a free {T} ability (Talon Gates of
// Madara's "{1}, {T}: Add one mana of any color") is priced the same way, so
// the gate is any filter, not only a first-and-only one. availFor leaves a filter out of its units, so the price is the
// filter search a cast's gap prefix runs (filtScript), asked for the
// turn-up action: its prefix funds the filter's fee, activates it, and the
// pool then pays the cost (turnUpOffered).
//
//go:noinline
func (s *State) turnUpFilterPays(k *prep, p uint8, sl Slot, mode uint8) bool {
	if k.gen == nil || !k.gen.anyFilt || !s.scriptFilt(k, p).any() {
		return false
	}
	return s.filtScript(k, MakeAction(AkTurnFaceUp, TCode(s.perms[sl].obj), 0, mode), p, false)
}

// turnUpFilterOnly: no plain route pays the turn-up's cost, only the filter
// search does, so the planner has no plan to tap (tapForTurnUp would find
// none).
//
//go:noinline
func (s *State) turnUpFilterOnly(k *prep, p uint8, cost *kop.ManaCost, sp kop.SpecialMask, sl Slot, mode uint8) bool {
	return !s.sourcesPay(k, p, cost) && !s.turnUpRestrPays(k, p, cost, sp, sl) && !s.turnUpRiderPays(k, p, cost) &&
		!s.turnUpPlannerPays(k, p, cost) && s.turnUpFilterPays(k, p, sl, mode)
}

// playTurnUpFilter plays the filter search's prefix for the turn-up of o: the
// fee's funding taps and the filter's activation, leaving the mana in the pool.
//
//go:noinline
func (s *State) playTurnUpFilter(k *prep, p uint8, o Obj, mode uint8) {
	s.filtScript(k, MakeAction(AkTurnFaceUp, TCode(o), 0, mode), p, true)
}
