package dirac

import (
	"testing"
)

// Super-Skrull ({1}{B}{B}{B}) is payable from three Plains, a Swamp, Pym
// Technologies and Baxter Building's "{4}, {T}: Add four mana in any
// combination of colors": four of the other five lands fund the {4}, the four
// any-colour units pay the pips (CR 601.2g-h). gorge offers the cast
// (std-msh pair 166, decision 480); the kernel must too, within gorge's
// potential-script node budget.
func TestComboAnyFilterPaysAColouredCast(t *testing.T) {
	sc, id := stdMshScen(t, "Super-Skrull", "Baxter Building", "Pym Technologies", "Plains", "Swamp")
	var lands []Obj
	for i := 0; i < 3; i++ {
		lands = append(lands, sc.perm(id["Plains"], 0))
	}
	lands = append(lands, sc.perm(id["Swamp"], 0), sc.untappedPerm(id["Pym Technologies"], 0))
	baxter := sc.untappedPerm(id["Baxter Building"], 0)
	lands = append(lands, baxter)
	skrull := sc.hand(id["Super-Skrull"], 0)
	// Preconditions: the six sources are untapped and Baxter's {4},{T} Combo
	// Any is the board's only census gap; the cast is off the plain walk
	// (availFor does not count a filter), so it takes the filter search.
	for _, o := range lands {
		if sc.tapped(o) {
			t.Fatalf("precondition: %s is tapped", sc.k.t.Cards[sc.s.objs[o].card].Name)
		}
	}
	if !sc.s.filtOnly(sc.k, 0) || !sc.s.scriptFilt(sc.k, 0).has(sc.slot(baxter)) {
		t.Fatal("precondition: Baxter Building's {4},{T} Combo Any is not the board's filter gap")
	}
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(skrull)) {
			return
		}
	}
	t.Fatalf("Super-Skrull's {1}{B}{B}{B} not offered over Baxter Building's Combo Any filter; options %d", d.N)
}
