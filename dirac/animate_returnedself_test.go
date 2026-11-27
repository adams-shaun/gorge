package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// returnedSelfScen is Llanowar Greenwidow in a graveyard with floating mana to
// activate its "return this card from your graveyard to the battlefield
// tapped" ability (Replacements$ ReplaceLeaves on itself).
func returnedSelfScen(t *testing.T) (*scen, Obj) {
	t.Helper()
	set := cardsettest.Set(t, "animatereplacements")
	deck := []cardset.Entry{{Name: "Llanowar Greenwidow", Count: 4}, {Name: "Forest", Count: 16}, {Name: "Swamp", Count: 20}}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	var gw CardID
	found := false
	for _, c := range da {
		if tbl.Cards[c].Name == "Llanowar Greenwidow" {
			gw, found = c, true
		}
	}
	if !found {
		t.Skip("Llanowar Greenwidow not in the pool table")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	sc.s.pl[0].pool[kop.G] = 30
	return sc, sc.grave(gw, 0, 1)[0]
}

// TestAnimateReturnedSelfOverride: the Animate after a graveyard ability's own
// return applies to the returned permanent (a new object, CR 400.7, that
// objRef holds Self away from): Greenwidow's leave-to-exile record exists and
// dieZone exiles it. Activated twice before either resolves, the second
// activation's return finds the card back and moves nothing, yet gorge still
// runs the animate on it: two identical records merge into one with a repeat
// (addOvRecMk), and the CR 616.1 count is two, so dieZone stalls.
func TestAnimateReturnedSelfOverride(t *testing.T) {
	for _, twice := range []bool{false, true} {
		sc, gw := returnedSelfScen(t)
		s := sc.s
		if !sc.activateAb(gw, 0) {
			t.Fatalf("twice=%v: the graveyard activation was not offered", twice)
		}
		if twice && !sc.activateAb(gw, 0) {
			t.Fatalf("twice=%v: the second activation was not offered", twice)
		}
		sc.resolveAll(t)
		if !sc.onBF(gw) {
			t.Fatalf("twice=%v: Greenwidow did not return", twice)
		}
		sl := sc.slot(gw)
		want := 1
		if twice {
			want = 2
		}
		if got := s.overrideExileOnLeaveN(sc.k, sl); got != want {
			t.Fatalf("twice=%v: %d leave-to-exile records on the returned permanent, want %d", twice, got, want)
		}
		s.g.stalled = 0
		if z := s.dieZone(sc.k, sl); z != kop.ZoneExile {
			t.Fatalf("twice=%v: dieZone = %v, want exile", twice, z)
		}
		if got := s.g.stalled == StallReplLeave; got != twice {
			t.Fatalf("twice=%v: StallReplLeave raised = %v", twice, got)
		}
	}
}
