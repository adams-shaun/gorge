package kgen

import (
	"github.com/adams-shaun/gorge/engine/cards"
)

// The api: effects for tapping, combat, attachment and control (lower_api.go).
func init() {
	apiPS("Airbend", (*lowerCtx).airbend) // lower_airbend.go
	apiPS("Meld", (*lowerCtx).meld)       // lower_meld.go
	apiOp("Tap", (*lowerCtx).apiTap)
	apiOp("Untap", (*lowerCtx).apiTap)
	apiPS("RemoveFromCombat", (*lowerCtx).removeFromCombat) // lower_removefromcombat.go
	apiPS("Earthbend", (*lowerCtx).earthbend)
	apiPS("Goad", (*lowerCtx).goad) // lower_goad.go
	apiPS("Attach", (*lowerCtx).attach)
	apiPS("GainControl", (*lowerCtx).gainControl)
	apiPS("ExchangeControl", (*lowerCtx).exchangeControl)
	apiPS("UntapAll", (*lowerCtx).untapAll)
	apiPS("TapAll", (*lowerCtx).tapAll)
	apiPS("ElementalBend", (*lowerCtx).elementalBend)
	apiPS("TapOrUntap", (*lowerCtx).tapOrUntap)
	apiPS("TapOrUntapAll", (*lowerCtx).tapOrUntapAll) // lower_tapuntap.go
	apiPS("MustBlock", (*lowerCtx).mustBlockAPI)      // lower_mustblock.go
}

func (lc *lowerCtx) apiTap(sa *cards.SA, ps *params) error {
	if done, err := lc.untapChoose(sa, ps); done || err != nil { // lower_handmove.go
		return err
	}
	obj, err := lc.objOrTarget(ps, "Self", zBattlefield)
	if err != nil {
		return err
	}
	if ps.is("ETB", "True") {
		return refuse("ETB tap outside a replacement")
	}
	lc.emit(lInstr{Op: sa.API, Obj: obj})
	return nil
}
