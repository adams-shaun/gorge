package dirac

// Bloodthirst (CR 702.54a: "If an opponent was dealt damage this turn, this
// permanent enters with N +1/+1 counters on it"; Bloodthirst X: X the damage
// dealt to your opponents this turn). gorge synthesises a Moved replacement
// for it (rules/replacement_etb.go bloodthirstEntryMatch) gated on, or
// counting, Count$DamageOppsTakenThisTurn; kgen lowers the same replacement
// (lower_bloodthirst.go), and kop.ExDmgOppsTaken reads the tally here.
//
// The tally is gorge's Host.DamageTakenThisTurn per player: every damage
// event dealt to the player since the turn began (a combat pass's
// per-attacker player damage, noncombat damage, a pain land's, infect damage
// too, which gives poison rather than life loss), after prevention; cleared
// as the turn changes. Life loss that is not damage does not count. It costs
// no State bytes: a cold-buffer record (coldDmgT) that exists only for
// a table reading ExDmgOppsTaken (genPrep.anyDmgTaken); without it nothing
// is recorded and the read is 0.

// dmgTakenCold is the per-player tally, nil when the table has none.
func (s *State) dmgTakenCold() []int16 { return coldDmgT.of(s) }

var coldDmgT = newCold[int16]("dmgT", func(g *genPrep) int {
	if g.anyDmgTaken {
		return MaxSeats
	}
	return 0
})

// noteDmgTaken records n damage dealt to player p (saturating).
func (s *State) noteDmgTaken(p uint8, n int) {
	if n <= 0 {
		return
	}
	if t := s.dmgTakenCold(); t != nil {
		t[p] = int16(min(int(t[p])+n, 0x7fff))
	}
}

// dmgTakenNewTurn clears the tally as the turn changes.
func (s *State) dmgTakenNewTurn() {
	if t := s.dmgTakenCold(); t != nil {
		clear(t)
	}
}

// noteDmgTakenPass records a combat damage pass's player damage once, after
// its prevention (noteCmdDamagePass's walk; an ordered pass is recorded event
// by event as it lands, dealCombatEvent and dealAsked).
func (s *State) noteDmgTakenPass(r *dmgRec) {
	if s.dmgTakenCold() == nil {
		return
	}
	for m := s.occ; m.any(); m = m.rest() {
		a := m.first()
		if n := r.dealtPl[a]; n > 0 && isPlayerOrNone(s.perms[a].atk) {
			s.noteDmgTaken(CodeSeat(s.perms[a].atk), int(n))
		}
	}
}

// oppsDmgTaken is the damage p's opponents still in the game were dealt this
// turn (gorge count_body_paid.go: the sum over g.AliveFrom(0) but p).
func (s *State) oppsDmgTaken(p uint8) int {
	t := s.dmgTakenCold()
	if t == nil {
		return 0
	}
	v := 0
	for q := uint8(0); q < s.nSeats(); q++ {
		if q != p && !s.out(q) {
			v += int(t[q])
		}
	}
	return v
}
