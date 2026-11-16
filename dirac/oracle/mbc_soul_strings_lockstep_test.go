package oracle

import "testing"

// CR 106.6, 118.12: Soul Strings's "unless any player pays {X}" is a cost inside
// an effect, not a spell cast, so a Powerstone's {C} (which can't be spent to
// cast a nonartifact spell) pays it. With X = 1 and a Powerstone token the
// opponent's only untapped mana source, gorge offered the opponent the pay mode
// and dirac offered decline only (decision 572, options modes for p1): the
// unless ask's affordability counted no restricted source.
func TestMBCSoulStringsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_cost_paid_with_a_powerstone")
}
