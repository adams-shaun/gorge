package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// Giant's Boulder's "{7}, {T}, Sacrifice this artifact: Destroy target
// permanent" beside Bolg's Company's "{T}, Sacrifice another Goblin: Add
// {B}{R}" (CR 601.2g): the census gaps mix a filter (Boulder's own "{1}, {T}:
// Add one mana of any color") and a live sacrifice-another source, so the
// filter search takes the sacrifice as an otherGap step (mana_mixgaps.go) and
// the gaps walk counts one of each of its colours beside the filter's paid
// unit less its fee. The five untapped lands plus Bolg's {B}{R} are exactly
// the {7}; without the sacrifice step the destroy takes none of the walks and
// gorge's menu offers it (pair 882, std-hob).
func TestSacxGapBesideFilterOffersDestroy(t *testing.T) {
	set := cardsettest.Synth(t, "std-hob")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	sc := newScenTableLib(t, tbl, id["Mountain"], Config{NoElide: true, GorgeAutopaySurface: true})
	board := []string{"Mountain", "Swamp", "Swamp", "Swamp", "Elvenking's Halls",
		"Bolg's Company", "Bothersome Noisemaker", "Giant's Boulder"}
	var objs []Obj
	for _, n := range board {
		objs = append(objs, sc.perm(id[n], 0))
	}
	// Elvenking's Halls enters tapped; the game's copy did not.
	sc.s.perms[sc.slot(objs[4])].flags &^= pfTapped
	if !sc.s.mixBoard(sc.k, 0) {
		t.Fatal("precondition: the filter and the sacrifice-another gap do not mix")
	}
	boulder := objs[len(objs)-1]
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == uint8(boulder) && a.Mode() == 1 {
			return // the destroy activation, mode 1 (the {1} mana ability is mode 0)
		}
	}
	t.Fatalf("Boulder's destroy is not offered: %v", d.Options[:d.N])
}
