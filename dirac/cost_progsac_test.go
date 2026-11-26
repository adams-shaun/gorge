package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// viscScen is Viscerid Drone ({T}, Sacrifice a creature and a Swamp) on the
// battlefield of the costprogsac pool table, with an opposing creature to
// target.
func viscScen(t *testing.T) (*scen, map[string]CardID, Obj) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Viscerid Drone", Count: 2}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Swamp", Count: 2}, {Name: "Snow-Covered Swamp", Count: 2},
	}
	set := cardsettest.Set(t, "costprogsac")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Swamp"], Config{NoElide: true})
	o := sc.perm(id["Viscerid Drone"], 0)
	sc.perm(id["Grizzly Bears"], 1)
	return sc, id, o
}

// TestVisceridDroneTwoSacrifices: the creature part and the Swamp part need
// two permanents. The Drone alone (it is a creature) is not offered; the
// Drone and a Swamp are (the Drone pays the creature part, gorge
// SacrificeRemainderFeasible); a Swamp alone pays ability 0, not the snow
// Swamp's ability 1.
func TestVisceridDroneTwoSacrifices(t *testing.T) {
	sc, id, o := viscScen(t)
	if sc.activateOffered(o, 0) || sc.activateOffered(o, 1) {
		t.Fatal("offered with the Drone alone")
	}
	sc.perm(id["Swamp"], 0)
	if !sc.activateOffered(o, 0) {
		t.Fatal("ability 0 not offered with the Drone and a Swamp")
	}
	if sc.activateOffered(o, 1) {
		t.Fatal("ability 1 offered with no snow Swamp")
	}
}
