package rules

// resolve_mayask_dredgefill.go is the board gate of cards.MayAskCondDredgeFill: a
// chain that fills a graveyard and then draws (Mental Note's "mill two, draw a
// card") meets a Dredge election (CR 702.52a) when the card it just put there
// has Dredge, which the board gate at the start of the resolution cannot see
// in a graveyard that does not hold it yet.

import (
	"github.com/adams-shaun/gorge/engine/state"
)

// tapeDredgeFillMayAsk reports whether a card with Dredge sits in a zone the
// chain's graveyard fill can take it from: a library (Mill), a hand (a
// discard), the battlefield (a destroy or sacrifice) or the stack (a counter).
func tapeDredgeFillMayAsk(e *Engine) bool {
	for p := range e.G.Players {
		for _, z := range [...]state.Zone{state.ZLibrary, state.ZHand, state.ZBattlefield} {
			if dredgeIn(e, e.G.Zone(z, state.PlayerID(p))) {
				return true
			}
		}
	}
	return dredgeIn(e, e.G.Stack)
}

func dredgeIn(e *Engine, ids []state.ObjID) bool {
	for _, id := range ids {
		if o := e.G.Obj(id); o != nil {
			if f := o.Face(); f != nil {
				if _, ok := f.KeywordParam("Dredge"); ok {
					return true
				}
			}
		}
	}
	return false
}
