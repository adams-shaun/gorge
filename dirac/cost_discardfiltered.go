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

// discardPayable reports whether p's hand pays cost c's discard parts
// (exclude: a hand card being cast, not counted). A second part
// (Cost.Discard2N, Foil's alternative cost) counts against the same hand as
// gorge pay.DiscardCostPayable's reservation walk does: the first part
// reserves its first Cost.N candidates in hand order, the second counts its
// candidates among the rest.
func (s *State) discardPayable(k *prep, p uint8, c *kop.Cost, exclude Obj) bool {
	if c.Discard2N > 0 {
		return s.discardTwoPayable(k, p, c, exclude)
	}
	n := int(s.pl[p].handN)
	if c.Flags&kop.CostDiscardFiltered != 0 {
		n = s.discardCandN(k, p, c.Filter, exclude)
	} else if exclude != NoObj && s.objs[exclude].zone == kop.ZoneHand {
		n--
	}
	return n >= int(c.N)
}

// discardTwoPayable is discardPayable's two-part walk.
//
//go:noinline
func (s *State) discardTwoPayable(k *prep, p uint8, c *kop.Cost, exclude Obj) bool {
	pl := &s.pl[p]
	first, second := 0, 0
	for i := 0; i < int(pl.handN); i++ {
		o := pl.hand[i]
		if o == exclude {
			continue
		}
		card := s.objs[o].card
		if first < int(c.N) && (c.Flags&kop.CostDiscardFiltered == 0 || s.matchCardGen(k, c.Filter, card, p, p, o)) {
			first++ // reserved by the first part
			continue
		}
		if c.Discard2F == 0 || s.matchCardGen(k, c.Discard2F, card, p, p, o) {
			second++
		}
	}
	return first >= int(c.N) && second >= int(c.Discard2N)
}

// discardPartTwo: the entry being built has made its cost c's first-part
// discard picks, so the next discard pick is the second part's
// (Cost.Discard2N; gorge discardAsk's next part).
func (s *State) discardPartTwo(c *kop.Cost) bool {
	if c.Discard2N == 0 {
		return false
	}
	n := 0
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().pickKs[i] == pkDiscard {
			n++
		}
	}
	return n >= int(c.N)
}
