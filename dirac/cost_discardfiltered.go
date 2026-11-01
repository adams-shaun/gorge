package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// the candidates of a filtered discard cost
// (CostDiscardFiltered, CR 118.3 / 601.2h; gorge pay.DiscardCandidates over
// MatchesSpecFrom): the payer's hand cards matching the cost's Filter, the
// excluded source (a card being cast from hand) not counted.

// discardCandN counts p's hand cards matching f, excluding exclude.
//
//go:noinline
func (s *State) discardCandN(k *prep, p uint8, f kop.FilterID, exclude Obj) int {
	n := 0
	pl := &s.pl[p]
	for i := 0; i < int(pl.handN); i++ {
		o := pl.hand[i]
		if o == exclude || !s.matchCardGen(k, f, s.objs[o].card, p, p, o) {
			continue
		}
		n++
	}
	return n
}
