package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// plannedAfterLeaving: a flashback cast of o whose planned payment still pays
// once o has left the graveyard. A dynamic-amount mana source reads the board
// (Loot, the Nexus adds one mana per different power, Tarmogoyf's among them:
// a card type fewer in the graveyard is a point of power fewer), and gorge's
// cast planner prices the payment with the spell on the stack, so a flashback
// that only the source's graveyard-time amount pays is no planned cast: gorge
// reaches it as a potential play, tapping the sources first (CR 117.1d, 605.3a),
// at their amounts before the spell leaves.
func (s *State) plannedAfterLeaving(k *prep, p uint8, o Obj, cost *kop.ManaCost) bool {
	if k.gen == nil || s.objs[o].zone != kop.ZoneGraveyard {
		return true
	}
	dynamic := false
	for m := s.ctrlM[SeatIx(p)]; m.any() && !dynamic; m = m.rest() {
		dynamic = k.gen.flags[s.cardAt(m.first())]&gfManaVar != 0
	}
	if !dynamic {
		return true
	}
	var av avail
	s.availFor(k, p, &av)
	if !av.affords(cost, pipMaskOf(cost), 0) {
		return true // not paid before the move either: this is not the gate that differs
	}
	t := s.fork()
	defer s.unfork()
	t.detach(o)
	t.objs[o].zone = kop.ZoneStack
	var after avail
	t.availFor(k, p, &after)
	return after.affords(cost, pipMaskOf(cost), 0)
}
