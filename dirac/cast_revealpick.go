package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The Reveal<1/CARDNAME> cost (kop.Cost.RevealSelf; the forecast family, CR
// 702.54a): the offer gate (gorge pay.CostCandidates over the CARDNAME spec)
// matches the source object ALONE -- CR 201.5, a self-reference means this
// object, not another with the same name (engine/effects/filter.go
// matchesSpecFrom) -- so the cost's only candidate is the source itself and
// the activation is payable while the source sits in its controller's hand.
// A battlefield or graveyard source has no candidate, so the gate withholds
// the ability: the forecast cards are never offered from the hand either
// (gorge abilityZoneOK reads no ActivationZone as battlefield only, and
// their Forecast$ zone/timing support is unread). Payment (gorge cast_asks.go
// revealCostAsk) settles the lone candidate unasked and its Note moves
// nothing.

// revealSelfPayable reports whether the built entry's Reveal<1/CARDNAME> cost
// is payable: the source object itself is in the activating player's hand.
func (s *State) revealSelfPayable(k *prep, p uint8, src Obj) bool {
	_ = k
	o := &s.objs[src]
	return o.zone == kop.ZoneHand && o.owner() == p
}
