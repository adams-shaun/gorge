package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// planHoldsOnStack: gorge offers a planned cast only when every source its
// plan taps still makes the same mana once the card has moved to the stack
// (rules castprobe.go paymentPlanHoldsOnStack). The plan executes in the CR
// 601.2g window, which opens after CR 601.2a has moved the card from its zone,
// so a dynamic-amount or gated source reading that zone can change: Selvala,
// Eager Trailblazer's "one mana for each different power" beside Stingerback
// Terror (-1/-1 for each card in hand) makes one more with the spell still in
// hand. A plan whose step changes is withheld, and gorge reaches the cast as a
// potential play, tapping the sources first (CR 605.3a) at their amounts with
// the card still in hand.
//
// The plan is the one the cast would tap (exclude o, the HandReserve card),
// taken on a fork; each source it taps is then compared, on a second fork with
// o on the stack, by its planner view (manaPCGen: the production variant and
// gate) and whether it can still tap. Only a player controlling a
// dynamic-amount source (gfManaVar) can see a step change, and only a card
// leaving the hand, exile or the graveyard (gorge offerAsSpellOnStack) moves.
func (s *State) planHoldsOnStack(k *prep, p uint8, o Obj, cost *kop.ManaCost) bool {
	if k.gen == nil {
		return true
	}
	switch s.objs[o].zone {
	case kop.ZoneHand, kop.ZoneExile, kop.ZoneGraveyard:
	default:
		return true
	}
	seat := SeatIx(p)
	dynamic := false
	for m := s.ctrlM[seat]; m.any() && !dynamic; m = m.rest() {
		dynamic = k.gen.flags[s.cardAt(m.first())]&gfManaVar != 0
	}
	if !dynamic {
		return true
	}
	var steps slotMask
	t := s.fork()
	var pk planner
	pk.try = true // an unpayable cost taps nothing and is no plan
	c := *cost
	t.tapForPK(k, p, &c, 0, o, false, false, &pk)
	for m := s.ctrlM[seat]; m.any(); m = m.rest() {
		sl := m.first()
		if t.perms[sl].obj != s.perms[sl].obj || t.perms[sl].flags&pfTapped != 0 && s.perms[sl].flags&pfTapped == 0 {
			steps.set(sl)
		}
	}
	s.unfork()
	if !steps.any() {
		return true
	}
	u := s.fork()
	defer s.unfork()
	u.detach(o)
	u.objs[o].zone = kop.ZoneStack
	for m := steps; m.any(); m = m.rest() {
		sl := m.first()
		before, after := s.manaPCGen(k, sl), u.manaPCGen(k, sl)
		if before != after || s.canTapForManaPC(k, sl, before) != u.canTapForManaPC(k, sl, after) {
			return false
		}
	}
	return true
}
