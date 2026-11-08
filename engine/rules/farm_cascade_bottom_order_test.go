package rules

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// cascadeBottomOrderRun casts Bloodbraid Elf over a stacked four-card
// non-found pile (two lands, two MV>=4 nonlands) with Grizzly Bears fifth,
// and returns the scan order, the order the cascade bottomed it in, and the
// final bottom five after the election is declined.
func cascadeBottomOrderRun(t *testing.T, seed uint64) (scan, bottomed, final []state.ObjID, grizzly state.ObjID) {
	t.Helper()
	names := []string{"Forest", "Mountain", "Hill Giant", "Giant Spider", "Grizzly Bears"}
	e, cfg := cascadeTestEngine(t, seed, "Bloodbraid Elf", names, nil)
	lib := e.G.Zone(state.ZLibrary, 0)
	for i, name := range names {
		o := e.G.Obj(lib[i])
		if o == nil || o.Face() == nil || o.Face().Name != name {
			t.Fatalf("library[%d] is not %s", i, name)
		}
		if i < 4 && !o.Face().IsLand() && o.Face().ManaValue() < 4 {
			t.Fatalf("%s would be found by the scan", name)
		}
	}
	grizzly = lib[4]
	scan = append([]state.ObjID(nil), lib[:4]...)
	want := append([]state.ObjID(nil), scan...)
	snap := e.rng.clone()
	before := e.RNGDraws()
	elf := searchMoveByName(t, e, "Bloodbraid Elf", state.ZHand)
	addMana(t, e, 0, "GGRR")
	castFixture(t, e, elf, -1)
	cascadeElection(t, e, "Grizzly Bears")
	if got := e.RNGDraws() - before; got != 3 {
		t.Fatalf("cascade drew %d rng values, want 3 (one Fisher-Yates pass over four cards)", got)
	}
	snap.Shuffle(want)
	nl := e.G.Zone(state.ZLibrary, 0)
	bottomed = append([]state.ObjID(nil), nl[len(nl)-4:]...)
	if !reflect.DeepEqual(bottomed, want) {
		t.Fatalf("bottomed %v, want the seeded shuffle %v", bottomed, want)
	}
	if reflect.DeepEqual(want, scan) {
		t.Fatalf("seed %d shuffle left the scan order unchanged", seed)
	}
	submitChoices(t, e)
	passUntilStackEmpty(t, e, 40)
	nl = e.G.Zone(state.ZLibrary, 0)
	final = append([]state.ObjID(nil), nl[len(nl)-5:]...)
	if !reflect.DeepEqual(final, append(append([]state.ObjID(nil), want...), grizzly)) {
		t.Fatalf("final bottom five %v, want shuffled rest then the found card %v", final, grizzly)
	}
	replayCheck(t, e, cfg)
	return scan, bottomed, final, grizzly
}

// TestFarmCascadeBottomOrderIsSeededShuffle pins CR 702.85a: the cards a
// cascade exiled without finding go on the bottom in a random order (the
// engine's seeded Fisher-Yates), not the order the scan exiled them in; the
// found card (declined) still goes last.
func TestFarmCascadeBottomOrderIsSeededShuffle(t *testing.T) {
	t.Parallel()
	_, a, _, _ := cascadeBottomOrderRun(t, 9213)
	_, b, _, _ := cascadeBottomOrderRun(t, 9214)
	if reflect.DeepEqual(a, b) {
		t.Fatalf("seeds 9213 and 9214 bottomed the same order %v", a)
	}
}
