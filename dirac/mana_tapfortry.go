package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// tapForTry is tapForRider for a cost the caller may find unpayable: it taps
// the plan and reports true, or taps nothing and reports false when the
// planner has none (tapForRider panics there, a kernel defect for a cost the
// offer priced). Only a cost the offer could not price yet, as a per-target
// raise charged after the targets (cost_raise.go), uses it.
func (s *State) tapForTry(k *prep, p uint8, cost *kop.ManaCost, x int, exclude Obj, anyColor bool) bool {
	var pk planner
	pk.try = true
	s.tapForPK(k, p, cost, x, exclude, false, anyColor, &pk)
	return pk.needT == 0 || pk.found
}

// tapForSpellPlain taps the plain cast's sources for cost and reports true when
// it reversed the cast instead. A card with "costs {1} more for each target
// beyond the first" (Fireball) has the targets chosen after the X was announced
// raise the price past what the sources make: CR 601.2c lets the player choose
// any number of targets and 601.2h finds the cost unpayable afterwards, so the
// cast is reversed (CR 733.1) where the plan would find none. Every other cast
// keeps tapForRider and its panic, for a cost the offer priced.
func (s *State) tapForSpellPlain(k *prep, e *stackEntry, cost *kop.ManaCost, anyColor bool, saved *[len(build{}.picks)]uint8) bool {
	if s.raisePerTargetCharge(e.card, e.ctrl) == 0 {
		s.tapForRider(k, e.ctrl, cost, int(e.x), e.obj, anyColor)
		return false
	}
	if s.tapForTry(k, e.ctrl, cost, int(e.x), e.obj, anyColor) {
		return false
	}
	s.releaseCostTaps(k, saved)
	s.reverseCast(k)
	return true
}
