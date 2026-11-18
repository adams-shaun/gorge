package oracle

import "testing"

// CR 601.2f with CR 601.2c: Icefall Regent taxes an opponent's spell that
// targets it {2}, and the caster's Twisted Experiment ({1}{B}, two mana already
// floating) still has Turbulent Crater and a Darkwater Catacombs ({1},{T}: add
// {U}{B}) to pay the four mana, so the Regent is a legal target
// (tail-04-c000 pair 7, decision 458: gorge offered the Regent, dirac did not).
func TestMBCIcefallRegentReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "target_tax_filter_land_window")
	// tail-04-c000 pair 82, decision 322: Might of Alara ({G}) with a floating
	// {G}, an untapped Mountain and the same Catacombs pays {2}{G} through the
	// CR 601.2g window, so the Regent stays on its target menu here too.
	stdReproNamed(t, "raise_target_window_filter_land")
}
