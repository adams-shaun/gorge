package dirac

// "If that player attacked you during their last turn": a combat DamageDone
// line with kop.Tf5DmgAttackedYouLastTurn (O-Kagachi, Vengeful Kami's
// ValidTarget$ Player.attackedYouTheirLastTurn) fires only when the damaged
// player declared an attack on the listener's controller during the damaged
// player's most recent completed turn. gorge folds the same record from its
// DeclareAttackers events (state.Player.AttackedSeatsThisTurn and
// AttackedSeatsLastTurn): an attack whose defending seat is that player,
// whether at the player, a planeswalker they control or a battle they
// protect, in any combat of the turn.
//
// The record is a cold array of one byte per seat, kept only when
// genPrep.anyAtkLast. Bits 4-7 are the defending seats the seat attacked in
// the turn in progress (only the active seat attacks), bits 0-3 those of its
// most recent completed turn. finishAttacks ORs each declaration's defenders
// in (noteAtkSeats); beginTurn shifts the outgoing active seat's byte down
// (atkLastTurnEnd), so a turn in progress never shows and every other seat
// keeps the turn it last took.

// atkLastCold is the per-seat record, or nil when the table keeps none.
func (s *State) atkLastCold() []uint8 { return coldAtkLast.of(s) }

var coldAtkLast = newCold[uint8]("atkLast", func(g *genPrep) int {
	if g.anyAtkLast {
		return MaxSeats
	}
	return 0
})

// noteAtkSeats records the defending seat of each of the active player's
// attacking creatures m in the active seat's this-turn bits.
func (s *State) noteAtkSeats(k *prep, m slotMask) {
	c := s.atkLastCold()
	if c == nil {
		return
	}
	for ; m.any(); m = m.rest() {
		a := m.first()
		if s.perms[a].flags&pfAttacking == 0 {
			continue
		}
		if d := s.atkSeat(k, a); d < MaxSeats {
			c[SeatIx(s.active)] |= 1 << (4 + d)
		}
	}
}

// atkLastTurnEnd ends the active seat's turn in the record: its this-turn
// defenders become its last turn's.
func (s *State) atkLastTurnEnd() {
	if c := s.atkLastCold(); c != nil && s.active < MaxSeats {
		c[SeatIx(s.active)] >>= 4
	}
}

// attackedYouLastTurn reports whether seat q attacked seat you during q's
// most recent completed turn.
func (s *State) attackedYouLastTurn(q, you uint8) bool {
	c := s.atkLastCold()
	if c == nil || q >= MaxSeats || you >= MaxSeats {
		return false
	}
	return c[SeatIx(q)]&(1<<you) != 0
}
