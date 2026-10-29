package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestETBTypeChoiceCarriesItsTypeCategory drives the real corpus as-enters
// ChooseType carriers and asserts the posed decision carries the ETBReplacement
// body's Type$ category, which the SpellBench v2 mapping reads to label the
// choose_name purpose.
func TestETBTypeChoiceCarriesItsTypeCategory(t *testing.T) {
	t.Parallel()
	cases := []struct {
		card string
		want string
	}{
		{"Realmwright", "Basic Land"},
		{"Deification", "Planeswalker"},
		{"Kindred Discovery", "Creature"},
	}
	for _, c := range cases {
		t.Run(c.card, func(t *testing.T) {
			t.Parallel()
			e := handEngine(t, corpusAlternativeCard(t, c.card))
			id := e.G.Zone(state.ZHand, 0)[0]
			etbTypeChoiceLabels(t, e, id)
			d := e.Pending()
			if d == nil {
				t.Fatalf("%s: no pending decision after the entry ask", c.card)
			}
			if d.ResumeSA == nil {
				t.Fatalf("%s: entry type decision has no ResumeSA, so the v2 mapping cannot read Type$", c.card)
			}
			if got := d.ResumeSA.ParamStr(cards.PKType); got != c.want {
				t.Fatalf("%s: ResumeSA Type$ = %q, want %q", c.card, got, c.want)
			}
		})
	}
}
