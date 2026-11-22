package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// W7WithoutFlanking (kop.W7WithoutFlanking), gorge's withoutFlanking: the
// negation of the exact keyword word objectHasKeyword reads over the derived
// keyword list (CR 702.25a: flanking's blocker must lack flanking). The only
// flanking kgen lowers is a printed bare K:Flanking (CardDef.Flanking; a
// granted one refuses), so the derived list is the printed face: a
// battlefield permanent that is face down or lost all abilities has none; off
// the battlefield the printed card counts in any zone.

// lacksFlankingPerm is W7WithoutFlanking on a battlefield permanent.
func (s *State) lacksFlankingPerm(k *prep, sl Slot) bool {
	c := s.cardAt(sl)
	has := c != kop.NoCard && k.t.Cards[c].Flanking && !s.faceDown(k, sl) &&
		s.perms[sl].ovr.flags&(ovLoseAll|ovLoseAllEOT) == 0
	return !has
}

// lacksFlankingCard is W7WithoutFlanking on a card off the battlefield.
func lacksFlankingCard(k *prep, c CardID) bool {
	return c == kop.NoCard || !k.t.Cards[c].Flanking
}
