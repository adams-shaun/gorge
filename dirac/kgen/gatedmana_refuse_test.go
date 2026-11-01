package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

func gatedmanaRefuseMana(produced string, ps map[string]string) *cards.SA {
	m := map[string]string{"Cost": "T", "Produced": produced}
	for k, v := range ps {
		m[k] = v
	}
	return &cards.SA{Kind: "AB", API: "Mana", Params: m}
}

// TestGatedManaRefusals: a gated mana ability is lowered only in the two shapes
// the kernel models (a plain gated {T} ability alone, or a Verge: a plain
// ungated {T} ability then a plain gated one of another colour); every other
// gated spelling is refused with a named reason.
func TestGatedManaRefusals(t *testing.T) {
	gate := map[string]string{"IsPresent": "Land.Basic+YouCtrl"}
	cases := []struct {
		name string
		abs  []*cards.SA
		want string // "" = lowers without a refusal
	}{
		{"verge", []*cards.SA{gatedmanaRefuseMana("B", nil), gatedmanaRefuseMana("R", gate)}, ""},
		{"gated below the first", []*cards.SA{gatedmanaRefuseMana("R", nil), gatedmanaRefuseMana("B", gate)}, ""},
		{"gated C beside colour", []*cards.SA{gatedmanaRefuseMana("B", nil), gatedmanaRefuseMana("C", gate)}, "gated mana ability producing {C} beside another"},
		{"gated colour beside C", []*cards.SA{gatedmanaRefuseMana("C", nil), gatedmanaRefuseMana("B", gate)}, "gated mana ability producing {C} beside another"},
		{"single gated", []*cards.SA{gatedmanaRefuseMana("C", gate)}, ""},
		{"present zone", []*cards.SA{gatedmanaRefuseMana("C", map[string]string{"IsPresent": "Land.YouCtrl", "PresentZone": "Graveyard"})}, "gated mana ability PresentZone"},
		{"present player", []*cards.SA{gatedmanaRefuseMana("C", map[string]string{"IsPresent": "Land", "PresentPlayer": "Opponent"})}, "gated mana ability PresentPlayer"},
		{"present defined", []*cards.SA{gatedmanaRefuseMana("C", map[string]string{"IsPresent": "Card.Self", "PresentDefined": "Self"})}, "gated mana ability PresentDefined"},
		{"gate on the first of two", []*cards.SA{gatedmanaRefuseMana("B", gate), gatedmanaRefuseMana("R", nil)}, "gate on the first of two mana abilities"},
		{"both gated", []*cards.SA{gatedmanaRefuseMana("B", gate), gatedmanaRefuseMana("R", gate)}, "gate on the first of two mana abilities"},
		{"three abilities", []*cards.SA{gatedmanaRefuseMana("W", nil), gatedmanaRefuseMana("B", nil), gatedmanaRefuseMana("R", gate)}, "gated mana ability among 3 mana abilities"},
		{"mirrex: any colour beside C", []*cards.SA{gatedmanaRefuseMana("C", nil), gatedmanaRefuseMana("Any", gate)}, ""},
		{"Dark Fortress", []*cards.SA{gatedmanaRefuseMana("C", nil), gatedmanaRefuseMana("Combo B R", gate)}, ""},
		{"Gathering Place", []*cards.SA{gatedmanaRefuseMana("C", nil), gatedmanaRefuseMana("Combo G W", gate)}, ""},
		{"Gleaming Bastion", []*cards.SA{gatedmanaRefuseMana("C", nil), gatedmanaRefuseMana("Combo W U", gate)}, ""},
		{"Hidden Lair", []*cards.SA{gatedmanaRefuseMana("C", nil), gatedmanaRefuseMana("Combo U B", gate)}, ""},
		{"Training Compound", []*cards.SA{gatedmanaRefuseMana("C", nil), gatedmanaRefuseMana("Combo R G", gate)}, ""},
		{"gated Combo with a repeated colour beside C", []*cards.SA{gatedmanaRefuseMana("C", nil), gatedmanaRefuseMana("Combo B B", gate)}, "gated mana ability beside a choice ability"},
		{"gated Combo with {C} beside C", []*cards.SA{gatedmanaRefuseMana("C", nil), gatedmanaRefuseMana("Combo C B", gate)}, "gated mana ability beside a choice ability"},
		{"any colour beside a colour", []*cards.SA{gatedmanaRefuseMana("B", nil), gatedmanaRefuseMana("Any", gate)}, "gated mana ability beside a choice ability"},
		{"choice beside fixed", []*cards.SA{gatedmanaRefuseMana("B", nil), gatedmanaRefuseMana("Combo W U", gate)}, "gated mana ability beside a choice ability"},
		{"same colour", []*cards.SA{gatedmanaRefuseMana("B", nil), gatedmanaRefuseMana("B", gate)}, "gated mana ability of the same colour as the first"},
		{"amount 2", []*cards.SA{gatedmanaRefuseMana("C", map[string]string{"IsPresent": "Land.YouCtrl", "PresentCompare": "GE5", "Amount": "2"})}, "gated mana ability beside a mana ability or cost not modelled"},
		{"extra cost", []*cards.SA{gatedmanaRefuseMana("C", map[string]string{"IsPresent": "Land.YouCtrl", "Cost": "T PayLife<1>"})}, "gated mana ability beside a mana ability or cost not modelled"},
	}
	for _, c := range cases {
		face := &cards.Face{Name: "Synthetic", ManaCost: "no cost", Types: []string{"Land"}, Abilities: c.abs}
		card := &cards.Card{Faces: []*cards.Face{face}}
		p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
		var got []string
		for _, r := range lowerCard(p, "Synthetic", card, false).Refusals {
			got = append(got, r.Reason)
		}
		all := strings.Join(got, " | ")
		if c.want == "" {
			if len(got) != 0 {
				t.Errorf("%s: refused: %s", c.name, all)
			}
			continue
		}
		if !strings.Contains(all, c.want) {
			t.Errorf("%s: refusals %q, want one containing %q", c.name, all, c.want)
		}
	}
}
