package effects

import "testing"

// TestEladamriChosenCardMovesFromEitherOrigin is the no-ask leg: Eladamri's real
// ChooseCard sub moves the already-chosen card from either origin zone with
// no second, unrestricted choice.
func TestEladamriChosenCardMovesFromEitherOrigin(t *testing.T) {
	assertEladamriChosenCardMovesFromEitherOrigin(t)
}
