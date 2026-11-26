package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDemoCharmShapesShapes: the four Charm shapes of the demo decks lower
// with no refusal: a CharmNum$/MinCharmNum$ pair both read off the kicker
// (Inscription of Abundance), a multi-pick charm whose mode targets are
// 0..X (Kozilek's Command), a mode that is a remembered-source prevention
// Effect (Dromoka's Command) and a mode Clone whose template is its own
// target beside CloneTarget$ Valid ...+NotDefinedTargeted (Masterful
// Replication).
func TestDemoCharmShapesShapes(t *testing.T) {
	names := []string{"Dromoka's Command", "Inscription of Abundance", "Kozilek's Command", "Masterful Replication"}
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
	var shield, minKicked, upTo, clone int
	for _, b := range bs {
		for _, r := range b.refusals {
			t.Errorf("%s build refusal: %v", b.def.Name, r)
		}
		if b.def.ModesMinKicked {
			minKicked++
		}
		for _, ab := range b.def.Abilities {
			for _, tg := range ab.Targets[:ab.NTargets] {
				if tg.UpTo && tg.TFlags&kop.TgfBoundsV != 0 && ab.NModes > 0 {
					upTo++
				}
			}
			for _, in := range ab.Prog {
				if in.Op != kop.OpExt {
					continue
				}
				switch in.Ext() {
				case kop.ExtPreventNext:
					if in.C&kop.PrevNextChosenSpell != 0 {
						shield++
					}
				case kop.ExtCloneCopy:
					if in.B.N&kop.CloneExclTmpl != 0 {
						clone++
					}
				}
			}
		}
	}
	// each shape lowers to the representation the kernel reads
	if shield != 1 || minKicked != 1 || upTo != 1 || clone != 1 {
		t.Errorf("spell shield %d, kicked minimum %d, up-to-X mode slot %d, clone template exclusion %d; want one each", shield, minKicked, upTo, clone)
	}
}
