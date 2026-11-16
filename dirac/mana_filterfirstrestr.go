package dirac

// filtFeeRestrSrc: the mana of p's restricted planner sources that pay the fee
// of the first-only filter at filt (CR 106.6, 605.1a). gorge's potential mana
// counts a source whose only mana is restricted (Sage of the Unknowable's
// "Spend this mana only to cast a colorless spell or to activate an ability"),
// and a mana ability is an activated ability, so that mana funds Ferrous Lake's
// {1}: the filter is a unit of the census though no printed source pays its
// fee. A restricted source that widens a census unit of its own is already in
// filtOtherMana's count; only the units of their own (rkExt, rkDyn) are added.
//
//go:noinline
func (s *State) filtFeeRestrSrc(k *prep, p uint8, filt Slot) int {
	if !k.restrSrc {
		return 0
	}
	d := payDesc{class: pdAct, card: s.cardAt(filt), src: filt}
	n := 0
	for m := s.ctrlM[SeatIx(p)]; m.any(); m = m.rest() {
		sl := m.first()
		if sl == filt || k.gen.flags[s.cardAt(sl)]&gfRestrSrc == 0 {
			continue
		}
		v, kind, _ := s.restrView(k, p, sl, s.manaPCGen(k, sl), &d)
		if v != nil && kind != rkWiden {
			n += int(v.manaN)
		}
	}
	return n
}
