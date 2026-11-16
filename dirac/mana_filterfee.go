package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// filtFeeUnits pays the {N} fee of the filter at sl out of t's sources made so
// far (CR 605.3b, 601.2h): each unit the fee spends leaves the window, so a
// colour the cost's pips need is not counted twice, once as the pip and once
// as the fee that unlocked the filter. A bare total drop would leave the
// fee-paying source's colour in the mask counts and read Forest + Forest +
// Darkwater Catacombs ({1},{T}: add {U}{B}) as {1}{G}{G}. Each unit comes off
// the group whose loss leaves score (the X the rest can pay) highest; a
// one-pip fee takes a unit of its colour. With no unit to take only the total falls.
// A unit is scored on the window with the filter's own output added, since
// the pips may be met only by that output (Swamp, Forest, Catacombs on {X}{B}{B}),
// so the pick does not depend on the sources' slot order.
func (s *State) filtFeeUnits(k *prep, t *avail, sl Slot, fee int, cost *kop.ManaCost, anyColor bool) {
	col, pip := s.filtPip(k, sl)
	for ; fee > 0; fee-- {
		best, bestScore := -1, 0
		for i := 0; i < t.n; i++ {
			if t.cnt[i] == 0 || pip && t.mask[i]&(1<<uint(col)) == 0 {
				continue
			}
			c := *t
			c.removeWide(t.mask[i], 1)
			s.filtProduce(k, &c, sl)
			if sc := c.maxXScore(cost, anyColor); best < 0 || sc > bestScore ||
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

// filtProduce adds the mana the filter at sl makes to t: its units, a one-colour
// filter's colour choice and the TapsForMana extra its tap brings.
func (s *State) filtProduce(k *prep, t *avail, sl Slot) {
	a, units := s.filtAb(k, sl), uint8(s.filtUnits(k, sl))
	addFiltUnits(t, a, units)
	if manaOneColour(a, units) {
		s.addOneColour(t, units)
	}
	if !comboFiltAb(a) {
		// a combination filter's activation keeps the stall (filtDo)
		xv := s.filtExtra(k, sl)
		for c := range xv {
			if xv[c] != 0 {
				t.addUnits(1<<c, xv[c])
			}
		}
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

// maxXScore is the X a can pay for cost under the window's colour rule.
func (a *avail) maxXScore(cost *kop.ManaCost, anyColor bool) int {
	if anyColor {
		return a.maxXAnyColor(cost)
	}
	return a.maxX(cost, pipMaskOf(cost))
}
