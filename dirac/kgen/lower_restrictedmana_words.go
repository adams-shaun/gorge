package kgen

// Spell-filter words of a spend restriction (RestrictValid$) that are not
// plain card characteristics:
//
//   - Spell.withDevoid lowers through filterWordPred3 (kop.W7WithDevoid, the
//     printed CardDef.Devoid) like the other with<Keyword> words;
//   - Spell.wasCastFromYourGraveyard lowers to the PredWasCastSpell origin
//     spellYourGrave (spellSideCast, under manaRestrict's spellProvOK);
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
// the stack (cmpWasCast -> cmpWasCastSpell): the bare and negated word,
// wasCastFromYourGraveyard (its ByYou spelling too: gorge reads both
// by == you, cast_provenance.go:573) and the bare graveyard-origin word,
// spellGrave (gorge ObjectWasCastFromGraveyard: the zone, a keyword cast
// included). The zero-spend word and webSlinged refuse.
func (lc *lowerCtx) spellSideCast(c *lClause, expr string) error {
	if c.CmpN&wasCastSpentZero != 0 {
		return refuse("filter %q: CastSa Spell.ManaSpent EQ0 on a spell", expr)
	}
	if c.CmpN&wasCastWebSling != 0 {
		return refuse("filter %q: webSlinged on a spell", expr)
	}
	n := spellCast
	switch {
	case c.CmpN == wasCastYourGrave:
		n = spellYourGrave
	case c.CmpN == wasCastGrave:
		n = spellGrave
	case c.CmpN == wasCastHand|wasCastNot:
		// !wasCastFromYourHandByYou on a spell: cast, from anywhere but a
		// hand by you (gorge conditions.go's '!' of the ByYou word)
		n = spellHandBy | spellNot
	case c.CmpN&^wasCastNot != wasCastAny:
		return refuse("filter %q: wasCastFromGraveyard on a spell", expr)
	case c.CmpN&wasCastNot != 0:
		n = spellNot
	}
	c.CmpStat, c.CmpN = cmpWasCastSpell, n
	return nil
}
