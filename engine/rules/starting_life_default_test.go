package rules

import "testing"

// TestCommanderStartingLifeDefaultsTo40 pins CR 903.7 at the engine rather
// than at a caller: a FormatCommander Config that leaves StartingLife at its
// zero value starts every seat at 40. Commanders are deliberately absent --
// the format alone selects the default, before any commander is seated.
func TestCommanderStartingLifeDefaultsTo40(t *testing.T) {
	cfg := fourSeatConfig(t, 60, 0)
	cfg.Format = FormatCommander
	e := New(cfg)
	if got := len(e.G.Players); got != 4 {
		t.Fatalf("precondition: %d seats, want 4", got)
	}
	for i := range e.G.Players {
		if got := e.G.Players[i].Life; got != 40 {
			t.Errorf("seat %d: life %d, want 40 (CR 903.7)", i, got)
		}
	}
}

// TestConstructedStartingLifeDefaultsTo20 pins the other half of the
// default: a non-Commander Config that leaves StartingLife zero still starts
// every seat at 20, byte-identical to the pre-Commander-default engine.
func TestConstructedStartingLifeDefaultsTo20(t *testing.T) {
	cfg := fourSeatConfig(t, 60, 0)
	e := New(cfg)
	if got := len(e.G.Players); got != 4 {
		t.Fatalf("precondition: %d seats, want 4", got)
	}
	for i := range e.G.Players {
		if got := e.G.Players[i].Life; got != 20 {
			t.Errorf("seat %d: life %d, want 20", i, got)
		}
	}
}
