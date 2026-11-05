package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestStackkwConspireLowering: Raiding Schemes' AffectedZone$ Stack Conspire
// grant lowers to a StackKw static over the nonCreature filter; every shape
// that is not "the caster's spells, plain, ungated, from the battlefield" is
// refused with a named reason.
func TestStackkwConspireLowering(t *testing.T) {
	const base = "Mode$ Continuous | AddKeyword$ Conspire | Affected$ Card.nonCreature+YouCtrl+wasCast | AffectedZone$ Stack | EffectZone$ Battlefield"
	cases := []struct {
		name string
		body string
		want string // "" = lowers
	}{
		{name: "Raiding Schemes", body: base},
		{name: "wasCastByYou", body: strings.Replace(base, "YouCtrl+wasCast", "wasCastByYou", 1)},
		{name: "parameter", body: strings.Replace(base, "AddKeyword$ Conspire", "AddKeyword$ Conspire:1", 1), want: `AffectedZone Stack grant of "Conspire:1"`},
		{name: "gated", body: base + " | Condition$ PlayerTurn", want: "Condition$ / IsPresent$ / CheckSVar$ gate"},
		{name: "is present", body: base + " | IsPresent$ Creature.YouCtrl", want: "Condition$ / IsPresent$ / CheckSVar$ gate"},
		{name: "effect zone", body: strings.Replace(base, "EffectZone$ Battlefield", "EffectZone$ Graveyard", 1), want: `EffectZone "Graveyard"`},
		{name: "caster unnamed", body: strings.Replace(base, "Card.nonCreature+YouCtrl+wasCast", "Card.nonCreature+wasCast", 1), want: "does not name the caster"},
		{name: "foreign provenance word", body: strings.Replace(base, "wasCast", "wasCastSaThing", 1), want: "Affected$ word"},
	}
	for _, c := range cases {
		st, ok := cards.ParseStaticLine(c.body)
		if !ok {
			t.Fatalf("%s: unparsable body", c.name)
		}
		card := &cards.Card{Faces: []*cards.Face{{Name: "Synthetic", ManaCost: "3 R G", Types: []string{"Enchantment"}, Statics: []cards.Static{st}}}}
		p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
		lc := lowerCard(p, "Synthetic", card, false)
		var got []string
		for _, r := range lc.Refusals {
			got = append(got, r.Reason)
		}
		all := strings.Join(got, " | ")
		if c.want != "" {
			if !strings.Contains(all, c.want) {
				t.Errorf("%s: refusals %q, want one containing %q", c.name, all, c.want)
			}
			continue
		}
		if len(got) != 0 {
			t.Errorf("%s: refused: %s", c.name, all)
			continue
		}
		var b strings.Builder
		dumpLIR(&b, lc)
		if !strings.Contains(b.String(), "static StackKw") || !strings.Contains(b.String(), "kw=[Conspire]") {
			t.Errorf("%s: listing has no StackKw Conspire static:\n%s", c.name, b.String())
		}
	}
}
