package dirac

import "testing"

// TestComboFilterOfferedBesideOtherSources: Baxter Building's {4},{T}: four
// mana in any combination is a filter (CR 605.3a), so Captain America's
// {2}{W} is castable off four other lands whatever they are (CR 601.2g): a
// Swamp-only board, a two-colour Pym Technologies among them, and Castle Doom
// whose free {C} pays the fee (its artifact-only any-colour mana cannot).
func TestComboFilterOfferedBesideOtherSources(t *testing.T) {
	for _, b := range []map[string]int{
		{"Swamp": 4},
		{"Swamp": 3, "Pym Technologies": 1},
		{"Swamp": 4, "Castle Doom": 1},
		{"Swamp": 2, "Pym Technologies": 1, "Castle Doom": 1},
	} {
		names := []string{"Captain America, Wings of Freedom", "Baxter Building"}
		for _, n := range []string{"Swamp", "Pym Technologies", "Castle Doom"} {
			if b[n] != 0 {
				names = append(names, n)
			}
		}
		sc, id := synthScen(t, "std-msh", names...)
		sc.untappedPerm(id["Baxter Building"], 0)
		for _, n := range []string{"Swamp", "Pym Technologies", "Castle Doom"} {
			for i := 0; i < b[n]; i++ {
				sc.untappedPerm(id[n], 0)
			}
		}
		c := sc.hand(id["Captain America, Wings of Freedom"], 0)
		if acts := sc.castOptions(c); len(acts) == 0 {
			t.Errorf("board %v: no cast offered", b)
		}
	}
}
