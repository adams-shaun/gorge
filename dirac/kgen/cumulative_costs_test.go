package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Every corpus card whose cumulative upkeep cost is coloured or hybrid mana,
// life, a land or creature sacrifice or a discard lowers (CR 702.24a; gorge
// startCumulativeUpkeep pays the printed cost once per age counter). The
// costs of the other parseCumulativeAction kinds and a snow cost stay refused.
func TestCumulativeUpkeepCostsLower(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	lower := func(name string) *lCard {
		card, ok := reg.Lookup(name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", name)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		return lowerCard(p, card.Faces[0].Name, card, false)
	}
	dump := func(c *lCard) string {
		var b strings.Builder
		dumpLIR(&b, c)
		return b.String()
	}

	for _, name := range []string{
		"Arctic Nishoba", "Arnjlot's Ascent", "Dystopia", "Earthen Goo", "Firestorm Hellkite", "Gallowbraid",
		"Illusionary Forces", "Illusionary Wall", "Jötun Owl Keeper", "Krovikan Whispers", "Maddening Wind", "Morinfen",
		"Mystic Might", "Phobian Phantasm", "Phyrexian Etchings", "Phyrexian Soulgorger", "Polar Kraken", "Uktabi Efreet",
		"Vexing Sphinx", "Yavimaya Ants",
	} {
		c := lower(name)
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
			continue
		}
		if got := dump(c); !strings.Contains(got, "Cumulative") || !strings.Contains(got, "UnlessPaid obj=You") {
			t.Errorf("%s: lowered IR lacks the cumulative upkeep program:\n%s", name, got)
		}
	}

	// the shape each cost kind lowers to
	for _, tc := range []struct{ name, want string }{
		{"Phobian Phantasm", "Cumulative B=2 C=2"},
		{"Phobian Phantasm", "CS=Mana skip=1"},
		{"Yavimaya Ants", "Cumulative B=2 C=2"},
		{"Mystic Might", "UnlessPaid obj=You A=1 B=8 CS=Mana"},
		{"Arctic Nishoba", "hyb=GW"},
		{"Dystopia", "UnlessPaid obj=You A=CountersOn(Self,AGE)*1 CS=PayLife"},
		{"Polar Kraken", "CS=Sacrifice F={types=0x1 ctrl=1}"},
		{"Phyrexian Soulgorger", "CS=Sacrifice F={types=0x2 ctrl=1}"},
		{"Vexing Sphinx", "UnlessPaid obj=You A=CountersOn(Self,AGE)*1 CS=Discard"},
	} {
		if got := dump(lower(tc.name)); !strings.Contains(got, tc.want) {
			t.Errorf("%s: lowered IR lacks %q:\n%s", tc.name, tc.want, got)
		}
	}

	// the cost kinds that stay refused, by name
	for _, name := range []string{"Aboroth", "Braid of Fire", "Glacial Plating", "Jötun Grunt", "Psychic Vortex", "Wall of Shards"} {
		found := false
		for _, r := range lower(name).Refusals {
			found = found || strings.Contains(r.Reason, "cumulative upkeep cost")
		}
		if !found {
			t.Errorf("%s: want a refusal naming the cumulative upkeep cost", name)
		}
	}
}
