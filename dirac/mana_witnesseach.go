package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A plain cast's witness can tap an all-at-once restricted source (Troyan,
// Gutsy Explorer's {G}{U}) for more than the cost needs: the {G} pays
// Grove's Bounty ({X}{G}, X = 0) and the {U} floats on. That {U} is still
// Troyan's mana, spendable only as its restriction allows (CR 106.6), so
// gorge floats it restricted and a creature cast later from adventure exile
// (Elusive Otter's {U}, no {X}: CR 715.4, 202.1) cannot spend it. The plain
// rider witness (tapForRider) adds a tapped source's mana to the ordinary
// pool, so a table with such a source untapped takes the restricted-aware
// witness instead (lowerWitnessEx → witnessManaRestricted).

// untappedRestrSrcs is p's untapped restricted planner sources.
func (s *State) untappedRestrSrcs(k *prep, p uint8) slotMask {
	var untapped slotMask
	if !k.restrSrc {
		return untapped
	}
	for m := s.ctrlM[p].and(s.occ); m.any(); m = m.rest() {
		if sl := m.first(); s.perms[sl].flags&pfTapped == 0 && k.gen.flags[s.cardAt(sl)]&gfRestrSrc != 0 {
			untapped = untapped.or(slotBit(sl))
		}
	}
	return untapped
}

// anyRestrEach: some source in m is an all-at-once restricted source, whose
// one tap can make mana the payment leaves floating.
func (s *State) anyRestrEach(k *prep, m slotMask) bool {
	for ; m.any(); m = m.rest() {
		c := s.cardAt(m.first())
		if rec := k.gen.restrRecOf(c); rec != nil && k.t.Cards[c].Abilities[rec.ab].ManaEach {
			return true
		}
	}
	return false
}

// witnessEachRestricted floats the mana all-at-once restricted source sl's
// witness tap added (n of each of its colours, over pool, p's pool before
// the witness) as restricted: floatRestrAb floats every colour.
func (s *State) witnessEachRestricted(k *prep, p uint8, pool *[kop.NumManaKinds]uint8, sl Slot, ab *kop.Ability, n uint8) {
	m := uint8(ab.ManaColors) & 0x1f
	for c := 0; c < 5; c++ {
		if m&(1<<c) != 0 && s.pl[p].pool[c] < pool[c]+n {
			return // the plan did not add this source's every colour
		}
	}
	for c := 0; c < 5; c++ {
		if m&(1<<c) != 0 {
			s.pl[p].pool[c] -= n
		}
	}
	if !s.floatRestrAb(k, p, sl, ab, 0, n) { // a ManaEach ability floats every colour: col is unread
		s.capFull(StallRestrSrc)
	}
}
