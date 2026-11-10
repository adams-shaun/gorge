package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestGatedLargerSameColourMana: a plain {T}: Add {G} beside a gated {T}: Add
// {G}{G} (Whisperer of the Wilds) lowers (the kernel's gated pair unit); an
// amount above maxEachMana is refused, and a different colour, an amount of
// one or an ungated second ability is not that shape.
func TestGatedLargerSameColourMana(t *testing.T) {
	gate := map[string]string{"IsPresent": "Creature.YouCtrl+powerGE4"}
	with := func(ps map[string]string) map[string]string {
		m := map[string]string{}
		for k, v := range gate {
			m[k] = v
		}
		for k, v := range ps {
			m[k] = v
		}
		return m
	}
	const named = "gated {T} ability making more than four mana of the colour a plain {T} ability makes one of"
	cases := []struct {
		name string
		abs  []*cards.SA
		want string
	}{
		{"Whisperer", []*cards.SA{gatedmanaRefuseMana("G", nil), gatedmanaRefuseMana("G", with(map[string]string{"Amount": "2"}))}, ""},
		{"three of the colour", []*cards.SA{gatedmanaRefuseMana("G", nil), gatedmanaRefuseMana("G", with(map[string]string{"Amount": "3"}))}, ""},
		{"four of the colour (Fanatic of Rhonas)", []*cards.SA{gatedmanaRefuseMana("G", nil), gatedmanaRefuseMana("G", with(map[string]string{"Amount": "4"}))}, ""},
		{"five of the colour", []*cards.SA{gatedmanaRefuseMana("G", nil), gatedmanaRefuseMana("G", with(map[string]string{"Amount": "5"}))}, named},
		{"another colour", []*cards.SA{gatedmanaRefuseMana("G", nil), gatedmanaRefuseMana("R", with(map[string]string{"Amount": "2"}))}, "gated mana ability beside a mana ability or cost not modelled"},
		{"gated one of the same colour", []*cards.SA{gatedmanaRefuseMana("G", nil), gatedmanaRefuseMana("G", gate)}, "gated mana ability of the same colour as the first"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			face := &cards.Face{Name: "Synthetic", ManaCost: "no cost", Types: []string{"Creature"}, Abilities: c.abs}
			card := &cards.Card{Faces: []*cards.Face{face}}
			p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
			var rs []string
			for _, r := range lowerCard(p, "Synthetic", card, false).Refusals {
				rs = append(rs, r.Reason)
			}
			got := strings.Join(rs, "; ")
			if c.want == "" {
				if got != "" {
					t.Errorf("refusals %q, want none", got)
				}
				return
			}
			if !strings.Contains(got, c.want) {
				t.Errorf("refusals %q, want %q", got, c.want)
			}
		})
	}
}
