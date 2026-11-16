package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

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

// maxXScore is the X a can pay for cost under the window's colour rule.
func (a *avail) maxXScore(cost *kop.ManaCost, anyColor bool) int {
	if anyColor {
		return a.maxXAnyColor(cost)
	}
	return a.maxX(cost, pipMaskOf(cost))
}
