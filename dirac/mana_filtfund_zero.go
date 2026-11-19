package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A funding source that also holds a filter whose amount is zero now (Three
// Tree City's "{2}, {T}: Add X mana of one chosen colour" with no creature of
// the chosen type). scriptFilt leaves such a filter out of the census gaps
// (gorge's manaAbilityYieldsNothing), but gorge's prefix search still
// activates the source as a covered one, and the activation's ask lists the
// filter beside the printed {C} while the floating pool pays its fee: a
// legal activation that taps the land and spends the fee for no mana (CR
// 605.1a). The ask is a node, its colour ask another, and each of the five
// colour answers a child (all but the first dedup against it), so a line
// through such a land spends the 160-node budget faster than the plain tap
// the kernel searched; a play whose only script sits past that budget
// (Otawara's channel paid by Starfleet Academy's {5} filter, the land
// tapped first) is one gorge never offers.

// zeroFiltFund reports whether funding source sl's activation asks for a
// filter that yields nothing at this node: a {T} filter whose evaluated
// amount is zero and whose fee the floating pool pays.
func (s *State) zeroFiltFund(k *prep, p uint8, sl Slot) bool {
	if k.gen == nil || !k.gen.anyDynFilt || !k.gen.has(s.cardAt(sl), gfFilt) || s.filtRepeat(k, sl) || s.filtNoTap(k, sl) {
		return false
	}
	a := s.filtAb(k, sl)
	return a.AFlags&kop.AfManaDynamic != 0 && s.dynManaN(k, a, sl) == 0 && s.filtUsable(k, sl) && s.filtPaid(k, p, sl)
}

// fundZeroFilt is the zero-yield filter's branch of funding source sl's ask:
// its colour ask, then one activation per colour answer, each spending the
// fee and adding nothing (the answers key the same state, so only the first
// goes on).
//
//go:noinline
func (fs *filtSearch) fundZeroFilt(k *prep, t *State, sl Slot, funded int, gapUsed bool) bool {
	if !fs.tick() {
		return false
	}
	var order [5]uint8
	n := manaColourOrder(t.filtAb(k, sl), &order)
	needFirst(order[:n], fs.need)
	for _, c := range order[:n] {
		if fs.try(k, t, filtStep{sl: sl, ans: 1 + c, gap: true}, funded+1, gapUsed) {
			return true
		}
	}
	return false
}
