package oracle

import "testing"

// CR 601.2h / 602.2b: Go-Shintai of Life's Origin's {W}{U}{B}{R}{G} activation
// needs five distinct colours of mana. Its controller's untapped lands make B, R
// (twice), G and only one more colour: School of the Unseen makes a coloured
// mana solely through its {2},{T} ability, and Needleverge Pathway is the red
// face. The kernel offered the activation anyway (decision 621 of 621, pool
// tail-42-c000 pair 37).
func TestMBCGoShintaiOfLifeSOriginReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "five_color_activation_without_two_any_sources")
}
