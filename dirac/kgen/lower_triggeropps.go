package kgen

// drawTriggerOpps lowers a Draw with Defined$ TriggeredCardOpponent: gorge
// definedSpecTriggeredCardOpponent, each opponent of the triggering card's
// controller read in AliveFrom(source controller) order (Standstill's "each of
// that player's opponents draws three cards"). Only a SpellCast trigger's
// caster is bound (the kernel's trigger player), and only the plain Draw
// lowers: no RememberDrawn$, Upto$ or OptionalDecider$.
func (lc *lowerCtx) drawTriggerOpps(ps *params) error {
	tr := lc.ab.Trigger
	if tr == nil || tr.Event != "SpellCast" || tr.OrCopy {
		return refuse("Defined$ TriggeredCardOpponent outside a SpellCast trigger")
	}
	for _, k := range [...]string{"RememberDrawn", "RememberDrawnAll", "Upto", "OptionalDecider"} {
		if _, ok := ps.get(k); ok {
			return refuse("Defined$ TriggeredCardOpponent Draw beside %s$", k)
		}
	}
	n, err := lc.amount(ps, "NumCards", 1)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "Draw", Obj: lRef{Kind: rTriggerOpps}, A: n})
	return nil
}
