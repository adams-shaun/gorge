package kgen

import "testing"

// TestExileOwnerCountsHoldAtAnySeats: the cards whose exile count names a
// player (Ravenloft Adventurer's TriggeredDefendingPlayer$ValidExile, Etrata,
// the Silencer's ControlledBy TriggeredTarget) read ExCountExile N 2 over that
// player's seat, so neither lowers two-seat only.
func TestExileOwnerCountsHoldAtAnySeats(t *testing.T) {
	for _, name := range []string{"Ravenloft Adventurer", "Etrata, the Silencer"} {
		cs := lowerNamed(t, []string{name})
		if len(cs) != 1 {
			t.Fatalf("%s: got %d cards", name, len(cs))
		}
		for _, r := range cs[0].Refusals {
			t.Errorf("%s refusal: %q", name, r.Reason)
		}
		if cs[0].TwoSeat {
			t.Errorf("%s lowered two-seat only: %v", name, cs[0].TwoSeatWhy)
		}
	}
}
