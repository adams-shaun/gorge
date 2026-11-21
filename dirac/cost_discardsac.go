package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// discardPart is the one reader of a cost's discard part: its card count, its
// hand-card filter and whether the hand cards must match that filter. A
// discard beside a sacrifice (Skoa, Embermage's Discard<1/Card.namedSkoa,
// Embermage> Sac<2/Mountain>) keeps Cost.N / Cost.Filter as the sacrifice's
// and carries its own on Cost.DiscardSideN / DiscardSideF; every other
// discard reads Cost.N and, when CostDiscardFiltered, Cost.Filter.
func discardPart(c *kop.Cost) (n uint8, f kop.FilterID, filtered bool) {
	if c.DiscardSideN > 0 && c.Flags&(kop.CostSac|kop.CostDiscard|kop.CostDiscardFiltered) == kop.CostSac|kop.CostDiscard {
		return c.DiscardSideN, c.DiscardSideF, true
	}
	return c.N, c.Filter, c.Flags&kop.CostDiscardFiltered != 0
}

// discardCount is discardPart's count alone.
func discardCount(c *kop.Cost) int {
	n, _, _ := discardPart(c)
	return int(n)
}

// pickCount is the number of picks a cost part of kind k asks first: the
// discard's own count for a discard pick, Cost.N otherwise.
func pickCount(c *kop.Cost, k pickKind) uint8 {
	if k == pkDiscard {
		n, _, _ := discardPart(c)
		return n
	}
	return c.N
}
