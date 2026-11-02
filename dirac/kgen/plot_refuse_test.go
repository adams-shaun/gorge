package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestPlotRefusals: K:Plot lowers only as a plain mana cost on a
// single-faced card with no alternative or additional cost; every
// other shape is refused with a named reason.
func TestPlotRefusals(t *testing.T) {
	face := func(cost string, kw ...string) *cards.Face {
		return &cards.Face{Name: "Synthetic", ManaCost: cost, Types: []string{"Creature"}, Keywords: kw}
	}
	cases := []struct {
		name  string
		card  *cards.Card
		want  string // "" = lowers without a refusal
		check func(t *testing.T, c *lCard)
	}{
		{
			name: "plain",
			card: &cards.Card{Faces: []*cards.Face{face("1 R", "Plot:2 R")}},
			check: func(t *testing.T, c *lCard) {
				if c.Plot == nil {
					t.Fatal("no plot cost lowered")
				}
				if got := c.Plot.String(); got != "{2R}" {
					t.Errorf("plot cost %s, want {2R}", got)
				}
			},
		},
		{name: "beside kicker", card: &cards.Card{Faces: []*cards.Face{face("1 R", "Kicker:2", "Plot:2 R")}}, want: "Plot beside an alternative or additional cost"},
		{name: "beside warp", card: &cards.Card{Faces: []*cards.Face{face("1 R", "Warp:2 R", "Plot:2 R")}}, want: "Plot beside an alternative or additional cost"},
		{name: "X spell", card: &cards.Card{Faces: []*cards.Face{face("X R", "Plot:2 R")}}, want: "Plot on an {X} spell"},
		{
			name: "multi-face",
			card: &cards.Card{AlternateMode: cards.AlternateModal, Faces: []*cards.Face{
				face("1 R", "Plot:2 R"),
				{Name: "Synthetic Back", Types: []string{"Land"}},
			}},
			want: "Plot on a multi-face card",
		},
		{name: "empty cost", card: &cards.Card{Faces: []*cards.Face{face("1 R", "Plot:")}}, want: `plot cost ""`},
		{name: "non-mana part", card: &cards.Card{Faces: []*cards.Face{face("1 R", "Plot:2 R:Sac<1/Creature>")}}, want: `plot cost "2 R:Sac<1/Creature>"`},
		{name: "X cost", card: &cards.Card{Faces: []*cards.Face{face("1 R", "Plot:X R")}}, want: `plot cost "X R"`},
		{name: "hybrid cost", card: &cards.Card{Faces: []*cards.Face{face("1 R", "Plot:1 RG")}}, want: `plot cost "1 RG"`},
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
			if c.check != nil {
				c.check(t, lc)
			}
			continue
		}
		if !strings.Contains(all, c.want) {
			t.Errorf("%s: refusals %q, want one containing %q", c.name, all, c.want)
		}
	}
}
