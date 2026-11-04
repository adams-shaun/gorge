package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestMayFlashLowering: K:MayFlashCost lowers as a plain extra mana cost on a
// single-faced card with no alternative or additional cost (CardDef
// HasMayFlash + MayFlashCost); every other shape is refused with a named
// reason.
func TestMayFlashLowering(t *testing.T) {
	face := func(cost string, kw ...string) *cards.Face {
		return &cards.Face{Name: "Synthetic", ManaCost: cost, Types: []string{"Sorcery"}, Keywords: kw}
	}
	cases := []struct {
		name  string
		card  *cards.Card
		want  string // "" = lowers without a refusal
		check func(t *testing.T, c *lCard)
	}{
		{
			name: "plain",
			card: &cards.Card{Faces: []*cards.Face{face("1 U", "MayFlashCost:2")}},
			check: func(t *testing.T, c *lCard) {
				if c.MayFlash == nil {
					t.Fatal("no may-flash cost")
				}
				if got := c.MayFlash.String(); got != "{2}" {
					t.Errorf("may-flash cost %s, want {2}", got)
				}
				var b strings.Builder
				dumpLIR(&b, c)
				if !strings.Contains(b.String(), "mayflash={2}") {
					t.Errorf("listing %q has no mayflash={2}", b.String())
				}
			},
		},
		{name: "behold part", card: &cards.Card{Faces: []*cards.Face{face("1 R", "MayFlashCost:Behold<1/Dragon>")}}, want: `may-flash cost "Behold<1/Dragon>"`},
		{name: "tap part", card: &cards.Card{Faces: []*cards.Face{face("4 B B", "MayFlashCost:tapXType<3/Creature.withFlying>")}}, want: `may-flash cost "tapXType<3/Creature.withFlying>"`},
		{name: "empty cost", card: &cards.Card{Faces: []*cards.Face{face("1 R", "MayFlashCost:")}}, want: `may-flash cost ""`},
		{name: "X extra", card: &cards.Card{Faces: []*cards.Face{face("1 R", "MayFlashCost:X")}}, want: `may-flash cost "X"`},
		{name: "hybrid extra", card: &cards.Card{Faces: []*cards.Face{face("1 R", "MayFlashCost:WU")}}, want: `may-flash cost "WU"`},
		{name: "second keyword", card: &cards.Card{Faces: []*cards.Face{face("1 R", "MayFlashCost:2", "MayFlashCost:1")}}, want: "two MayFlashCost keywords"},
		{name: "beside kicker", card: &cards.Card{Faces: []*cards.Face{face("1 R", "Kicker:2", "MayFlashCost:2")}}, want: "MayFlashCost beside an alternative or additional cost"},
		{name: "beside flashback", card: &cards.Card{Faces: []*cards.Face{face("1 R", "Flashback:2 R", "MayFlashCost:2")}}, want: "MayFlashCost beside an alternative or additional cost"},
		{name: "beside convoke", card: &cards.Card{Faces: []*cards.Face{face("1 R", "Convoke", "MayFlashCost:2")}}, want: "MayFlashCost beside an alternative or additional cost"},
		{name: "X spell (Ghitu Fire)", card: &cards.Card{Faces: []*cards.Face{face("X R", "MayFlashCost:2")}}, want: "MayFlashCost on an {X} spell"},
		{
			name: "multi-face",
			card: &cards.Card{AlternateMode: cards.AlternateModal, Faces: []*cards.Face{
				face("1 R", "MayFlashCost:2"),
				{Name: "Synthetic Back", Types: []string{"Land"}},
			}},
			want: "MayFlashCost on a multi-face card",
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
