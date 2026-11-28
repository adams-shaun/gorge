package state

import "testing"

// TestRestartClearsTurnRotation: the restarted game's turn rotation starts
// over with the rest of the turn structure.
func TestRestartClearsTurnRotation(t *testing.T) {
	g := NewGame([]string{"a", "b"})
	g.Rotation = TurnRotation{Away: true, Seat: 1, ExtraNext: true}

	g.Restart(20, nil)

	if g.Rotation != (TurnRotation{}) {
		t.Errorf("rotation = %+v, want the zero value", g.Rotation)
	}
}
