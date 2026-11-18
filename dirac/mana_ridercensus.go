package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// depleteLive reports whether the source at sl holds a live depletion rider
// (Gemstone Mine's "{T}, Remove a mining counter: Add one mana of any color",
// Remote Farm's {W}{W}): untapped, still carrying the counter its cost
// removes. gorge's untappedManaSource counts it, though canTapForMana does
// not (the planner defers the rider's first ability).
func (s *State) depleteLive(k *prep, p uint8, sl Slot) bool {
	if k.gen == nil || !k.gen.anyManaRider {
		return false
	}
	a, ok := s.riderAbilityOf(k, sl)
	return ok && a.ManaRider == kop.ManaRiderDeplete && a.Cost.Flags&kop.CostRemoveCtr != 0 && s.riderActivatable(k, p, sl, a)
}

// depleteCensusGap reports whether the untapped source at sl is a live
// depletion rider that gorge's cast-window census leaves unpriced:
// castWindowProbeUnits prices no SubCounter cost, so castWindowSourceExcluded
// finds the source's paid shape unpriced and the census is incomplete. A
// pool-short target menu then keeps its candidates (affordableTargetCandidates'
// fallback, CR 601.2c: the cast only reverses at 601.2h). A source printing a
// plain one-mana {T} ability first (riderPlainFirst) has that unit priced and
// is no gap.
func (s *State) depleteCensusGap(k *prep, p uint8, sl Slot) bool {
	return s.depleteLive(k, p, sl) && !s.riderPlainFirst(k, sl)
}
