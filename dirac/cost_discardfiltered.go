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
	dn, df, filtered := discardPart(c)
	if filtered {
		n = s.discardCandN(k, p, df, exclude)
	} else if exclude != NoObj && s.objs[exclude].zone == kop.ZoneHand {
		n--
	}
	need := int(dn)
	if c.DiscardX {
		// Discard<X/Spec>: the announced X (Cost.N stays 0). An offer
		// before the announcement reads X = 0, which every hand pays.
		if s.buildLive() {
			need = int(uint8(s.bld().e.x))
		} else {
			need = 0
		}
	}
	return n >= need
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

// useDiscardX: some spell or ability costs an announced Discard<X/Spec>
// (CostDiscard with Cost.N 0): the bsX gate poses the announced-only X ask
// (cast.go, genneed.go).
var useDiscardX = newGenUse(func(t *kop.Table) bool {
	for i := range t.Cards {
		if discardXCost(&t.Cards[i].SpellCost) {
			return true
		}
		for j := range t.Cards[i].Abilities {
			if discardXCost(&t.Cards[i].Abilities[j].Cost) {
				return true
			}
		}
	}
	return false
})

// discardXCost reports whether c discards the announced X cards.
func discardXCost(c *kop.Cost) bool {
	return c != nil && c.DiscardX
}

// discardXBuilding reports whether the entry being built discards the
// announced X.
//
//go:noinline
func (s *State) discardXBuilding(k *prep) bool {
	return discardXCost(s.buildPickCost(k))
}

// discardXClamp bounds the X ask of a Discard<X/Spec> cost being built by the
// matching hand cards the payer can discard (CR 107.3, 601.2b; gorge xAsk
// reads the announced part's candidates, the card being cast excluded).
// Announced only (no {X} mana, cost.X == 0) the count is the bound, offered
// only when X = 0 is payable by the same avail (mx >= 0), else gorge's
// empty-range fallback offers X = 0 alone and the cast reverses; beside a
// printed {X} the count caps the mana bound mx. Cost.N stays 0. X is a uint8.
//
//go:noinline
func (s *State) discardXClamp(k *prep, cost *kop.ManaCost, mx int) int {
	pc := s.buildPickCost(k)
	if pc == nil || !pc.DiscardX {
		return mx
	}
	e := &s.bld().e
	exclude := NoObj
	if e.kind == seSpell {
		exclude = e.obj
	}
	n := int(s.pl[e.ctrl].handN)
	if pc.Flags&kop.CostDiscardFiltered != 0 {
		n = s.discardCandN(k, e.ctrl, pc.Filter, exclude)
	} else if exclude != NoObj && s.objs[exclude].zone == kop.ZoneHand {
		n--
	}
	if n < 0 {
		n = 0
	}
	if n > 255 {
		n = 255
	}
	if cost.X == 0 {
		if mx < 0 {
			return 0
		}
		return n
	}
	return min(mx, n)
}
