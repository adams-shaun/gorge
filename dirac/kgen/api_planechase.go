package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// api_planechase.go lowers the planechase verbs the kernel models (dirac/
// zone_planechase.go). A plane's own chaos ability is a trigger (lower_trigger.go);
// DB$ ChaosEnsues is the marker that erupts the resolving controller's
// face-up current plane (CR 901.9).
func init() {
	apiOp("ChaosEnsues", (*lowerCtx).apiChaosEnsues)
}

func (lc *lowerCtx) apiChaosEnsues(_ *cards.SA, ps *params) error {
	if d := ps.str("Defined"); d != "" && d != "You" {
		return refuse("ChaosEnsues Defined$ %q", d)
	}
	who, err := lc.objOrTarget(ps, "You", zNone)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "ChaosEnsues", Obj: who})
	return nil
}
