package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// An OpMayPlay record with Duration$ UntilYourNextTurn ends at the cleanup
// before its controller's next turn begins: gorge's
// rescheduleNextTurnBoundaries re-derives the boundary as nextTurnFor - 1
// from the live rotation and the pending extra-turn queue every cleanup
// (engine/rules/layers_effect.go), so an extra turn granted the controller
// ends the record at the current turn's cleanup and a skipped turn shortens
// the way. Keeping it while its controller is the active player
// (turn_gen.go's old arm) keeps it exactly one turn too long in both cases.
// The treatment is dropNextTurnOv's and dropNextTurnEmblems': a controller
// in the game ends its records when their turn is the next (nextActive), one
// that has left counts down the counter it froze at the departure. The class
// has its own countdown pool so a cleanup that also drops UntilYourNextTurn
// overrides or emblems decrements each class's counter once.

var coldMayNTQ = newCold[uint8]("mayNTQ", nil)

// prepMayPlayNT lays coldMayNTQ out for a table with an UntilYourNextTurn
// play permission. It is on prep, not genPrep, because the fixed-State
// inventory is pinned (TestNoNewFixedStateFields); a table without such a
// grant has no region.
func prepMayPlayNT(t *kop.Table, p *prep) {
	if tableMayPlayNT(t) {
		coldMayNTQ.set(&p.cold, MaxSeats)
	}
}

// tableMayPlayNT reports whether some program of t grants a play permission
// that lasts until its controller's next turn.
func tableMayPlayNT(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpMayPlay && in.A.K == kop.VConst && in.A.N == kop.DurUntilYourNextTurn {
					return true
				}
			}
		}
	}
	return false
}

// mayNTArm sets controller c's expiry counter for a record registered now:
// the boundary is the turn before c's next turn, that many cleanups from
// this turn's, this turn's the first (ovNTArm's treatment).
// mayNTGrantCtrl marks an owner-held UntilYourNextTurn record whose turn
// word holds the grant's controller seat in its low bits (partitionRecord):
// the record's permission subject is the card's owner (r.ctrl,
// cast_whileexiled.go), but gorge's expiry boundary is the effect
// controller's next turn for every record of the grant
// (layers_effect.go nextTurnFor(ce.Controller)). The bit is clear on every
// other record's turn word: a creation turn number, a raise amount, or
// mayHeld's object|ver packing never reach it.
const mayNTGrantCtrl uint32 = 1 << 31

// ntSeat is the seat whose next turn ends record r's UntilYourNextTurn
// grant.
func (r mayPlay) ntSeat() uint8 {
	if r.turn&mayNTGrantCtrl != 0 {
		return uint8(r.turn)
	}
	return r.ctrl
}

// mayNTArm sets controller c's expiry counter for a record registered now:
// the boundary is the turn before c's next turn, that many cleanups from
// this turn's, this turn's the first (ovNTArm's treatment).
func (s *State) mayNTArm(c uint8) {
	q := coldMayNTQ.of(s)
	if q == nil || c >= MaxSeats {
		return
	}
	n := uint8(0)
	if d := s.turnsTo(c); d > 1 {
		n = d - 1
	}
	q[SeatIx(c)] = n
}

// mayNTEnding is embNTEnding for the controllers (a bit per seat) whose live
// UntilYourNextTurn play-permission records end at this cleanup, given the
// controllers that hold one (have). A controller in the game ends them when
// their turn is the next, else re-derives the counter; one that has left
// ends them when the counter it froze runs out.
func (s *State) mayNTEnding(have uint8) (ends uint8) {
	q := coldMayNTQ.of(s)
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
		case q != nil && q[SeatIx(c)] > 0:
			q[SeatIx(c)]--
		default:
			ends |= 1 << c
		}
	}
	return ends
}

// mayNTSweep returns the controllers (a bit per seat) whose UntilYourNextTurn
// play-permission records end at the cleanup in progress; a table with no
// such grant has no coldMayNTQ region and sweeps nothing.
func (s *State) mayNTSweep() uint8 {
	if coldMayNTQ.count(s.cold.layout) == 0 {
		return 0
	}
	var have uint8
	for i := 0; i < int(s.g.nmay); i++ {
		if r := *s.mayAt(i); r.dur() == kop.DurUntilYourNextTurn {
			have |= 1 << r.ntSeat()
		}
	}
	if have == 0 {
		return 0
	}
	return s.mayNTEnding(have)
}
