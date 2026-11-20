package dirac

// Effect-delivered Continuous statics.

// coldEmbNTQ is one countdown per seat for UntilYourNextTurn emblems whose
// controller has left the game, the emblem counterpart of coldOvNTQ
// (static_ovntleave.go). gorge's rescheduleNextTurnBoundaries re-derives an
// UntilYourNextTurn boundary every cleanup while the controller is in the
// rotation and keeps the one it last had once nextTurnFor returns 0 for them,
// so a departed controller's effect ends at that frozen boundary, not at the
// loss. A controller still in the game re-derives it at every cleanup; one
// that has left counts it down.
var coldEmbNTQ = newCold[uint8]("embNTQ", func(g *genPrep) int {
	if g.anyNTEmb {
		return MaxSeats
	}
	return 0
})

// embNTArm sets controller c's expiry counter for an UntilYourNextTurn emblem
// created now: the boundary is the cleanup before c's next turn, that many
// cleanups from this turn's, this turn's the first (gorge UntilTurn =
// nextTurnFor - 1). The count is ovNTArm's: every UntilYourNextTurn effect of
// one controller still alive shares that next boundary, so one count per seat
// is enough.
func (s *State) embNTArm(c uint8) {
	q := coldEmbNTQ.of(s)
	if q == nil || c >= MaxSeats {
		return
	}
	n := uint8(0)
	if d := s.turnsTo(c); d > 1 {
		n = d - 1
	}
	q[SeatIx(c)] = n
}

// embNTEnding is ovNTEnding for the controllers (a bit per seat) whose live
// UntilYourNextTurn emblems end at this cleanup, given the controllers that
// hold one (have). A controller in the game ends them when their turn is the
// next, else re-derives the counter; one that has left ends them when the
// counter it froze runs out.
func (s *State) embNTEnding(have uint8) (ends uint8) {
	q := coldEmbNTQ.of(s)
	nt := s.nextActive()
	for c := uint8(0); c < s.nSeats(); c++ {
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
		case q != nil && q[SeatIx(c)] > 0:
			q[SeatIx(c)]--
		default:
			ends |= 1 << c
		}
	}
	return ends
}

// dropNextTurnEmblems removes the emblems whose Duration$ UntilYourNextTurn
// ends at the cleanup before their controller's next turn begins. gorge
// computes the boundary as nextTurnFor - 1 (layers_effect.go), so the drop
// happens when the controller is the next active player -- the same test
// dropNextTurnOv uses for OpOverride records. A controller that has left the
// game never is nextActive, so its counter (embNTEnding) ends the emblem at
// the boundary gorge froze at the departure. Emblems are not objects, so
// there is no slot to touch; the per-player arrays are compacted in place.
//
//go:noinline
func (s *State) dropNextTurnEmblems(k *prep) {
	var have uint8
	for p := uint8(0); p < s.nSeats(); p++ {
		for i := 0; i < int(s.g.nemb[p]); i++ {
			if k.t.Emblems[s.embIdx(p, i)-1].UntilNextTurn {
				have |= 1 << p
				break
			}
		}
	}
	if have == 0 {
		return
	}
	ends := s.embNTEnding(have)
	for p := uint8(0); p < s.nSeats(); p++ {
		if ends&(1<<p) == 0 {
			continue
		}
		var drop uint8
		for i := 0; i < int(s.g.nemb[p]); i++ {
			if k.t.Emblems[s.embIdx(p, i)-1].UntilNextTurn {
				drop |= 1 << uint(i)
			}
		}
		if drop != 0 {
			s.embCompact(p, drop)
		}
	}
}
