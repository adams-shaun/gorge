package kgen

// counterTgtFilter lowers a Counter slot's ValidTgts$. A plain "target spell"
// slot (TargetType$ Spell, kop.TgtSpell) admits the cast-provenance words
// (Wash Away's Card.!wasCastFromTheirHand): the kernel reads them off the
// candidate spell's own stack entry (wasCastSpell, spellEntryOf), as gorge's
// stack census does through castProvenanceAdmits over the candidate's latest
// cast, and a copy that was never cast reads false for the positive word, true
// for the negated one in both. wasCastFromTheirHand is the same predicate as
// wasCastFromYourHand there (a cast from a hand, any caster). Only the hand
// (1), exile (3) and cast-not-from-hand (4) families lower; ByYou, bare
// wasCast, MayPlaySource and Mayhem read the chooser or more state, and refuse.
func (lc *lowerCtx) counterTgtFilter(tt, vt string) (*lFilter, error) {
	if tt != "Spell" {
		return lc.filter(vt)
	}
	save, saveT := lc.spellProvOK, lc.tgtProv
	lc.spellProvOK, lc.tgtProv = true, true
	defer func() { lc.spellProvOK, lc.tgtProv = save, saveT }()
	f, err := lc.filter(vt)
	if err != nil {
		return nil, err
	}
	for _, c := range f.Clauses {
		if c.CmpStat != cmpWasCastSpell {
			continue
		}
		if n := c.CmpN & 15; n != spellHand && n != spellExile && n != spellNotHand {
			return nil, refuse("Counter ValidTgts %q: cast provenance outside the origin families", vt)
		}
	}
	return f, nil
}
