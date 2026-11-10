package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// grantedWarpCost is the Warp cost a layer-6 grant gives a card in the hand
// (Tannuk, Steadfast Second: "Artifact cards and red creature cards in your
// hand have warp {2}{R}", CR 702.185a). The cost is the derived Warp keyword's
// parameter, read at the card's current zone; a card outside the hand has none
// (a grant never opens warp from another zone).
func (e *Engine) grantedWarpCost(id state.ObjID) (Cost, bool) {
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZHand {
		return Cost{}, false
	}
	raw, ok := e.derivedKeywordParamAt(id, "Warp", 0)
	if !ok || raw == "" {
		return Cost{}, false
	}
	c := ParseCost(raw)
	if len(c.Unknown) != 0 {
		return Cost{}, false
	}
	return c, true
}

// altRowCost is row m's cost for card id on face f: the printed keyword's cost
// (the shared compiled read), and for the Warp row a hand grant when the face
// prints none. The offer, the cast's charge and the base-cost reader all read
// it, so the three cannot drift.
func (e *Engine) altRowCost(m *altCastMode, id state.ObjID, f *cards.Face) (Cost, bool) {
	if c, ok := m.faceCost(f, e.walkFaceFactsOf(f)); ok {
		return c, true
	}
	if m.id == altWarp {
		return e.grantedWarpCost(id)
	}
	return Cost{}, false
}
