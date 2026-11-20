package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// K:Kicker with a non-mana cost (CR 702.33a, Vayne's
// Treachery's Sac<1/Creature;Artifact>, Chocobo Kick's Return<1/Land>),
// copied from gorge (rules cast_altcost.go kickerCost is a full ParseCost;
// cast_begin.go's kicked mode adds it to the cast cost, cost.Plus(kc);
// legal_walk_hand.go offers the kicked mode only when offerCastable holds
// for the base plus the kicker):
//
//   - CardDef.Kicker is zero mana and CardDef.KickerCost is the part: a
//     kicked cast is the ordinary kicked action (AkCast mode 1), offered
//     only when the part is payable (kickerCostOffer);
//   - the part settles through the ordinary cost stages: buildPickCost
//     returns KickerCost on a kicked cast, so the sacrifice picks
//     (ChCostSacrifice) and the return pick (returnPickOptions, paid by
//     payReturnPicks before the sacrifices) are the spell-cost machinery's.
//     kgen refuses a kicker cost beside an additional cost, so there is
//     one pick cost per cast;
//   - a copy keeps sfKicked without paying (CR 707.10): copy_gen.go's
//     mintSpellCopy never builds, so no pick is posed for it.
//
// Every path is behind genPrep.anyKickCost.

// DebugKickerCostFree (mutation test only): a kicked cast pays no kicker
// cost (kicked for free).
var DebugKickerCostFree bool

// kickerCostPick is the cost whose pick parts a kicked spell owes (its
// CardDef.KickerCost, or a Squad/Replicate payment's non-mana part scaled to
// the answered count), or nil.
func (s *State) kickerCostPick(k *prep, e *stackEntry) *kop.Cost {
	if e.flags&sfKicked == 0 || DebugKickerCostFree {
		return nil
	}
	cd := &k.t.Cards[e.card]
	if rc := repeatPart(cd); rc != nil {
		return s.repeatPickCost(cd, s.multikickCount(e)) // kw_squadreplicate.go
	}
	if cd.KickerCost.Flags != 0 {
		return &cd.KickerCost
	}
	return nil
}

// kickerCostOffer: p may cast o (card c) kicked: the kicker's sacrifice or
// return, or a Squad/Replicate payment's non-mana part, has one candidate
// (gorge offerCastable over base + the payment). A card without such a cost
// is always true.
func (s *State) kickerCostOffer(k *prep, p uint8, c CardID, o Obj) bool {
	cd := &k.t.Cards[c]
	if rc := repeatPart(cd); rc != nil {
		one, ok := repeatScale(*rc, 1)
		return ok && s.pickCostPayable(k, p, &one, o)
	}
	kc := &cd.KickerCost
	return kc.Flags == 0 || s.pickCostPayable(k, p, kc, o)
}
