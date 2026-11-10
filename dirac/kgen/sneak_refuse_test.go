package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestSneakRefusals: K:Sneak lowers only as a plain mana cost on a
// single-faced card with no alternative or additional cost; every
// other shape is refused with a named reason.
func TestSneakRefusals(t *testing.T) {
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
			card: &cards.Card{Faces: []*cards.Face{face("3 W", "Sneak:1 W")}},
			check: func(t *testing.T, c *lCard) {
				if c.Sneak == nil {
					t.Fatal("no sneak cost")
				}
				if got := c.Sneak.String(); got != "{1W}" {
					t.Errorf("sneak cost %s, want {1W}", got)
				}
			},
		},
		{name: "empty cost", card: &cards.Card{Faces: []*cards.Face{face("1 R", "Sneak:")}}, want: `sneak cost ""`},
		{name: "X cost", card: &cards.Card{Faces: []*cards.Face{face("1 R", "Sneak:X R")}}, want: `sneak cost "X R"`},
		{name: "CardManaCost", card: &cards.Card{Faces: []*cards.Face{face("1 R", "Sneak:CardManaCost")}}, want: `sneak cost "CardManaCost"`},
		{name: "rider", card: &cards.Card{Faces: []*cards.Face{face("1 R", "Sneak:3 B:Card.Creature")}}, want: `sneak cost "3 B:Card.Creature"`},
		{name: "non-mana part", card: &cards.Card{Faces: []*cards.Face{face("1 R", "Sneak:2 R PayLife<2>")}}, want: `sneak cost "2 R PayLife<2>"`},
		{name: "beside kicker", card: &cards.Card{Faces: []*cards.Face{face("1 R", "Kicker:2", "Sneak:2 R")}}, want: "Sneak beside an alternative or additional cost"},
		{name: "beside warp", card: &cards.Card{Faces: []*cards.Face{face("1 R", "Warp:2 R", "Sneak:2 R")}}, want: "Sneak beside an alternative or additional cost"},
		{name: "beside plot", card: &cards.Card{Faces: []*cards.Face{face("1 R", "Plot:2 R", "Sneak:2 R")}}, want: "Sneak beside an alternative or additional cost"},
		{name: "beside flashback", card: &cards.Card{Faces: []*cards.Face{face("1 R", "Flashback:2 R", "Sneak:2 R")}}, want: "Sneak beside an alternative or additional cost"},
		{name: "beside convoke", card: &cards.Card{Faces: []*cards.Face{face("1 R", "Convoke", "Sneak:2 R")}}, want: "Sneak beside an alternative or additional cost"},
		{name: "second sneak", card: &cards.Card{Faces: []*cards.Face{face("1 R", "Sneak:2 R", "Sneak:R")}}, want: "Sneak beside an alternative or additional cost"},
		{name: "X spell", card: &cards.Card{Faces: []*cards.Face{face("X R", "Sneak:2 R")}}, want: "Sneak on an {X} spell"},
		{
			name: "multi-face",
			card: &cards.Card{AlternateMode: cards.AlternateModal, Faces: []*cards.Face{
				face("1 R", "Sneak:2 R"),
				{Name: "Synthetic Back", Types: []string{"Land"}},
			}},
			want: "Sneak on a multi-face card",
		},
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
