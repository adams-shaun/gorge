package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// altRowReader is the narrow read the alternative-cost row readers need (the
// Engine implements it): the object table, the derived-keyword precheck and
// parameter read, and the printed face's compiled facts.
type altRowReader interface {
	Game() *state.Game
	mayHaveDerivedKeywordH(state.ObjID, kwHead) bool
	derivedWith(state.ObjID, state.Zone) Derived
	walkFaceFactsOf(*cards.Face) *walkFaceFacts
}

// grantedWarpCosts are the Warp costs the derived keywords give a card in the
// hand (Tannuk, Steadfast Second: "Artifact cards and red creature cards in
// your hand have warp {2}{R}", CR 702.185a), in keyword order. Every Warp
// instance is read, not the first: a card that prints Warp and is granted
// another carries both. The read is at the card's current zone; a card outside
// the hand has none (a grant never opens warp from another zone).
func grantedWarpCosts(e altRowReader, id state.ObjID) []Cost {
	o := e.Game().Obj(id)
	if o == nil || o.Zone != state.ZHand || !e.mayHaveDerivedKeywordH(id, kwhWarp) {
		return nil
	}
	var out []Cost
	for _, k := range e.derivedWith(id, 0).Keywords {
		head := cardsKeywordHead(k)
		if head != kwhWarp.S || len(k) <= len(head)+1 {
			continue
		}
		if c := ParseCost(k[len(head)+1:]); len(c.Unknown) == 0 {
			out = append(out, c)
		}
	}
	return out
}

// altRowCosts are row m's costs for card id on face f, in offer order: the
// printed keyword's cost (the shared compiled read), and for the Warp row each
// hand grant's after it. CR 702.185a: each warp instance is its own
// alternative cost and the caster picks one, so each surviving instance is its
// own row, addressed by the option's AltCostIndex. An instance another one
// dominates (warpCostCovers) is dropped, the earlier of two equal ones kept:
// any payment for it pays the other, and both yield the same warped cast.
// Incomparable instances ({4} and {2}{R}) both stay.
func altRowCosts(e altRowReader, m *altCastMode, id state.ObjID, f *cards.Face) []Cost {
	var all []Cost
	if c, ok := m.faceCost(f, e.walkFaceFactsOf(f)); ok {
		all = append(all, c)
	}
	if m.id != altWarp {
		return all
	}
	all = append(all, grantedWarpCosts(e, id)...)
	if len(all) < 2 {
		return all
	}
	out := all[:0:0]
	for i := range all {
		dropped := false
		for j := range all {
			if j != i && warpCostCovers(all[j], all[i]) && (j < i || !warpCostCovers(all[i], all[j])) {
				dropped = true
				break
			}
		}
		if !dropped {
			out = append(out, all[i])
		}
	}
	return out
}

// altRowCost is instance idx of row m's costs (altRowCosts), the ONE read the
// hand offer, the cast's charge and the base-cost reader share, so the three
// cannot drift. An out-of-range idx (a stale option) has no cost.
func altRowCost(e altRowReader, m *altCastMode, id state.ObjID, f *cards.Face, idx int) (Cost, bool) {
	costs := altRowCosts(e, m, id, f)
	if idx < 0 || idx >= len(costs) {
		return Cost{}, false
	}
	return costs[idx], true
}

// warpCostCovers reports whether a dominates b: both are plain mana (coloured
// pips and generic only) and a asks no more of any mana type and no more in
// total, so every payment of b also pays a. Equal costs cover each other; a
// cost with any other part neither dominates nor is dominated otherwise.
func warpCostCovers(a, b Cost) bool {
	if a.Equal(&b) {
		return true
	}
	if !plainManaCost(a) || !plainManaCost(b) || a.Colored.Total()+a.Generic > b.Colored.Total()+b.Generic {
		return false
	}
	for i := range a.Colored {
		if a.Colored[i] > b.Colored[i] {
			return false
		}
	}
	return true
}

// plainManaCost is a cost of coloured pips and generic mana only.
func plainManaCost(c Cost) bool {
	plain := Cost{Colored: c.Colored, Generic: c.Generic}
	return c.Equal(&plain)
}
