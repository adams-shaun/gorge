package kgen

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestKeywordStrive: K:Strive:<plain mana> lowers to the card's own
// StRaisePerTarget raise (Blinding Flare's {R}, Ajani's Presence's {2}{W},
// Launch the Fleet's {1}); a non-plain parameter, a face whose spell has no
// root target (Polymorphous Rush) and a face whose sub-ability targets too
// refuse by name.
func TestKeywordStrive(t *testing.T) {
	want := map[string][2]int16{ // generic, packed pips per charge
		"Blinding Flare":   {0, 1 << (kop.ReduceColorBits * int(kop.R))},
		"Ajani's Presence": {2, 1 << (kop.ReduceColorBits * int(kop.W))},
		"Launch the Fleet": {1, 0},
	}
	all, bs, _, err := buildPool(lowerNamedPool(t, []string{"Blinding Flare", "Ajani's Presence", "Launch the Fleet"}))
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for i, c := range all {
		w, ok := want[c.Name]
		if !ok {
			continue
		}
		seen++
		if r := lowerRefusals(c); len(r) != 0 {
			t.Errorf("%s refused: %v", c.Name, r)
			continue
		}
		n := 0
		for _, a := range bs[i].def.Abilities {
			if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatIncreaseCost {
				continue
			}
			n++
			st := a.Static
			if st.StFlags&kop.StRaisePerTarget == 0 || st.A != w[0] || st.B != w[1] || st.Zone != kop.ZoneCommand {
				t.Errorf("%s: flags %#x A %d B %#x Zone %v, want StRaisePerTarget, %d, %#x, ZoneCommand", c.Name, st.StFlags, st.A, st.B, st.Zone, w[0], w[1])
			}
		}
		if n != 1 {
			t.Errorf("%s: %d StatIncreaseCost abilities, want 1", c.Name, n)
		}
	}
	if seen != len(want) {
		t.Fatalf("%d of %d cards lowered", seen, len(want))
	}

	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte("Blinding Flare\nPolymorphous Rush\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, ratchetCards(t))
	if err != nil {
		t.Fatal(err)
	}
	variant := func(name string, edit func(f *cards.Face)) *cards.Card {
		src, ok := p.byName[name]
		if !ok {
			t.Fatalf("%s not loaded", name)
		}
		f := *src.Faces[0]
		f.Keywords = append([]string(nil), f.Keywords...)
		f.SVars = maps.Clone(f.SVars)
		edit(&f)
		return &cards.Card{Faces: []*cards.Face{&f}}
	}
	setStrive := func(param string) func(f *cards.Face) {
		return func(f *cards.Face) {
			for i, k := range f.Keywords {
				if strings.HasPrefix(k, "Strive:") {
					f.Keywords[i] = "Strive:" + param
				}
			}
		}
	}
	refused := []struct {
		name string
		card *cards.Card
		want string
	}{
		{"hybrid", variant("Blinding Flare", setStrive("R/W")), `Strive cost "R/W" is not plain mana`},
		{"X", variant("Blinding Flare", setStrive("X")), `Strive cost "X" is not plain mana`},
		{"non-mana", variant("Blinding Flare", setStrive("PayLife<2>")), "is not plain mana"},
		{"four pips", variant("Blinding Flare", setStrive("R R R R")), "is not plain mana"},
		{"empty", variant("Blinding Flare", setStrive("")), "is not plain mana"},
		{"no root target", p.byName["Polymorphous Rush"], "Strive on a face whose spell has no root target"},
		{"targeting sub-ability", variant("Blinding Flare", func(f *cards.Face) {
			f.SVars["DBTap"] = "DB$ Tap | ValidTgts$ Creature"
		}), "Strive beside a targeting sub-ability"},
	}
	for _, c := range refused {
		p.byName["Synthetic"] = c.card
		lc := lowerCard(p, "Synthetic", c.card, false)
		var got []string
		for _, r := range lc.Refusals {
			got = append(got, r.Node+": "+r.Reason)
		}
		if all := strings.Join(got, " | "); !strings.Contains(all, c.want) {
			t.Errorf("%s: refusals %q, want one containing %q", c.name, all, c.want)
		}
	}
}
