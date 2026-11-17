package oracle

import "testing"

// Havoc Devils ({2}{R}{R}) with Mountain, Grove of the Burnwillows, Otawara and
// Darkwater Catacombs ({1}, {T}: Add {U}{B}) untapped: Otawara's {U} pays the
// Catacombs' fee, the two Catacombs mana pay the generic, Mountain and the
// Grove the {R}{R}. gorge offers the cast; dirac once did not (decision 281,
// tail-04 pair 5): the Grove's rider gap kept the filter search off the board.
func TestMBCHavocDevilsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_beside_rider_gap")
}
