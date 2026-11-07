package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Funding sources of the filter search (filtSearch.visit's second pass).
// gorge's prefix search (rules scriptSearch.visit) activates every covered
// "activate" option while funding lasts, a Treasure's "{T}, Sacrifice this
// token" among them, and answers each colour ask in the ask's order with the
// play's needed colours first (scriptOrderAnswers, a stable partition). Every
// one of those branches is a node of its 160-node budget, so the kernel's
// search must branch the same way or it finds a script gorge's search runs out
// of budget before reaching (CR 605.3a: a Treasure's mana ability is
// activatable while the play is being priced).

// fundOrder writes the colour answers of funding source sl's ask in gorge's
// order: the ability's printed colour order (manaColourOrder: Birnin Zana
// Plaza's Combo G W asks G before W), the colours it does not name after in
// W U B R G C order, then the play's needed colours moved first (needFirst).
func (fs *filtSearch) fundOrder(k *prep, t *State, sl Slot, pc *pcard, mask uint8, out *[kop.NumManaKinds]uint8) int {
	n := 0
	var seen uint8
	if cd := &k.t.Cards[t.cardAt(sl)]; pc.manaAb >= 0 && int(pc.manaAb) < len(cd.Abilities) {
		var ord [5]uint8
		m := manaColourOrder(&cd.Abilities[pc.manaAb], &ord)
		for _, c := range ord[:m] {
			if mask&(1<<c) != 0 && n < len(out) {
				out[n] = c
				seen |= 1 << c
				n++
			}
		}
	}
	for m := mask &^ seen; m != 0 && n < len(out); m &= m - 1 {
		out[n] = uint8(bits.TrailingZeros8(m))
		n++
	}
	needFirst(out[:n], fs.need)
	return n
}

// fundSac sacrifices funding source sl after its mana is added, when its mana
// ability sacrifices it (a Treasure): gorge pays that cost at the search's
// priority node, and the token ceases at the next priority's state-based
// check, before the next activation (the grant search's sacrifice, mana_grantedoffers.go).
func (s *State) fundSac(k *prep, sl Slot) {
	if !s.manaPCGen(k, sl).manaSac {
		return
	}
	o := s.perms[sl].obj
	s.fireSac(k, sl)
	s.leave(k, sl, kop.ZoneGraveyard, false)
	if s.g.on && s.objs[o].of&ofToken != 0 && s.objs[o].zone == kop.ZoneGraveyard {
		s.ceaseManaToken(o)
	}
}
