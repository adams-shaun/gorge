package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A painland funding the filter search (filtSearch.visit's second pass).
// gorge's prefix search activates the covered source and its mana ask lists
// every ability the source can activate, in ability order, each one's colours
// in printed order: Karplusan Forest's {C}, then its painful {R} or {G}
// (CR 605.3a, 605.3b: the rider is no cost, so the painful ability is as
// activatable as the plain one). Each answer is a node of gorge's 160-node
// budget, so a kernel search that funded with {C} alone reached a line gorge's
// ran out of budget before (Dauthi Embrace's {B}{B} beside School of the
// Unseen, Karplusan Forest and five Mountains: gorge leaves it unpriced and
// unoffered).

// painFunds reports whether funding source pc's ask lists a painful ability
// beside its plain one (a pain-only source's sole ability is its plain one).
func painFunds(k *prep, pc *pcard) bool {
	return k.painMana && pc.painCol != 0 && !pc.painOnly
}

// fundMaskAt is the colours funding source sl's ask answers: its census
// production and, for a painland, its painful ability's colours.
func (s *State) fundMaskAt(k *prep, pc *pcard, sl Slot) uint8 {
	m := s.srcMaskAt(pc, sl)
	if painFunds(k, pc) {
		m |= uint8(pc.painCol) & 0x3f
	}
	return m
}

// painFundOrder appends to out, after its first n answers, the painful
// ability's colours of mask in printed order (manaColourOrder), skipping the
// ones seen; it returns the new count and seen set.
func painFundOrder(k *prep, t *State, sl Slot, pc *pcard, mask uint8, out []uint8, n int, seen uint8) (int, uint8) {
	if !painFunds(k, pc) {
		return n, seen
	}
	cd := &k.t.Cards[t.cardAt(sl)]
	plain := 0
	for j := range cd.Abilities {
		a := &cd.Abilities[j]
		if a.Kind != kop.AbMana || a.AFlags&kop.AfLatent != 0 {
			continue
		}
		if plain++; plain < 2 {
			continue // the plain ability, the first (prepPainMana)
		}
		var ord [5]uint8
		for _, c := range ord[:manaColourOrder(a, &ord)] {
			if mask&(1<<c) != 0 && seen&(1<<c) == 0 && n < len(out) {
				out[n] = c
				seen |= 1 << c
				n++
			}
		}
		break
	}
	return n, seen
}
