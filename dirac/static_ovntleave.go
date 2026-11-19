package dirac

// An UntilYourNextTurn override record whose controller leaves the game
// (CR 800.4a) still ends: gorge's rescheduleNextTurnBoundaries re-derives the
// boundary every cleanup while the controller is in the rotation and keeps the
// one it last had once nextTurnFor returns 0 for them, so the effect ends at
// that frozen boundary (turnsTo, repl_dmgmark.go, counts the same rotation).
// dropNextTurnOv ends a record when its controller's turn is the next, which
// an eliminated seat never is, so each controller carries an expiry counter in
// a cold region: the cleanups left after the one in progress before the
// cleanup the records end at, as dmkQ does for the damage marks. A controller
// still in the game re-derives it at every cleanup; one that has left counts
// it down.

// DebugOvNTKeepLost is the mutation switch (dirac/oracle
// TestVentureOvNTMutation): a lost controller's records never end.
var DebugOvNTKeepLost bool

var coldOvNTQ = newCold[uint8]("ovNTQ", func(g *genPrep) int {
	if g.anyNTOv {
		return MaxSeats
	}
	return 0
})

// ovNTArm sets controller c's expiry counter for a record registered now: the
// boundary is the turn before c's next turn, that many cleanups from this
// turn's, this turn's the first (gorge UntilTurn = nextTurnFor - 1).
func (s *State) ovNTArm(c uint8) {
	q := coldOvNTQ.of(s)
	if q == nil || c >= MaxSeats {
		return
	}
	n := uint8(0)
	if d := s.turnsTo(c); d > 1 {
		n = d - 1
	}
	q[SeatIx(c)] = n
}

// ovNTEnding is the controllers (a bit per seat) whose live next-turn records
// end at this cleanup, given the controllers that hold records (have). A
// controller in the game ends them when their turn is the next, else
// re-derives the counter; one that has left ends them when the counter it
// froze runs out.
func (s *State) ovNTEnding(have uint8) (ends uint8) {
	q := coldOvNTQ.of(s)
	nt := s.nextActive()
	for c := uint8(0); c < MaxSeats; c++ {
		if have&(1<<c) == 0 {
			continue
		}
		switch {
		case !s.out(c):
			if c == nt {
				ends |= 1 << c
			} else if d := s.turnsTo(c); d > 1 && q != nil {
				q[SeatIx(c)] = d - 2
			}
		case DebugOvNTKeepLost:
		case q != nil && q[SeatIx(c)] > 0:
			q[SeatIx(c)]--
		default:
			ends |= 1 << c
		}
	}
	return ends
}
