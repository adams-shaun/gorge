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

// grantedWarpCost is the cheapest Warp cost the derived keywords give a card in
// the hand (Tannuk, Steadfast Second: "Artifact cards and red creature cards in
// your hand have warp {2}{R}", CR 702.185a). Every Warp instance is read, not
// the first: a card that prints Warp and is granted another carries both, and
// each is its own alternative cost. The read is at the card's current zone; a
// card outside the hand has none (a grant never opens warp from another zone).
func grantedWarpCost(e altRowReader, id state.ObjID) (Cost, bool) {
	o := e.Game().Obj(id)
	if o == nil || o.Zone != state.ZHand || !e.mayHaveDerivedKeywordH(id, kwhWarp) {
		return Cost{}, false
	}
	var best Cost
	found := false
	for _, k := range e.derivedWith(id, 0).Keywords {
		head := cardsKeywordHead(k)
		if head != kwhWarp.S || len(k) <= len(head)+1 {
			continue
		}
		raw := k[len(head)+1:]
		c := ParseCost(raw)
		if len(c.Unknown) != 0 {
			continue
		}
		if !found || costSize(c) < costSize(best) {
			best, found = c, true
		}
	}
	return best, found
}

// altRowCost is row m's cost for card id on face f: the printed keyword's cost
// (the shared compiled read), and for the Warp row a hand grant too. A card
// that prints Warp and is granted another cost (CR 702.185: each instance is
// its own alternative cost) takes the cheaper of the two, which dominates the
// dearer one because both yield the same warped cast. The offer, the cast's
// charge and the base-cost reader all read it, so the three cannot drift.
func altRowCost(e altRowReader, m *altCastMode, id state.ObjID, f *cards.Face) (Cost, bool) {
	printed, ok := m.faceCost(f, e.walkFaceFactsOf(f))
	if m.id != altWarp {
		return printed, ok
	}
	granted, gok := grantedWarpCost(e, id)
	if !gok || (ok && costSize(printed) <= costSize(granted)) {
		return printed, ok
	}
	return granted, true
}

// costSize is the mana a cost asks for, the order altRowCost picks the
// cheaper of two warp costs by.
func costSize(c Cost) int32 { return c.Colored.Total() + c.Generic }
