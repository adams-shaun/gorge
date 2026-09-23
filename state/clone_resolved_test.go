package state

import "testing"

// TestCloneOwnsResolvedThisTurn pins Game.Clone's contract for the
// Count$ResolvedThisTurn tally: events.Apply increments it IN PLACE, so a
// clone must own its own map, exactly like ExtraTurns and the other
// in-place-mutated maps/slices Clone walks by hand. Sharing the backing map
// would let a clone's resolution corrupt the original's per-ability count.
func TestCloneOwnsResolvedThisTurn(t *testing.T) {
	const key = "7|DB:LoseLife"
	g := NewGame([]string{"a", "b"})
	g.ResolvedThisTurn = map[string]int32{key: 3}
	c := g.Clone()
	if got := c.ResolvedThisTurn[key]; got != 3 {
		t.Fatalf("clone tally = %d, want 3", got)
	}
	// Precondition: the write below only proves separation if the two maps
	// are distinct; incrementing the clone must not touch the original.
	c.ResolvedThisTurn[key]++
	if got := g.ResolvedThisTurn[key]; got != 3 {
		t.Fatalf("writing the clone changed the original tally to %d, want 3 -- Clone shares the map", got)
	}
	if got := c.ResolvedThisTurn[key]; got != 4 {
		t.Fatalf("clone tally after ++ = %d, want 4", got)
	}
	// A nil tally clones to nil (not an empty non-nil map), the same shape
	// the other optional maps keep.
	if ec := NewGame([]string{"a", "b"}).Clone(); ec.ResolvedThisTurn != nil {
		t.Fatalf("clone of a nil tally = %v, want nil", ec.ResolvedThisTurn)
	}
}
