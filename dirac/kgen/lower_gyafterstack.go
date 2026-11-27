package kgen

import "strings"

// gyAfterStackRedundant consumes a graveyard permission's ValidAfterStack$
// when it adds nothing to Affected$ (Haakon, Stromgald Scourge: Affected$
// Knight.YouCtrl with ValidAfterStack$ Spell.Knight). gorge asks the
// qualifier of every non-land card the permission offers, as the spell view
// of the card in its origin zone (engine/rules/mayplay.go), and a card in a
// graveyard has the characteristics its spell would, so a qualifier of one
// Spell.<Type> word that every Affected$ alternative already names as its
// head type holds exactly where the alternative does. Any other qualifier
// (a different word, several alternatives, a property after the head) stays
// unread and refuses. The one difference is a land: the qualifier is never
// asked of a land (CR 305.1), so a type word a land can carry (Land,
// Artifact, Creature, Enchantment, Legendary...) is not folded.
func gyAfterStackRedundant(sp *params, aff string) {
	q, ok := sp.m["ValidAfterStack"]
	if !ok {
		return
	}
	word, found := strings.CutPrefix(strings.TrimSpace(q), "Spell.")
	if !found || word == "" || strings.ContainsAny(word, ".+,! ") || landCarriable[word] {
		return
	}
	for _, alt := range strings.Split(aff, ",") {
		head, _, _ := strings.Cut(strings.TrimSpace(alt), ".")
		if head != word {
			return
		}
	}
	sp.consume("ValidAfterStack")
}

// landCarriable are the card type words a land card can have: the qualifier
// would differ from the type for them (never asked of a land).
var landCarriable = map[string]bool{
	"Land": true, "Artifact": true, "Creature": true, "Enchantment": true, "Legendary": true,
	"Basic": true, "Snow": true, "Tribal": true, "Kindred": true, "Permanent": true, "Card": true,
}
