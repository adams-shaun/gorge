package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAltCostXGrantLowers: Kentaro, the Smiling Cat's battlefield grant ("You
// may pay {X} rather than pay the mana cost for Samurai spells you cast, where
// X is that spell's mana value"; ValidSA$ Spell.Samurai | XAlternative$
// Count$CardManaCost) lowers as a StatAlternativeCost grant whose Filter is
// the Samurai spell filter and whose mana is one X. The kernel prices that X
// at the cast card's mana value (cast_altgrant.go). The lowering refused the
// static as "AlternativeCost shape".
func TestAltCostXGrantLowers(t *testing.T) {
	all, bs, pt := keclBuildPT(t, []string{"Kentaro, the Smiling Cat"})
	i := keclIndex(t, all, "Kentaro, the Smiling Cat")
	if r := lowerRefusals(all[i]); len(r) != 0 {
		t.Fatalf("Kentaro refused: %v", r)
	}
	n := 0
	for _, a := range bs[i].def.Abilities {
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatAlternativeCost || a.Static.StFlags&kop.StAltGrant == 0 {
			continue
		}
		n++
		if a.Cost.Flags != 0 || a.Cost.N != 0 {
			t.Errorf("grant cost has a non-mana part: flags %b N %d", a.Cost.Flags, a.Cost.N)
		}
		if a.Cost.Mana != (kop.ManaCost{X: 1}) {
			t.Errorf("grant mana = %+v, want one X", a.Cost.Mana)
		}
		if a.Static.Filter == 0 {
			t.Fatal("grant carries no Samurai filter")
		}
		f := pt.filters[a.Static.Filter]
		if f.Sub == 0 || int(f.Sub-1) >= len(pt.subtypes) || pt.subtypes[f.Sub-1] != "Samurai" {
			t.Errorf("grant filter subtype = %d (%v), want Samurai", f.Sub, pt.subtypes)
		}
	}
	if n != 1 {
		t.Fatalf("%d StatAlternativeCost grants, want 1", n)
	}
}
