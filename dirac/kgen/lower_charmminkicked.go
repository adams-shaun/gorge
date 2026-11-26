package kgen

import "strings"

// charmMinKicked reads a MinCharmNum$ that is a bare SVar over
// Count$Kicked.0.<lo> beside a CharmNum$ val over Count$Kicked.<hi>.<lo>
// (Inscription of Abundance: "choose one; if this spell was kicked, choose any
// number instead"). gorge's CharmModeBounds evaluates the minimum at each ask
// like the maximum, so a kicked cast chooses 0..hi modes and an unkicked one
// exactly lo. It records the kicked minimum in CardDef.ModesMinKicked and
// reports true; the unkicked arm lo is the offer gate's minimum and the mode
// step's evaluated count (lc.out.ModesMinCount, set by the caller). Any other
// MinCharmNum$ returns false, consuming nothing.
func (lc *lowerCtx) charmMinKicked(ps *params, mv string, val lValue, lo int) bool {
	if val.Kind != vKicked || strings.Contains(mv, "$") {
		return false
	}
	body, ok := lc.face.SVars[mv]
	if !ok {
		return false
	}
	min, err := lc.svarValue(strings.TrimSpace(body), 0)
	if err != nil || min.Kind != vKicked || min.Mul != 0 || min.Div != 0 || min.Add != 0 || min.Neg || len(min.Sub) != 2 ||
		!isConstN(min.Sub[0], 0) || !isConstN(min.Sub[1], lo) {
		return false
	}
	ps.consume("MinCharmNum")
	lc.out.ModesMinKicked = true
	return true
}
