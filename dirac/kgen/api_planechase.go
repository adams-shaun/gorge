package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// api_planechase.go lowers the planechase verbs the kernel models (dirac/
// zone_planechase.go). A plane's own chaos ability is a trigger (lower_trigger.go);
// DB$ ChaosEnsues is the marker that erupts the resolving controller's
// face-up current plane (CR 901.9), or -- Defined$ Remembered (The Fertile
// Lands of Saulvinia) -- each remembered plane of that controller's own
// planar deck (gorge effChaosEnsues' chaosDestinations arm; the dig's
// RememberFound$ put the revealed plane there, bottom seat, face down, and
// gorge planar.go's checkChaosEnsuesTriggers gates the marker on the plane's
// membership in ev.Player's own deck).
func init() {
	apiOp("ChaosEnsues", (*lowerCtx).apiChaosEnsues)
}

func (lc *lowerCtx) apiChaosEnsues(_ *cards.SA, ps *params) error {
	switch strings.TrimSpace(ps.str("Defined")) {
	case "You", "":
		who, err := lc.objOrTarget(ps, "You", zNone)
		if err != nil {
			return err
		}
		lc.emit(lInstr{Op: "ChaosEnsues", Obj: who})
	case "Remembered":
		lc.emit(lInstr{Op: "ChaosEnsues", Obj: lRef{Kind: rRemembered}})
	default:
		return refuse("ChaosEnsues Defined$ %q", ps.str("Defined"))
	}
	return nil
}
