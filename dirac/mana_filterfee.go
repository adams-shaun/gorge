package dirac

// filtFeeUnits pays the {N} fee of the filter at sl out of t's sources made so
// far (CR 605.3b, 601.2h): each unit the fee spends leaves the window, so a
// colour the cost's pips need is not counted twice, once as the pip and once
// as the fee that unlocked the filter. A bare total drop would leave the
// fee-paying source's colour in the mask counts and read Forest + Forest +
// Darkwater Catacombs ({1},{T}: add {U}{B}) as {1}{G}{G}. Each unit comes off
// the group whose loss leaves score (the X the rest can pay) highest; a
// one-pip fee takes a unit of its colour. With no unit to take, or no score,
// only the total falls.
func (s *State) filtFeeUnits(k *prep, t *avail, sl Slot, fee int, score func(*avail) int) {
	col, pip := s.filtPip(k, sl)
	if score == nil {
		t.total -= fee
		return
	}
	for ; fee > 0; fee-- {
		best, bestScore := -1, 0
		for i := 0; i < t.n; i++ {
			if t.cnt[i] == 0 || pip && t.mask[i]&(1<<uint(col)) == 0 {
				continue
			}
			c := *t
			c.removeWide(t.mask[i], 1)
			if sc := score(&c); best < 0 || sc > bestScore ||
				sc == bestScore && popcount8(t.mask[i]) < popcount8(t.mask[best]) {
				best, bestScore = i, sc
			}
		}
		if best < 0 {
			t.total--
			continue
		}
		t.removeWide(t.mask[best], 1)
	}
}

// popcount8 counts the colours of a production mask.
func popcount8(m uint8) int {
	n := 0
	for ; m != 0; m &= m - 1 {
		n++
	}
	return n
}
