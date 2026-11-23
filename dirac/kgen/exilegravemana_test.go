package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func exileGraveLower(t *testing.T, name string) *lCard {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup(name)
	if !ok || card == nil {
		t.Fatalf("%s not in the corpus", name)
	}
	p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
	return lowerCard(p, card.Faces[0].Name, card, false)
}

// TestExileGraveManaLowers: Molt Tender's "{T}, Exile a card from your
// graveyard: Add one mana of any color" lowers with no refusal and builds
// the kop.ExileGManaAb shape; the other cards whose mana abilities carry an
// exile-from-graveyard part keep a refusal that names the mechanism
// they wait on.
func TestExileGraveManaLowers(t *testing.T) {
	lc := exileGraveLower(t, "Molt Tender")
	for _, rf := range lc.Refusals {
		t.Errorf("Molt Tender refused: %s", rf.Reason)
	}
	_, all, _, err := buildPool(&lowered{cards: []*lCard{lc}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	saw := false
	for i := range all[0].def.Abilities {
		if a := &all[0].def.Abilities[i]; kop.ExileGManaAb(a) {
			saw = true
			if a.ManaColors != kop.ColAny || a.Cost.N != 1 || a.Cost.Flags != kop.CostTap|kop.CostExileGrave {
				t.Errorf("mana %b, cost %+v: want any colour, {T} + exile one", a.ManaColors, a.Cost)
			}
		}
	}
	if !saw {
		t.Error("Molt Tender built with no exile-from-graveyard mana ability")
	}

	for _, c := range []struct{ card, want string }{
		{"Sunken Palace", "TriggersWhenSpent$"},
		{"Titans' Nest", "RestrictValid$"},
		{"Rubble Rouser", "rider program"},
	} {
		got := keclReason(exileGraveLower(t, c.card).Refusals)
		if !strings.Contains(got, c.want) {
			t.Errorf("%s: refusals %q, want one naming %q", c.card, got, c.want)
		}
	}

	// The shape is {T} plus ExileFromGrave<1/Spec> and nothing else: a second counted part, a {T}-less
	// cost and a mana fee stay refused on a synthetic probe.
	const body = "Name:Exile Grave Mana Probe\nManaCost:0\nTypes:Artifact\nA:AB$ Mana | Cost$ %s | Produced$ Any | Amount$ 1\n"
	for _, c := range []struct {
		cost string
		ok   bool
	}{
		{"T ExileFromGrave<1/Card>", true},
		{"T ExileFromGrave<2/Card>", false},
		{"ExileFromGrave<1/Card>", false},
		{"1 T ExileFromGrave<1/Card>", false},
		{"T ExileFromGrave<1/CARDNAME>", false},
	} {
		src := strings.Replace(body, "%s", c.cost, 1)
		got := keclReason(discardHandSynth(t, src))
		if c.ok != (got == "") {
			t.Errorf("cost %q: refusals %q, want refused=%v", c.cost, got, !c.ok)
		}
	}
}
