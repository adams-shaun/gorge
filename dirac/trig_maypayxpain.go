package dirac

// trigXLethalPain is the mana the one-tap census (availForCapped) leaves out
// of a trigger's "you may pay {X}" bound for p's pain-only sources whose own
// damage would be lethal: painAvailMask drops a source whose damage reaches
// the payer's life (the planner's lethal guard), but gorge's trigger X supply
// (triggeredCostXSupply) counts the payment-window census, where a pain-only
// source is listed by its colour with no lethal guard (windowManaUnitsWith),
// so Nomad Stadium at 1 life still funds X = 1 (CR 605.1a: tapping it for
// mana is legal, the damage is dealt after). A paid-life source (Mana
// Confluence) keeps the guard: the window census prices its life itself.
// Each source counts once, at its whole activation.
//
// Cold: reached only from execTrigX's bound.
func (s *State) trigXLethalPain(k *prep, p uint8) (n int) {
	if !k.painMana {
		return 0
	}
	life := int(s.pl[p].life)
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		pc := s.manaPCGen(k, sl)
		if !pc.painOnly || pc.painLife || pc.painCol == 0 || pc.manaAb < 0 || int(pc.painN) < life || !s.canTapForManaPC(k, sl, pc) {
			continue
		}
		n += max(int(pc.manaN), 1)
	}
	return n
}
