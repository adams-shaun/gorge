package dirac

// FeatCommander is seat p's first commander (CR 903.3), NoObj when the seat
// has none; FeatCommanderN names either of a partner pair.
func (s *State) FeatCommander(p uint8) Obj { return s.pl[SeatIx(p)].cmd }

// FeatCommanderN is seat p's commander in slot j (0 or 1; slot 1 is a
// partner, CR 702.124), NoObj when the slot is empty.
func (s *State) FeatCommanderN(p, j uint8) Obj { return s.cmdObj(SeatIx(p), int(j&1)) }

// FeatCommanderGame reports whether s is a Commander game (Config.Commander).
func (s *State) FeatCommanderGame() bool { return s.cfg&cfgCommander != 0 }

// FeatCommanderCast is seat p's commander casts from the command zone
// (CR 903.8), both commanders of a partner pair together.
func (s *State) FeatCommanderCast(p uint8) uint8 {
	p = SeatIx(p)
	return satAdd8(s.pl[p].cmdCast, s.cmdCast2(p))
}

// FeatCommanderCastN is the casts from the command zone of seat p's
// commander in slot j: its own tax count (CR 903.8); 0 for an empty slot 1,
// as in every game without a partner record.
func (s *State) FeatCommanderCastN(p, j uint8) uint8 {
	if j&1 == 0 {
		return s.pl[SeatIx(p)].cmdCast
	}
	return s.cmdCast2(SeatIx(p))
}

// FeatCommanderDamage is the combat damage player p has taken from seat q's
// commander (CR 903.10a); of a partner pair, the larger tally, the one
// nearer the 21 that loses.
func (s *State) FeatCommanderDamage(p, q uint8) uint8 {
	p, q = SeatIx(p), SeatIx(q)
	return max(s.pl[p].cmdDmg[q], s.cmdDmg2(p, q))
}

// FeatCommanderDamageN is the combat damage player p has taken from seat q's
// commander in slot j (CR 903.10a: each commander's own tally); 0 for an
// empty slot 1, as in every game without a partner record.
func (s *State) FeatCommanderDamageN(p, q, j uint8) uint8 {
	p, q = SeatIx(p), SeatIx(q)
	if j&1 == 0 {
		return s.pl[p].cmdDmg[q]
	}
	return s.cmdDmg2(p, q)
}

// FeatLost reports whether seat p has left the game.
func (s *State) FeatLost(p uint8) bool { return s.pl[SeatIx(p)].lost }
