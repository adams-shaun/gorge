package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCantPreventIsCombat: Frenzied Baloth's "combat damage can't be
// prevented" (IsCombat$ True) lowers to a StatCantPreventDamage carrying
// StNoPrevCombat; the blanket form carries no qualifier; IsCombat$ False and
// a ValidSource$ beside IsCombat$ (Questing Beast's shape) still refuse.
func TestCantPreventIsCombat(t *testing.T) {
	lw := lowerNamedPool(t, []string{"Frenzied Baloth"})
	all, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for i, c := range all {
		if r := lowerRefusals(c); len(r) > 0 {
			t.Errorf("%s: refused: %v", c.Name, r)
		}
		if c.Name != "Frenzied Baloth" {
			continue
		}
		for _, a := range bs[i].def.Abilities {
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatCantPreventDamage {
				n++
				if a.Static.StFlags&kop.StNoPrevCombat == 0 {
					t.Errorf("StFlags %#x, want StNoPrevCombat", a.Static.StFlags)
				}
			}
		}
	}
	if n != 1 {
		t.Fatalf("StatCantPreventDamage statics on Frenzied Baloth = %d, want 1", n)
	}
	for _, c := range []struct{ name, line, want string }{
		{"combat-only lowers", "S:Mode$ CantPreventDamage | IsCombat$ True | Description$ x", ""},
		{"noncombat-only refuses", "S:Mode$ CantPreventDamage | IsCombat$ False | Description$ x", "IsCombat"},
		{"combat-only source-scoped refuses", "S:Mode$ CantPreventDamage | IsCombat$ True | ValidSource$ Creature.YouCtrl | Description$ x", "ValidSource"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := kcantselfRefusals(t, c.line)
			if c.want == "" && got != "" || !strings.Contains(got, c.want) {
				t.Errorf("want refusal %q, got %q", c.want, got)
			}
		})
	}
}
