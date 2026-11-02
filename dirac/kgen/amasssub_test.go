package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAmassKeepsArmySubtype: the matchup prune must keep the Army subtype
// (and the amass "also a <Type>" grant) the kernel's amass reads by name,
// even when no filter in the matchup names Army.
func TestAmassKeepsArmySubtype(t *testing.T) {
	set := cardsettest.Set(t, "selfcond")
	tbl, _, _, err := set.Matchup(
		[]cardset.Entry{
			{Name: "Lazotep Reaver", Count: 1}, {Name: "Mordor Muster", Count: 1},
			{Name: "Grim Initiate", Count: 1}, {Name: "Gray Ogre", Count: 1},
			{Name: "Swamp", Count: 2},
		},
		[]cardset.Entry{
			{Name: "Grizzly Bears", Count: 1}, {Name: "Forest", Count: 2},
		})
	if err != nil {
		t.Fatal(err)
	}
	idx := func(name string) int {
		for i, n := range tbl.Subtypes {
			if n == name {
				return i
			}
		}
		return -1
	}
	army, orc, zombie := idx("Army"), idx("Orc"), idx("Zombie")
	if army < 0 || orc < 0 || zombie < 0 {
		t.Fatalf("matchup subtypes %v, want Army, Orc and Zombie", tbl.Subtypes)
	}
	// the created Army tokens keep their printed Army bit (+ the Type$ bit)
	for i := range tbl.Cards {
		cd := &tbl.Cards[i]
		switch cd.Name {
		case "b_0_0_orc_army":
			if !cd.Subtypes.Has(army) || !cd.Subtypes.Has(orc) {
				t.Errorf("%s subtypes %v, want Army|Orc", cd.Name, cd.Subtypes)
			}
		case "b_0_0_zombie_army":
			if !cd.Subtypes.Has(army) || !cd.Subtypes.Has(zombie) {
				t.Errorf("%s subtypes %v, want Army|Zombie", cd.Name, cd.Subtypes)
			}
		}
	}
	// the amass "also a <Type>" override is not masked to zero
	for i := range tbl.Cards {
		cd := &tbl.Cards[i]
		if cd.Name != "Lazotep Reaver" {
			continue
		}
		for _, ab := range cd.Abilities {
			for _, in := range ab.Prog {
				if in.Op != kop.OpAmass {
					continue
				}
				ov := int(uint16(in.B.N)&0xfff) - 1
				if ov < 0 || ov >= len(tbl.Overrides) {
					t.Fatalf("amass override index %d", ov)
				}
				if !tbl.Overrides[ov].AddSub.Has(zombie) {
					t.Errorf("amass override %d AddSub %v, want the Zombie bit", ov, tbl.Overrides[ov].AddSub)
				}
			}
		}
	}
}
