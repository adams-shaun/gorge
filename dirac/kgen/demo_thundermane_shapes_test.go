package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDemoThundermaneShapes: Thundermane Dragon lowers with no refusal. Its
// library-top permission is the MayPlayFrom static (lower_libtop.go); its
// Static$ True SpellCast haste rider lowers to a static trigger whose one
// body is the timed Animate of the triggering spell, marked spellCarry so the
// kernel stamps the spell entry with the override record and applies it as
// the permanent enters (CR 400.7a; dirac/static_spellcarry.go).
func TestDemoThundermaneShapes(t *testing.T) {
	names := []string{"Thundermane Dragon"}
	lw := lowerNamedPool(t, names)
	if len(lw.cards) != len(names) {
		t.Fatalf("lowered %d cards, want %d", len(lw.cards), len(names))
	}
	for _, c := range lw.cards {
		for _, r := range c.Refusals {
			t.Errorf("%s refused: %s", c.Name, r.Reason)
		}
	}
	_, bs, pt, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range bs {
		for _, r := range b.refusals {
			t.Errorf("%s build refusal: %v", b.def.Name, r)
		}
		if b.def.Name != "Thundermane Dragon" {
			continue
		}
		perm, trigger := false, false
		for i := range b.def.Abilities {
			ab := &b.def.Abilities[i]
			switch ab.Kind {
			case kop.AbStatic:
				perm = true
			case kop.AbTriggered:
				if ab.Trig.Flags3&kop.Tf3Static == 0 || len(ab.Prog) != 1 {
					continue
				}
				in := &ab.Prog[0]
				if in.Op != kop.OpOverride || in.B.N != kop.OvSpellCarry || in.A.N != kop.DurEOT {
					continue
				}
				if in.C < 0 || int(in.C) >= len(pt.overrides) || pt.overrides[in.C].AddKw&kop.KwHaste == 0 {
					t.Errorf("Thundermane Dragon: static SpellCast rider override %d is not the haste grant", in.C)
					continue
				}
				trigger = true
			}
		}
		if !perm {
			t.Error("Thundermane Dragon: no library-top MayPlay static lowered")
		}
		if !trigger {
			t.Error("Thundermane Dragon: no static SpellCast trigger with the carried haste rider")
		}
	}
}
