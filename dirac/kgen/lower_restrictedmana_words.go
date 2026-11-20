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
