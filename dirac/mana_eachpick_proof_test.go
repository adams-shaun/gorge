package dirac

import "testing"

// TestEachPickVoidsTheWindowAndMadnessProofs: an all-at-once source's reflected
// pick (srcEachPick) is a production gorge defers (manaProductionAltered), and
// the window census and the madness count proof price no unit for it, so each
// must report the gap itself rather than prove reach from a census that
// includes the pick (srcExtraHit alone reads it as no hit: the census and plan
// price the pick, mana_each_reflectpick_test.go).
func TestEachPickVoidsTheWindowAndMadnessProofs(t *testing.T) {
	sc, sl := eachReflectScen(t)
	pc := sc.s.windowGrantPC(sc.k, sl)
	if r := sc.s.srcEachPick(sc.k, sl, pc); r != 1 {
		t.Fatalf("precondition: srcEachPick = %d, want 1", r)
	}
	if _, hit := sc.s.srcExtraHit(sc.k, sl, pc); hit {
		t.Fatal("precondition: srcExtraHit reports the pick shape (the readers' own gate would already hold)")
	}
	if !sc.s.windowCanTap(sc.k, sl) || sc.s.planDeferred(sc.k, 0, sl) {
		t.Fatal("precondition: the source must reach the tap-extra gate undeferred")
	}
	if sc.s.windowCensusComplete(sc.k, &stackEntry{ctrl: 0}) {
		t.Fatal("windowCensusComplete proves a census that prices no unit for the pick")
	}
	o := sc.obj(sc.s.cardAt(sl), 0)
	complete, relaxable, gains := sc.s.madnessDeferProofClass(sc.k, 0, o)
	if complete || relaxable || !gains {
		t.Fatalf("madnessDeferProofClass = %v/%v/%v, want false/false/true", complete, relaxable, gains)
	}
}
