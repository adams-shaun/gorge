package kgen

// Spell-filter words of a spend restriction (RestrictValid$) that are not
// plain card characteristics:
//
//   - Spell.withDevoid lowers through filterWordPred3 (kop.W7WithDevoid, the
//     printed CardDef.Devoid) like the other with<Keyword> words;
//   - Spell.wasCastFromYourGraveyard lowers to the PredWasCastSpell origin
//     spellYourGrave (lower.go reads it through lc.spellGraveOK, which
//     manaRestrict sets for this filter only);
//   - Activated.Equip names no object property in gorge: restrictValidTermMatches
//     reads the tail as the object filter Card.Equip, whose unknown word
//     matches nothing (effects.MatchesSpec fails closed), so Ronin, Shadow
//     Stalker's mana never pays an equip activation there. The kernel mirrors
//     that read with a clause that matches no card.

// neverActivatedWord reports whether the Activated.<tail> term's tail is a
// word gorge's object filter does not know and so matches nothing.
func neverActivatedWord(tail string) bool { return tail == "Equip" }

// blankNeverClauses replaces the clauses of f at idx with one that matches no
// card, and reports false when f does not hold one clause per alternative (an
// alternative lowered to several or to none), where idx no longer names them.
func blankNeverClauses(f *lFilter, alts int, idx []int) bool {
	if len(idx) == 0 {
		return true
	}
	if f == nil || len(f.Clauses) != alts {
		return false
	}
	for _, i := range idx {
		f.Clauses[i] = lClause{NotTypes: tAnyCard}
	}
	return true
}

// spellSideCast reads the effects-side wasCast words of clause c on a spell on
// the stack (cmpWasCast -> cmpWasCastSpell): the bare and negated word, and
// wasCastFromYourGraveyard where lc.spellGraveOK allows it. The other
// graveyard spelling, the zero-spend word and webSlinged refuse.
func (lc *lowerCtx) spellSideCast(c *lClause, expr string) error {
	if c.CmpN&wasCastSpentZero != 0 {
		return refuse("filter %q: CastSa Spell.ManaSpent EQ0 on a spell", expr)
	}
	if c.CmpN&wasCastWebSling != 0 {
		return refuse("filter %q: webSlinged on a spell", expr)
	}
	n := spellCast
	switch {
	case c.CmpN == wasCastYourGrave && lc.spellGraveOK:
		n = spellYourGrave
	case c.CmpN&^wasCastNot != wasCastAny:
		return refuse("filter %q: wasCastFromGraveyard on a spell", expr)
	case c.CmpN&wasCastNot != 0:
		n = spellNot
	}
	c.CmpStat, c.CmpN = cmpWasCastSpell, n
	return nil
}
