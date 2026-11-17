package effects

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// Brago, King Eternal's trigger is seeded with its own capture (Brago dealt
// the damage), then remembers Vow of Flight and Brago as it exiles them. The
// source's written list holds Brago, so the capture is kept -- but only its
// written occurrence: the seeded one leading Remembered would return Brago
// before the Aura (CR 603.6a walks the cards in the order they were exiled).
func TestSelectRememberedDropsTheSeededOccurrenceOfAWrittenCapture(t *testing.T) {
	t.Parallel()
	brago, vow := state.Target{Obj: 44}, state.Target{Obj: 78}
	remembered := []state.Target{brago, vow, brago}
	captured := []state.Target{brago}
	persistent := []state.Target{vow, brago}
	got := selectRemembered(remembered, captured, nil, persistent, false)
	if want := []state.Target{vow, brago}; !slices.Equal(got, want) {
		t.Fatalf("selectRemembered = %v, want the written order %v", got, want)
	}
	// A capture the body never remembered again stays where it was: the
	// written list holds it from an earlier resolution, so the one occurrence
	// is its only one.
	got = selectRemembered([]state.Target{brago, vow}, captured, nil, persistent, false)
	if want := []state.Target{brago, vow}; !slices.Equal(got, want) {
		t.Fatalf("selectRemembered = %v, want the capture kept as %v", got, want)
	}
}
