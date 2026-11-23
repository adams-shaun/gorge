package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

func init() {
	exprKinds[vCmdCastTotal] = kop.ExTotalCommanderCastFromCommandZone
}

// cmdCastValue lowers Count$TotalCommanderCastFromCommandZone (gorge
// count_body_paid.go evalCountBodyPaidCommanderCastFromCommandZone, the
// Host's CommanderCastsFromCommandZone of the resolving controller) to
// kop.ExTotalCommanderCastFromCommandZone: the controller's casts of its own
// commanders from the command zone, summed over its commander slots (CR
// 903.8). ok is false for any other head.
//
// Count$CommanderCastFromCommandZone (one commander) refuses: gorge's Count$
// table maps both spellings to the controller's total, so lowering it as the
// per-commander count would diverge, and lowering it as the total would copy
// a reading the corpus cards (Opal Palace, Myth Unbound, Liesa) do not mean.
func (lc *lowerCtx) cmdCastValue(rest string) (v lValue, ok bool, err error) {
	switch rest {
	case "TotalCommanderCastFromCommandZone":
		return lValue{Kind: vCmdCastTotal}, true, nil
	case "CommanderCastFromCommandZone":
		return lValue{}, true, refuse("Count$CommanderCastFromCommandZone: gorge reads the controller's total for both spellings; the one-commander count is not modelled")
	}
	return lValue{}, false, nil
}
