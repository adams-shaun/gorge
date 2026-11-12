package kgen

// drawUpto lowers a Draw with Upto$ True (gorge effDraw's up-to arm: the
// drawer is asked how many cards, Min 0, Max min(NumCards, library size),
// and draws that many; Arcane Denial's "its controller may draw up to two
// cards") to OpExt ExtDrawUpto. Only the one-player shape lowers: the drawer
// is the controller or the delayed program's trigger player, NumCards$ a
// constant the kernel's pick list holds, no RememberDrawn$ and no
// OptionalDecider$ (the ask is the drawer's own).
func (lc *lowerCtx) drawUpto(ps *params, who lRef, n lValue, remember bool) error {
	if remember {
		return refuse("Draw Upto$ beside RememberDrawn$")
	}
	if _, ok := ps.get("OptionalDecider"); ok {
		return refuse("Draw Upto$ beside OptionalDecider$")
	}
	switch who.Kind {
	case rYou, rTriggeredPlayer:
	default:
		return refuse("Draw Upto$ by %s", who)
	}
	if !n.isConst() || n.N < 1 || n.N > 8 {
		return refuse("Draw Upto$ over a count that is not a constant 1..8")
	}
	ps.consume("Upto", "AILogic") // AILogic$ OptionalDraw: the AI's hint only
	lc.emit(lInstr{Op: "DrawUpto", Obj: who, A: n})
	return nil
}
