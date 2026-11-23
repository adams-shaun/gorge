package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// CheckSoleLifeFilterFeeAbilityOffer pins the activation of Dungeon Shade's
// "{B}: +1/+1" on p0's board Heap Gate ("{T}: Add {C}" and the filter "{1},{T}:
// Add one mana of any color") and Treasonous Ogre ("Pay 3 life: Add {R}", its
// only mana ability: a pain census unit, mana_pain.go prepSoleLifeMana). gorge's
// planner funds the filter's {1} fee from the Ogre's {R} and the filter's
// any-colour mana pays the {B} (CR 605.3a, 601.2g-h, 118.3).
func CheckSoleLifeFilterFeeAbilityOffer(t *testing.T, tbl *kop.Table, id map[string]CardID) {
	t.Helper()
	for _, n := range []string{"Swamp", "Heap Gate", "Treasonous Ogre", "Dungeon Shade"} {
		if _, ok := id[n]; !ok {
			t.Fatalf("precondition: the table lacks %s", n)
		}
	}
	activates := func(sc *scen, o Obj) bool {
		d := sc.s.Step()
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkActivate && Obj(a.Source()) == o {
				return true
			}
		}
		return false
	}
	cfg := LockstepConfig(SurfaceFull)
	cfg.NoElide = true
	sc := newScenTableLib(t, tbl, id["Swamp"], cfg)
	sc.perm(id["Heap Gate"], 0)
	sc.perm(id["Treasonous Ogre"], 0)
	shade := sc.perm(id["Dungeon Shade"], 0)
	if !activates(sc, shade) {
		t.Fatal("the Dungeon Shade pump not offered: the Heap Gate filter's {1} fee is payable from the Ogre's {R} (life), its any-colour mana pays {B}")
	}
	// the Ogre alone makes {R}, and the Gate alone {C}: neither pays {B}
	sc2 := newScenTableLib(t, tbl, id["Swamp"], cfg)
	sc2.perm(id["Treasonous Ogre"], 0)
	shade2 := sc2.perm(id["Dungeon Shade"], 0)
	if activates(sc2, shade2) {
		t.Fatal("the pump offered with only a life source making {R}")
	}
}
