package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Leaving the game (CR 800.4a). At two seats a loss ends the game, so none of
// this runs there: every entry point is behind MaxSeats > 2 and folds away in
// the default build.

// checkOverN is checkOver at more than two seats: the game goes on while two
// or more seats remain (CR 800.4a); the last one wins (CR 104.2a), and with
// none left it is a draw (CR 104.4a). gorge checkGameOver.
//
//go:noinline
func (s *State) checkOverN() {
	n := s.aliveCount()
	if n > 1 {
		return
	}
	s.over = true
	s.ctl = ctlOver
	s.win = -1
	if n == 1 {
		for p := uint8(0); p < s.nSeats(); p++ {
			if !s.out(p) {
				s.win = int8(p)
			}
		}
	}
}

// sweepDeparted is gorge's sweepDepartedPlayers: every seat that has left
// leaves its objects behind (leaveGame). It reports whether anything moved,
// so the state-based loop runs another pass. Idempotent: a swept seat owns
// nothing in any zone any more, so the next call moves nothing.
//
//go:noinline
func (s *State) sweepDeparted(k *prep) bool {
	moved := false
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) && s.leaveGame(k, p) {
			moved = true
		}
	}
	return moved
}

// leaveGame applies CR 800.4a for departed seat p in object order, as gorge's
// ceaseDepartedObjects: every object p owns ceases to exist wherever it is,
// every spell or ability p controls leaves the stack, an attacker attacking p
// (or p's planeswalker) is removed from combat, and a permanent p still
// controls that another seat owns is exiled. p's pending triggers are
// dropped (gorge dropDepartedTriggers). Ceased objects keep their ids with
// no zone (kop.ZoneNone, which the oracle projects as ZCeased).
func (s *State) leaveGame(k *prep, p uint8) bool {
	moved := false
	// the attackers attacking p, judged before any of p's walkers cease
	var atk slotMask
	if s.nAttackers > 0 {
		for m := s.occ; m.any(); m = m.rest() {
			if a := m.first(); s.perms[a].flags&pfAttacking != 0 && s.atkSeat(k, a) == p {
				atk.set(a)
			}
		}
	}
	// the stack entries p controls: abilities, and spells p controls but
	// another seat owns (a spell p owns ceases with the walk below)
	for i := 0; i < int(s.sp); {
		e := &s.stack[i]
		if e.ctrl != p {
			i++
			continue
		}
		o, spell := e.obj, e.kind == seSpell
		copy(s.stack[i:s.sp-1], s.stack[i+1:s.sp])
		s.sp--
		if spell {
			s.touch(o)
			s.objs[o].zone = kop.ZoneNone
		}
		moved = true
	}
	for i := 0; i < int(s.nobj); i++ {
		o := Obj(i)
		ob := &s.objs[o]
		if ob.zone == kop.ZoneNone {
			continue
		}
		if ob.owner() != p {
			if ob.zone == kop.ZoneBattlefield && atk.has(ob.slot) {
				s.leaveCombat(ob.slot) // gorge EndCombatReset (CR 506.4)
				moved = true
			}
			continue
		}
		moved = true
		switch ob.zone {
		case kop.ZoneBattlefield:
			s.leave(k, ob.slot, kop.ZoneNone, false)
		case kop.ZoneStack:
			if j := s.stackIndexOf(o); j >= 0 {
				copy(s.stack[j:s.sp-1], s.stack[j+1:s.sp])
				s.sp--
			}
			s.touch(o)
			ob.zone = kop.ZoneNone
		case kop.ZonePhased, kop.ZonePhasedInd:
			// a phased-out permanent keeps its slot (ext.go): not modelled
			s.stall(StallLowering)
		default:
			s.detach(o)
		}
	}
	// CR 800.4a's last step: what p still controls is exiled to its owner
	for m := s.ctrlM[SeatIx(p)]; m.any(); m = m.rest() {
		s.leave(k, m.first(), kop.ZoneExile, false)
		moved = true
	}
	// p's pending triggers go with p (gorge dropDepartedTriggers)
	if s.orderLeft == 0 && s.orderPlace == 0 {
		n := uint8(0)
		for i := uint8(0); i < s.npend; i++ {
			if s.pend[i].ctrl != p {
				s.pend[n] = s.pend[i]
				n++
			}
		}
		if n != s.npend {
			s.npend = n
			moved = true
		}
	}
	return moved
}
