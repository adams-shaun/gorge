package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestBlitzCombosRefuse: printed Blitz lowers only beside no other cast cost
// route. The kernel's alt-cost chain sets stackEntry.mode's impending bit for
// the cleaved, surged and overloaded casts before the dashed/blitzed branch
// (dirac/cast.go), and blitzed() reads HasBlitz && impended(), so a printed
// Blitz+Overload card would run the blitz riders (haste, the dies draw, the
// end-step sacrifice) on its overloaded cast; each route refuses. No corpus
// carrier has these combos today; the cases are synthetic.
func TestBlitzCombosRefuse(t *testing.T) {
	face := func(cost, types string, kw []string, ab []*cards.SA) *cards.Face {
		return &cards.Face{Name: "Synthetic", ManaCost: cost, Types: []string{types}, Keywords: kw, Abilities: ab}
	}
	// Winged Portent's shape: the printed spell line, then the cleave line
	// (Cost$, NonBasicSpell$ True).
	cleaveSP := func(cost string) *cards.SA {
		return &cards.SA{Kind: "SP", API: "Draw", Params: map[string]string{
			"Cost": cost, "NumCards": "1", "NonBasicSpell": "True",
			"SpellDescription": "(You may cast this spell for its cleave cost.)",
		}}
	}
	drawSP := &cards.SA{Kind: "SP", API: "Draw", Params: map[string]string{"NumCards": "1"}}
	cases := []struct {
		name string
		card *cards.Card
		want string // "" = lowers without a refusal
	}{
		{
			name: "plain",
			card: &cards.Card{Faces: []*cards.Face{face("2 R", "Creature", []string{"Blitz:3 R"}, nil)}},
		},
		{name: "beside overload", card: &cards.Card{Faces: []*cards.Face{face("2 R", "Creature", []string{"Blitz:3 R", "Overload:4 R"}, nil)}}, want: "Blitz beside an alternative or additional cost"},
		{name: "beside surge", card: &cards.Card{Faces: []*cards.Face{face("2 R", "Creature", []string{"Blitz:3 R", "Surge:1 R"}, nil)}}, want: "Blitz beside an alternative or additional cost"},
		{name: "beside cleave", card: &cards.Card{Faces: []*cards.Face{face("2 R", "Instant", []string{"Blitz:3 R"}, []*cards.SA{drawSP, cleaveSP("4 G U")})}}, want: "Blitz beside an alternative or additional cost"},
	}
	for _, c := range cases {
		p := &pool{byName: map[string]*cards.Card{"Synthetic": c.card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
		lc := lowerCard(p, "Synthetic", c.card, false)
		var got []string
		for _, r := range lc.Refusals {
			got = append(got, r.Reason)
		}
		all := strings.Join(got, " | ")
		if c.want == "" {
			if len(got) != 0 {
				t.Errorf("%s: refused: %s", c.name, all)
			}
			if !lc.Blitz {
				t.Errorf("%s: no lowered blitz cost", c.name)
			}
			continue
		}
		if !strings.Contains(all, c.want) {
			t.Errorf("%s: refusals %q, want one containing %q", c.name, all, c.want)
		}
	}
}
