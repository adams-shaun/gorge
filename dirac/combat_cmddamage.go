package dirac

// noteCmdDamage adds the combat damage a commander dealt to a player this
// step to that player's tally for the commander's owner (CR 903.10a). It
// reads the per-attacker player damage left after prevention and
// replacement, so only damage actually dealt counts; damage redirected to a
// permanent never reaches dealtPl.
func (s *State) noteCmdDamage(r *dmgRec) {
	if s.cfg&cfgCommander == 0 {
		return
	}
	for m := s.occ; m.any(); m = m.rest() {
		a := m.first()
		n := r.dealtPl[a]
		if n <= 0 || s.perms[a].atk < PlayerTarget0 {
			continue
		}
		o := s.perms[a].obj
		q := s.objs[o].owner()
		if s.pl[q].cmd != o {
			continue
		}
		d := &s.pl[s.perms[a].atk-PlayerTarget0].cmdDmg[q]
		if t := int(*d) + int(n); t > 255 {
			*d = 255
		} else {
			*d = uint8(t)
		}
	}
}

// cmdDamageSBA: a player dealt 21 or more combat damage by one commander
// loses (CR 704.6c, 903.10a).
func (s *State) cmdDamageSBA(k *prep) {
	if s.cfg&cfgCommander == 0 {
		return
	}
	for p := uint8(0); p < 2; p++ {
		if (s.pl[p].cmdDmg[0] >= 21 || s.pl[p].cmdDmg[1] >= 21) && !s.pl[p].lost && !s.cantLose(k, p) {
			s.pl[p].lost = true
		}
	}
}
