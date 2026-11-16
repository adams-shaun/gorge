package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// staticRedirectCards are the corpus cards whose only blocker was
// `repl-hard: DamageDone replacement shape` on a printed static redirect
// (R:Event$ DamageDone | ... DamageTarget$ ... | ReplaceWith$ DB$ ReplaceEffect
// | VarName$ Affected): each lowers to one ReplDamageRedirect on its face.
var staticRedirectCards = []string{
	"Empyrial Archangel", "Harsh Judgment", "Martyrs of Korlis", "Palisade Giant",
	"Pariah", "Pariah's Shield", "Protector of the Crown", "Treacherous Link",
	"Veteran Bodyguard", "Weathered Bodyguards",
}

// TestStaticDamageRedirectLowers: every printed "all damage that would be
// dealt to X is dealt to Y instead" static lowers with zero refusals, as one
// ReplDamageRedirect ability.
func TestStaticDamageRedirectLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, name := range staticRedirectCards {
		card, ok := reg.Lookup(name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", name)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		lc := lowerCard(p, card.Faces[0].Name, card, false)
		if len(lc.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, lc.Refusals)
			continue
		}
		n := 0
		for _, r := range lc.Repls {
			if r.Kind == "DamageRedirect" {
				n++
			}
		}
		if n != 1 {
			t.Errorf("%s carries %d DamageRedirect replacements, want 1", name, n)
		}
	}
}
