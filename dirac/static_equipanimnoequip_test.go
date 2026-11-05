package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEquipAnimEquipmentWithoutEquip: Cryptic Coat is an Equipment with no
// Equip ability (its enters trigger cloaks a card and attaches the Coat to
// it), and Behind the Mask animates a target artifact. The Coat made a creature
// is unattached (CR 301.5c, 704.5p), so the Equipment arm of sba must be
// reachable on that table even though no card carries kop.AfEquip
// (std-mkm pair 39: the kernel kept the Coat on its cloaked bearer).
func TestEquipAnimEquipmentWithoutEquip(t *testing.T) {
	build := func(coatTypes kop.TypeMask) *kop.Table {
		tbl := &kop.Table{
			Filters:   []kop.Filter{{}},
			Overrides: []kop.Override{{HasTypes: true, AddTypes: kop.TArtifact | kop.TCreature}},
		}
		var coat, mask kop.CardDef
		coat.Name, coat.Types = "Test Coat", coatTypes
		mask.Name = "Test Mask"
		mask.Abilities = []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{{Op: kop.OpOverride, Obj: kop.RefTarget0, C: 0}}}}
		tbl.Cards = []kop.CardDef{coat, mask}
		for i := range tbl.Cards {
			for j := range tbl.Cards[i].Abilities {
				if tbl.Cards[i].Abilities[j].AFlags&kop.AfEquip != 0 {
					t.Fatalf("precondition: %s carries an Equip ability", tbl.Cards[i].Name)
				}
			}
		}
		return tbl
	}
	for _, tc := range []struct {
		name  string
		types kop.TypeMask
		want  bool
	}{
		{"Equipment without Equip", kop.TArtifact | kop.TEquipment, true},
		{"plain artifact", kop.TArtifact, false},
	} {
		var g genPrep
		prepEquipAnim(build(tc.types), &g)
		if g.equipAnim != tc.want {
			t.Errorf("%s: equipAnim = %v, want %v", tc.name, g.equipAnim, tc.want)
		}
	}
}
