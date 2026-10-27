package effects

import "testing"

func TestUntilYourNextUpkeepDurationPredicate(t *testing.T) {
	t.Parallel()
	for _, dur := range []string{"UntilYourNextUpkeep", " untilyournextupkeep "} {
		if !IsUntilYourNextUpkeepDuration(dur) {
			t.Errorf("IsUntilYourNextUpkeepDuration(%q) = false, want true", dur)
		}
	}
	// UntilTheEndOfYourNextUpkeep (Halfdane) ends at the END of the upkeep: a
	// different boundary that must not match.
	for _, dur := range []string{"", "UntilTheEndOfYourNextUpkeep", "UntilYourNextTurn", "UntilTheEndOfYourNextTurn", "UntilTheEndOfYourNextUntap"} {
		if IsUntilYourNextUpkeepDuration(dur) {
			t.Errorf("IsUntilYourNextUpkeepDuration(%q) = true, want false", dur)
		}
	}
	// The new code must not leak into the next-turn predicate, whose
	// UntilTurn boundary would move the bug instead of fixing it.
	if IsNextTurnDuration("UntilYourNextUpkeep") {
		t.Error("IsNextTurnDuration(UntilYourNextUpkeep) = true, want false")
	}
}
