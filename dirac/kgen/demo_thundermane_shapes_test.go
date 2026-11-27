package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDemoThundermaneShapes: Thundermane Dragon lowers with no refusal. Its
// library-top permission is the MayPlayFrom static (lower_libtop.go); its
// Static$ True SpellCast haste rider lowers to a static trigger whose body
// is dropped: gorge applies the Animate to the spell but ends it when the
// spell leaves the stack (engine/effects/combatfx.go registerAnimateEffects,
// the timed grant's ExileOnMoved$ Stack), so the entering permanent never has
// the haste and the kernel has nothing to mirror.
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
	_, bs, _, err := buildPool(lw)
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
				if ab.Trig.Flags3&kop.Tf3Static != 0 && len(ab.Prog) == 0 {
					trigger = true
				}
			}
		}
		if !perm {
			t.Error("Thundermane Dragon: no library-top MayPlay static lowered")
		}
		if !trigger {
			t.Error("Thundermane Dragon: no static SpellCast trigger with the dropped haste rider")
		}
	}
}
