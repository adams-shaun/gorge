package dirac

import "testing"

// TestKickedPotentialOnlyHeldOut: offerKickedPotentialOnly is the hand walk's
// kicked offer for a card whose plain cast the real census cannot pay. After a
// second no-progress undo gorge holds the card's whole cast out (castHeldOut,
// CR 733.2), so the helper must offer nothing then. It is called directly: the
// open-census pose's own gate pays the plain cast, so priorityOptions' early
// call (the plain-unaffordable branch) is not what reaches it there.
func TestKickedPotentialOnlyHeldOut(t *testing.T) {
	sc, caligo := openCensusScen(t, 1, 1, 1, true)
	s, k := sc.s, sc.k
	c := s.objs[caligo].card
	offers := func() bool {
		var av avail
		s.availFor(k, 0, &av)
		base := k.cards[c].cost
		var d Decision
		s.offerKickedPotentialOnly(k, &d, 0, c, caligo, base, &base, &base, &av, &av)
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkCast && Obj(a.Source()) == caligo && a.Mode() == 1 {
				return true
			}
		}
		return false
	}
	if !offers() {
		t.Fatal("precondition: the kicked cast is not offered before the hold-out")
	}
	s.heldObj, s.heldN = caligo, 2
	s.heldTurn, s.heldStep, s.heldTouch = uint16(s.turn), s.step, s.touchN
	if !s.castHeldOut(caligo) {
		t.Fatal("precondition: the card is not held out")
	}
	if offers() {
		t.Fatal("a held-out card's kicked cast is still offered by the potential-only arm")
	}
}
