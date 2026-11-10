package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A battlefield static granting warp to cards in its controller's hand
// (kop.StatWarpGrant; AffectedZone$ Hand AddKeyword$ Warp:<cost>; Tannuk,
// Steadfast Second; CR 702.185a). gorge derives the keyword on the hand card
// while the static applies (rules warp_grant.go grantedWarpCosts), so the
// kernel reads the static live: no mark is stamped on the card and no State
// byte records it. Each warp instance is its own alternative cost (gorge
// altRowCosts): the printed one first, then the grant's, an instance another
// dominates dropped. The hand offer carries the instance's index on the
// action target (gorge's AltCostIndex), and a cast paying the grant's
// instance marks its entry trigPlWarpG, which the cost builder reads.
// The end-step exile and the recast from exile are the printed warp's
// (kw_warp.go): modeWarped and the DurWarp record carry them.

// warpGrantStatic is the first StatWarpGrant static in force p controls whose
// filter matches card c owned by p (read with p as "you"), or nil.
func (s *State) warpGrantStatic(k *prep, p uint8, c CardID) *kop.Static {
	if k.gen == nil || !k.gen.anyWarpGrant {
		return nil
	}
	var got *kop.Static
	s.eachStaticC(k, sclsWarpG, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatWarpGrant || st.ctrl != p || st.sl == NoSlot {
			return true
		}
		// the card is read as a card (kgen lowers a type, colour and YouOwn
		// filter only), so the cost builder asks it with the spell on the stack
		if !s.matchCardGenAt(k, a.Static.Filter, c, p, p, NoObj, nil) {
			return true
		}
		got = &a.Static
		return false
	})
	return got
}

// warpInsts are a hand card's warp instances in gorge's offer order.
type warpInsts struct {
	n     uint8
	cost  [2]kop.ManaCost
	grant [2]bool // instance i is a StatWarpGrant's
}

// warpInstances are hand card o's warp instances for its owner p: the printed
// warp cost, then a live grant's (gorge altRowCosts), an instance another
// covers dropped and the earlier of two equal ones kept. A card outside the
// hand has no granted instance.
func (s *State) warpInstances(k *prep, p uint8, o Obj) warpInsts {
	ob := &s.objs[o]
	cd := &k.t.Cards[ob.card]
	var all warpInsts
	if cd.HasWarp {
		all.cost[0] = cd.WarpCost
		all.n = 1
	}
	if ob.zone == kop.ZoneHand && ob.owner() == p {
		if st := s.warpGrantStatic(k, p, ob.card); st != nil {
			all.cost[all.n], all.grant[all.n] = st.GrantCost, true
			all.n++
		}
	}
	if all.n < 2 {
		return all
	}
	var out warpInsts
	for i := uint8(0); i < all.n; i++ {
		dropped := false
		for j := uint8(0); j < all.n; j++ {
			if j != i && warpCostCovers(&all.cost[j], &all.cost[i]) && (j < i || !warpCostCovers(&all.cost[i], &all.cost[j])) {
				dropped = true
			}
		}
		if !dropped {
			out.cost[out.n], out.grant[out.n] = all.cost[i], all.grant[i]
			out.n++
		}
	}
	return out
}

// warpGrantCost is the mana cost of a warp cast marked trigPlWarpG: the
// grant in force's warp cost, or (its source gone during the cast) the
// table's first grant's (genPrep.warpGrantCost).
func (s *State) warpGrantCost(k *prep, e *stackEntry) kop.ManaCost {
	if st := s.warpGrantStatic(k, s.objs[e.obj].owner(), e.card); st != nil {
		return st.GrantCost
	}
	return k.gen.warpGrantCost
}

// warpCostCovers reports whether a dominates b (gorge warpCostCovers): both
// plain mana (coloured pips and generic only) and a asks no more of any mana
// type and no more in total, so every payment of b also pays a. Equal costs
// cover each other.
func warpCostCovers(a, b *kop.ManaCost) bool {
	if *a == *b {
		return true
	}
	if !plainMana(a) || !plainMana(b) {
		return false
	}
	ta, tb := int(a.Generic), int(b.Generic)
	for i := range a.Pips {
		if a.Pips[i] > b.Pips[i] {
			return false
		}
		ta += int(a.Pips[i])
		tb += int(b.Pips[i])
	}
	return ta <= tb
}

// plainMana is a cost of coloured pips and generic mana only.
func plainMana(c *kop.ManaCost) bool {
	return *c == kop.ManaCost{Generic: c.Generic, Pips: c.Pips}
}
