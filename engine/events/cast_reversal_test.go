package events

import "testing"

// A backward walk meets a reversal before the cast it undid, and a reversal
// pairs with the same player's cast only.
func TestCastReversalsPairsEachReversalWithItsCast(t *testing.T) {
	log := []Event{
		{Kind: PutOnStack, Player: 0},        // 0: cast, stays
		{Kind: PutOnStack, Player: 1},        // 1: cast, reversed
		{Kind: SpellCastReversed, Player: 1}, // 2
		{Kind: PutOnStack, Player: 0},        // 3: cast, reversed
		{Kind: SpellCastReversed, Player: 0}, // 4
		{Kind: PutOnStack, Player: 1},        // 5: cast, stays
	}
	var r CastReversals
	var undone []int
	for i := len(log) - 1; i >= 0; i-- {
		if r.Undone(&log[i]) {
			undone = append(undone, i)
		}
	}
	if len(undone) != 2 || undone[0] != 3 || undone[1] != 1 {
		t.Fatalf("undone casts = %v, want [3 1]", undone)
	}
}
