package kgen

func init() { kwHandlers["Enlist"] = (*lowerCtx).lowerEnlistKw }

// lowerEnlistKw lowers a printed bare K:Enlist (CR 702.154a): the printed
// keyword is CardDef.Enlist (kop.KwEnlist), and the kernel poses the
// declare-attackers election from it (dirac/kw_enlist.go: once the
// declaration is answered and its attack tax paid, each declared carrier with
// an eligible creature asks in declaration order, before the declaration's
// taps and Attacks triggers; a yes taps the creature and gives the carrier
// +X/+0 until end of turn, X the creature's power then). Gorge reads the
// printed line directly (engine/rules/enlist.go reads HasKeyword), so no
// expansion line belongs to the head. A GRANTED Enlist (a static's
// AddKeyword$) stays refused: gorge would ask for the derived read too, and
// the kernel reads only the printed field. The Mode$ Enlisted trigger and the
// enlistedThisCombat property refuse on their own lines.
func (lc *lowerCtx) lowerEnlistKw(node, param string) {
	if param != "" {
		lc.refuseNode(node, "enlist with a parameter %q", param)
		return
	}
	lc.out.Enlist = true
}
