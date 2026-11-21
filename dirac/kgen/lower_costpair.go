package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// discardBesideSacFiltered reports the one filtered-discard-plus-sacrifice
// shape: a Discard<N/Spec> with a hand-card filter beside a counted
// Sac<N/Spec> (Skoa, Embermage: Discard<1/Card.namedSkoa, Embermage>
// Sac<2/Mountain>), a battlefield activated ability's cost. Cost.N and
// Cost.Filter stay the sacrifice's; the discard's count and filter ride
// kop.Cost.DiscardSideN / DiscardSideF. gorge asks the sacrifice, then the
// discard (costObjectAsks), the kernel's pick order by flag bit. No other
// counted part, no second, whole-hand, random, self or announced discard,
// and no announced or exiling sacrifice may ride along. The caller gates the
// activated ability (lc.actCost).
func discardBesideSacFiltered(c lCost) bool {
	if c.DiscardF == nil || c.DiscardN < 1 || c.DiscardN > 255 || c.DiscardSpec != "" || c.DiscardHand || c.DiscardSelf ||
		c.DiscardX || c.Discard2N != 0 || c.Sac == nil || c.SacN < 1 || c.SacN > 255 || c.SacX || c.SacExile {
		return false
	}
	return c.Forage == nil && c.ExileG == nil && !c.SacSelf && c.Loyalty == 0 && !c.LoyaltyX &&
		c.SubCtr == 0 && !c.SubCtrX && c.SubAmongN == 0 && c.AddCtr == 0 &&
		c.PayLife == 0 && !c.LifeX && c.LifeBy == nil && c.TapN == 0 && c.TapPowN == 0 &&
		c.EvidenceN == 0 && !c.EvidenceX && c.Return == nil && c.BlightN == 0 &&
		c.Waterbend == 0 && !c.WaterbendX && c.XMin == 0
}

// sharedCountPairOK reports the two counted cost parts, beside a discard or
// an exile, that may share a Cost.N / Cost.Filter between them in one
// activated ability's cost: a filtered discard beside a sacrifice
// (discardBesideSacFiltered) or an unfiltered discard beside the source's own
// graveyard exile (discardBesideSelfExile).
func sharedCountPairOK(c lCost) bool {
	return discardBesideSelfExile(c) || discardBesideSacFiltered(c)
}

// discardCountPart writes the CostDiscard part's count onto out (cardBuilder.cost
// calls it where the count goes). A discard beside a sacrifice
// (discardBesideSacFiltered) keeps Cost.N as the sacrifice's and carries its
// own count on kop.Cost.DiscardSideN (kernel dirac.discardPart); any other
// discard shares Cost.N through set. Either way the part counts once.
func discardCountPart(out *kop.Cost, c lCost, set func(kop.CostFlags, int), counted *int) {
	switch {
	case discardBesideSacFiltered(c):
		out.Flags |= kop.CostDiscard
		out.DiscardSideN = uint8(c.DiscardN)
		*counted++
	case c.DiscardN > 0:
		set(kop.CostDiscard, c.DiscardN)
	}
}

// discardFilterPart writes the discard's hand-card filter onto out. A filtered
// discard shares Cost.N (the card count) and Cost.Filter (the candidates'
// spec, the hand zone) and so is alone among the counted parts; beside a
// sacrifice (discardBesideSacFiltered) its filter rides kop.Cost.DiscardSideF
// and CostDiscardFiltered stays clear.
func (cb *cardBuilder) discardFilterPart(out *kop.Cost, c lCost) (err error) {
	switch {
	case c.DiscardF == nil:
	case discardBesideSacFiltered(c):
		out.DiscardSideF, err = cb.filter(c.DiscardF, kop.ZoneHand)
	default:
		out.Flags |= kop.CostDiscardFiltered
		out.Filter, err = cb.filter(c.DiscardF, kop.ZoneHand)
	}
	return err
}
