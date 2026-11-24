package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The Cost.Mill payment (gorge cast_commit.go's payMillCostParts over
// pay.PayMillCost, CR 701.17b): the payer mills the count from the top of
// their own library as the cost is paid. gorge emits one real MoveZone event
// per card in top-first order, sharing a mill batch, and the offer gate
// (pay.MillCostPayable) withheld the cost while the library could not cover
// it; a library a mana ability shrank mid-payment clamps (gorge's guard).

// millRevealPayable prices a cost's Mill and Reveal<1/CARDNAME> parts for the
// hand and graveyard offer walks: CR 701.17b, the library must hold the
// summed N (gorge pay.MillCostPayable), and the reveal's only candidate is
// the source itself (CR 201.5, cast_revealpick.go), so a RevealSelf cost is
// payable only off the hand.
func (s *State) millRevealPayable(k *prep, p uint8, src Obj, c *kop.Cost) bool {
	return (c.Mill == 0 || s.pl[p].libLen() >= int(c.Mill)) &&
		(!c.RevealSelf || s.revealSelfPayable(k, p, src))
}

// costTokenPartsPayable is millRevealPayable with the PayLife part's price
// (legal.go canActivate's gate; the walks check life beside their other
// parts): the life cost owes lifeCostOwed, and a LifeIdent part the payer's
// commanders' colour count (cost_lifeident.go).
func (s *State) costTokenPartsPayable(k *prep, p uint8, src Obj, c *kop.Cost) bool {
	return !(c.Flags&kop.CostPayLife != 0 && int(s.pl[p].life) < lifeCostOwed(c)) &&
		s.lifeIdentPayable(k, p, c) &&
		s.millRevealPayable(k, p, src, c)
}

// millCostPay is a cost's mill payment: the same per-player move OpMill's
// loop makes (zone_changezoneparams.go), so a mill replacement reaches it
// through the same machinery (millShuffling, the parked ask) and a
// replacement-free payment is the plain top-first move.
func (s *State) millCostPay(k *prep, p uint8, n int) {
	if n <= 0 {
		return
	}
	if k.gen != nil && k.gen.anyRepl&rbGraveLib != 0 {
		s.millShuffling(k, p, n, 0)
		return
	}
	if k.gen != nil && len(s.cold.parked) != 0 && s.parkMills(k, p, n, 0) {
		return
	}
	pl := &s.pl[p]
	for i := 0; i < n && pl.libLo < pl.libHi; i++ {
		o := pl.lib[pl.libLo]
		pl.libLo++
		s.touch(o)
		s.objs[o].zone = kop.ZoneNone
		s.placeFromK(k, o, kop.ZoneGraveyard, false, kop.ZoneLibrary, false, false)
	}
}
