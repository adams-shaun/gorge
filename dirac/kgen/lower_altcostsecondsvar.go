package kgen

// altCostSecondSVar reads an AlternativeCost static's CheckSecondSVar$ (with
// SecondSVarCompare$, default GE1) as a second gate ANDed beside CheckSVar$:
// gorge alternativeCostScopeOK (rules/statics_altcost.go) checks both against
// the source's SVars, Forge StaticAbility.checkConditions. Massacre, Submerge,
// the Mercadian Legates, Mogg Salvage: "if an opponent controls an X and you
// control a Y". gorge reads CheckSecondSVar$ on no other static mode, so a
// Continuous line carrying it (Blood Baron of Vizkopa) stays unread and refused.
func altCostSecondSVar(lc *lowerCtx, sp *params, st *lStatic) error {
	cs, ok := sp.get("CheckSecondSVar")
	if !ok {
		return nil
	}
	v, err := lc.value(cs)
	if err != nil {
		return err
	}
	c := lCond{Kind: cValue, A: v, Op: opGE, B: cst(1)}
	if cmp, ok := sp.get("SecondSVarCompare"); ok {
		op, b, err := lc.cmpSpec(cmp)
		if err != nil {
			return err
		}
		c.Op, c.B = op, b
	}
	st.Cond = andCond(st.Cond, c)
	return nil
}
