package dirac

import "testing"

// TestTemptOptInSeatOrder: the repeated player's yes / no is posed yes first,
// as gorge's repeatEachElectionDecision lists them, whichever order the
// options were added in (the canonical sort), and the election is answered by
// the loop's seat, not the controller (act_optinseat.go).
func TestTemptOptInSeatOrder(t *testing.T) {
	s := new(State)
	d := &Decision{Sub: ChGenOptInSeat}
	s.optInSeatOptions(nil, d)
	if d.Sub != ChGenOptInSeat || d.N != 2 {
		t.Fatalf("sub %d with %d options, want ChGenOptInSeat with yes and no", d.Sub, d.N)
	}
	// reversed on purpose: the sort restores gorge's order
	d.Options[0], d.Options[1] = d.Options[1], d.Options[0]
	s.sortCanonical(d)
	if d.Options[0].Mode() != 1 || d.Options[1].Mode() != 0 {
		t.Fatalf("options sort to modes %d, %d, want yes (1) before no (0)", d.Options[0].Mode(), d.Options[1].Mode())
	}
	if s.canonKey(d, d.Options[0]) >= s.canonKey(d, d.Options[1]) {
		t.Fatal("yes does not key before no")
	}
}

// TestTemptOptInSeatAsksTheLoopSeat: the ask's player is the repeated player.
func TestTemptOptInSeatAsksTheLoopSeat(t *testing.T) {
	s := new(State)
	s.g.repEnd, s.g.eachPl = 9, 1
	s.res.e.ctrl = 0
	if ok, done := s.optInSeatExt(nil, nil); ok || !done {
		t.Fatalf("optInSeatExt = %v,%v, want an ask (false,true)", ok, done)
	}
	if s.res.ask != raGenOptInSeat || s.res.askP != 1 {
		t.Fatalf("ask %d for seat %d, want raGenOptInSeat for the loop's seat 1, not the controller", s.res.ask, s.res.askP)
	}
	s.res.ask = raNone
	s.g.repEnd = 0
	if ok, done := s.optInSeatExt(nil, nil); !ok || !done || s.res.ask != raNone {
		t.Fatal("outside a loop the election was posed")
	}
}
