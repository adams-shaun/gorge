package kgen

// foretoldCond lowers a bare Condition$ Foretold (gorge conditionBareMet:
// the resolving spell's FlagForetold cast provenance, "if this spell was
// foretold"; Poison the Cup). The kernel reads the spell's own stack entry
// (spellForetold, the entry mark a foretell cast sets), so only a spell
// ability has the same object, and a stack copy was never cast (CR 707.10).
func (lc *lowerCtx) foretoldCond() (lCond, error) {
	if lc.ab == nil || lc.ab.Kind != akSpell {
		return lCond{}, refuse("Condition$ Foretold outside a spell")
	}
	save := lc.spellProvOK
	lc.spellProvOK = true
	f, err := lc.filter("Card.wasCastFromExile")
	lc.spellProvOK = save
	if err != nil {
		return lCond{}, err
	}
	if len(f.Clauses) != 1 || f.Clauses[0].CmpStat != cmpWasCastSpell || f.Clauses[0].CmpN != spellExile {
		return lCond{}, refuse("Condition$ Foretold: unexpected cast-provenance filter")
	}
	f.Clauses[0].CmpN = spellForetold
	return lCond{Kind: cValue, A: lValue{Kind: vRefCount, Ref: lRef{Kind: rSelf}, F: f}, Op: opGE, B: cst(1)}, nil
}
