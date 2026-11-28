package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// W7WithoutFlanking (kop.W7WithoutFlanking), gorge's withoutFlanking: the
// negation of the exact keyword word objectHasKeyword reads over the derived
// keyword list (CR 702.25a: flanking's blocker must lack flanking). The only
// flanking kgen lowers is a printed bare K:Flanking (CardDef.Flanking; a
// granted one refuses), so the derived list is the printed keywords: a
// battlefield permanent has the flanking any card of its pile prints (CR
// 702.140e), none when face down or having lost all abilities; off the
// battlefield the printed card counts in any zone.

// lacksFlankingPerm is W7WithoutFlanking on a battlefield permanent. genLost
// is the loss read: a one-shot record or an attached lose-all static (an Aura
// like Reprobation) leaves no flanking.
func (s *State) lacksFlankingPerm(k *prep, sl Slot) bool {
	return !s.pilePrints(k, sl, printsFlanking) || s.genLost(k, sl)
}

func printsFlanking(cd *kop.CardDef) bool { return cd.Flanking }

// lacksFlankingCard is W7WithoutFlanking on a card off the battlefield.
func lacksFlankingCard(k *prep, c CardID) bool {
	return c == kop.NoCard || !k.t.Cards[c].Flanking
}
