package dirac

// noteCmdDamage adds n combat damage from the permanent at src to player p
// to that player's tally for the commander's owner (CR 903.10a). It is
// called where the damage lands, after prevention and replacement, so a
// damage event redirected to a permanent never reaches it.
func (s *State) noteCmdDamage(src Slot, p uint8, n int) {
	if s.cfg&cfgCommander == 0 || n <= 0 || src == NoSlot {
		return
	}
	o := s.perms[src].obj
	q := s.objs[o].owner()
	j := s.cmdSlot(q, o)
	if j < 0 {
		return
	}
	d := s.cmdDmgAt(p, q, j) // each commander its own tally (CR 903.10a)
	if t := int(*d) + n; t > 255 {
		*d = 255
	} else {
		*d = uint8(t)
	}
}

// noteCmdDamagePass tallies a combat damage pass once, after the pass's
// prevention has run, from the per-attacker player damage the step
// recorded. A pass parked on the replacement order ask is tallied event by
// event as the ordered walk lands it (dealCombatEvent, dealAsked), never
// from the recomputed pass: the recomputed pass holds the damage before the
// walk's replacements, and a resume would tally it again.
func (s *State) noteCmdDamagePass(r *dmgRec) {
	if s.cfg&cfgCommander == 0 || s.g.ro.on {
		return
	}
	for m := s.occ; m.any(); m = m.rest() {
		a := m.first()
		if n := r.dealtPl[a]; n > 0 && isPlayerOrNone(TCode(s.perms[a].atk)) {
			s.noteCmdDamage(a, CodeSeat(TCode(s.perms[a].atk)), int(n))
		}
	}
}

// cmdDamageSBA: a player dealt 21 or more combat damage by one commander
// loses (CR 704.6c, 903.10a).
func (s *State) cmdDamageSBA(k *prep) {
	if s.cfg&cfgCommander == 0 {
		return
	}
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		if s.cmdLethal(p) && !s.pl[p].lost && !s.cantLose(k, p) {
			s.pl[p].lost = true
			s.initLeft(k, p)    // CR 726.4 (act_initiative.go)
			s.monarchLeft(k, p) // CR 725.4 (act_monarch.go)
		}
	}
}

// cmdLethal reports whether one commander has dealt this player 21 or more
// combat damage (CR 903.10a: per commander, a partner pair's never summed).
func (s *State) cmdLethal(p uint8) bool {
	for q := range s.pl[p].cmdDmg {
		if s.pl[p].cmdDmg[q] >= 21 || s.cmdDmg2(p, uint8(q)) >= 21 {
			return true
		}
	}
	return false
}
