package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// An activated ability's XColor$ ("Spend only black mana on X"; Crypt Rats;
// CR 107.3, 601.2b, 601.2g). gorge folds the announced X into that colour's
// pips (Cost.WithX with Cost.XColor): the X ask offers no value that colour's
// mana cannot pay, and the charge spends it. The table encodes the colour as
// Cost.XColor (a byte, 1 + W..G; 0 any mana). The payment's routing still
// reads the printed {X} (gorge's planner leaves a "cost:x" to the manual
// window), so only the charge the window and the payment read is folded:
// buildCostCharge, once X is announced. Cold code behind genPrep.anyXColor.

// prepXColor sets anyXColor.
func prepXColor(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if t.Cards[i].Abilities[j].Cost.XColor != 0 {
				g.anyXColor = true
				return
			}
		}
	}
}

// xColorBuilding is the building activation's Cost.XColor, 0 for none.
func (s *State) xColorBuilding(k *prep) uint8 {
	if k.gen == nil || !k.gen.anyXColor || s.bld().e.kind != seAbility {
		return 0
	}
	if a := s.buildAbility(k); a != nil {
		return a.Cost.XColor
	}
	return 0
}

// xColorCharge is the charge c with the announced X folded into its colour's
// pips (gorge WithX): X and the reduction it would absorb (XRed) are gone, as
// gorge's generic reductions apply after the fold and never reach the
// coloured X. Unchanged before the announcement (the X ask reads the {X}).
func (s *State) xColorCharge(k *prep, c kop.ManaCost) kop.ManaCost {
	col := s.xColorBuilding(k)
	if col == 0 || c.X == 0 || s.bld().stage&0x7f == bsX || s.bld().e.x == 0 {
		return c
	}
	c.Pips[col-1] = uint8(min(255, int(c.Pips[col-1])+int(c.X)*int(s.bld().e.x)))
	c.X, c.XRed = 0, 0
	return c
}

// xColorMaxX is the greatest X the window av can pay with cost's {X} in
// colour col (gorge xAsk's payable walk over WithX). A restricted source or a
// filter widens the plain bound by rules this fold does not price, so the
// board stalls there instead of guessing.
func (s *State) xColorMaxX(k *prep, cost *kop.ManaCost, col uint8, av *avail, anyColor bool) int {
	if k.restrSrc || k.gen.anyMultiR || k.gen.anyFilt {
		s.capFull(StallLowering)
	}
	afford := func(c *kop.ManaCost) bool {
		if anyColor {
			return av.affordsAnyColor(c, 0)
		}
		return av.affords(c, pipMaskOf(c), 0)
	}
	c := *cost
	per := int(c.X)
	c.X, c.XRed = 0, 0
	if !afford(&c) {
		return -1
	}
	x := 0
	for x < maxXAsk {
		n := c
		n.Pips[col-1] = uint8(min(255, int(n.Pips[col-1])+per*(x+1)))
		if !afford(&n) {
			break
		}
		x++
	}
	return x
}
