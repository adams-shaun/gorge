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
func (s *State) sweepDeparted(k *prep, pre uint8) bool {
	moved := false
	if s.g.nfx != 0 {
		s.expireControlPerLoss(k, pre)
	}
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) && s.leaveGame(k, p) {
			moved = true
		}
	}
	return moved
}

// outMask is the set of seats out of the game, one bit a seat.
func (s *State) outMask() uint8 {
	var m uint8
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			m |= 1 << p
		}
	}
	return m
}

// expireControlPerLoss ends the effects giving each departed seat control.
// A seat that left before the state-based pass began (pre: an effect's loss)
// is passed one seat at a time in seat order, as gorge's expireControl runs at
// every PlayerLost emit (a stolen permanent goes back before the next player's
// loss is emitted, so simultaneous losses return their permanents in loss
// order, not in the effects' timestamp order). The seats the pass itself
// marked lost end their effects together in one pass after its batch applies
// (gorge's sbaLossHeld and releaseLossHold, CR 704.3, 800.4a), so their
// permanents return in the effects' order. A later seat reads as still in
// the game for its predecessor's pass.
func (s *State) expireControlPerLoss(k *prep, pre uint8) {
	var lost [MaxSeats]bool
	n := 0
	for p := uint8(0); p < s.nSeats(); p++ {
		lost[p] = s.out(p)
		if lost[p] {
			n++
		}
	}
	if n < 2 {
		return // one loss: leaveGame's own pass is the whole of it
	}
	for p := uint8(0); p < s.nSeats(); p++ {
		if !lost[p] || pre&(1<<p) == 0 {
			continue
		}
		for q := uint8(0); q < s.nSeats(); q++ {
			if lost[q] && (q > p || pre&(1<<q) == 0) {
				s.pl[SeatIx(q)].lost = false
			}
		}
		s.ctrlExpire(k, false)
		for q := uint8(0); q < s.nSeats(); q++ {
			s.pl[SeatIx(q)].lost = lost[q]
		}
	}
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
	if s.g.nfx != 0 {
		// every effect giving p control ends first, as gorge's expireControl
		// runs on the PlayerLost emit (ctrlSeatLeft): a stolen permanent goes
		// back to its controller before the effect, not to exile
		s.ctrlExpire(k, false)
	}
	// the attackers attacking p, judged before any of p's walkers cease
	var atk slotMask
	if s.nAttackers > 0 {
		for m := s.occ; m.any(); m = m.rest() {
			if a := m.first(); s.perms[a].flags&pfAttacking != 0 && s.atkSeatLeft(k, a) == p {
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

// ctrlSeatLeft reports whether control record r gives control to, or was
// made by, a seat that has left the game (gorge grantEnded's CR 800.4a
// check, which applies to a static's record too).
func (s *State) ctrlSeatLeft(r *fxRec) bool {
	return s.out(r.cTo()) || s.out(r.cYou())
}

// atkSeatLeft is the seat attacker a is attacking as a player leaves the
// game: atkSeat, or, once the planeswalker it attacks has left the
// battlefield, that planeswalker's owner. gorge keeps the declared defending
// seat on the attacker (Object.Attacking) after its planeswalker is gone
// (CR 506.4), so the attacker leaves combat with that seat's player.
func (s *State) atkSeatLeft(k *prep, a Slot) uint8 {
	q := s.atkSeat(k, a)
	if t := s.perms[a].atk; q == noSeat && !isPlayerOrNone(t) && s.objs[t].zone != kop.ZoneBattlefield {
		q = s.objs[t].owner()
	}
	return q
}
