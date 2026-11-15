package kgen

// drawYouAndTriggeredTarget lowers DB$ Draw | Defined$ You & TriggeredTarget
// (Xyris, the Writhing Storm: "you and that player each draw that many
// cards"). gorge knownDefinedTargets joins the parts in script order, so the
// controller draws first, then the damaged player, each NumCards$. The second
// part is the same referent Defined$ TriggeredTarget names (lc.defined, which
// refuses outside a damage trigger the kernel's trigger object or player
// names). An object referent is no player: refused.
func (lc *lowerCtx) drawYouAndTriggeredTarget(ps *params, remember bool) error {
	if remember {
		return refuse("RememberDrawn$ on a two-player draw")
	}
	if _, ok := ps.get("OptionalDecider"); ok {
		return refuse("OptionalDecider$ on a two-player draw")
	}
	if ps.is("Upto", "True") {
		return refuse("Upto$ on a two-player draw")
	}
	other, err := lc.defined("TriggeredTarget")
	if err != nil {
		return err
	}
	if other.Kind != rTriggeredPlayer {
		return refuse("Draw Defined$ You & TriggeredTarget: the referent is not a player")
	}
	ps.get("Defined")
	n, err := lc.amount(ps, "NumCards", 1)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "Draw", Obj: lRef{Kind: rYou}, A: n})
	lc.emit(lInstr{Op: "Draw", Obj: other, A: n})
	return nil
}
