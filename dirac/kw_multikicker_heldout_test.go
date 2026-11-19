package dirac

import "testing"

// TestKickedPotentialOnlyHeldOut: a kicker card whose plain cast the real
// census cannot pay is offered its kicked cast through the potential arm
// (offerKickedPotentialOnly); after a second no-progress undo gorge holds the
// card's whole cast out (castHeldOut, CR 733.2), so that offer is withheld.
func TestKickedPotentialOnlyHeldOut(t *testing.T) {
	sc, caligo := openCensusScen(t, 1, 1, 1, true)
	if !offeredKickedCast(sc, caligo) {
		t.Fatal("precondition: the kicked cast is not offered before the hold-out")
	}
	s := sc.s
	s.heldObj, s.heldN = caligo, 2
	s.heldTurn, s.heldStep, s.heldTouch = uint16(s.turn), s.step, s.touchN
	if !s.castHeldOut(caligo) {
		t.Fatal("precondition: the card is not held out")
	}
	if offeredKickedCast(sc, caligo) || castModeOffered(sc, caligo, 0) {
		t.Fatal("a held-out card's cast is still offered")
	}
}
