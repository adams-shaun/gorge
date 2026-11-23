package oracle

import (
	"path/filepath"
	"testing"
)

// Goliath, Mass Manipulator's Power-up ({4}{G}: two +1/+1 counters, then draw a card per creature with
// power 4 or greater) is offered at priority when the player's lands pay it (CR 602.2, 118.3). At
// decision 315 of the tail-19 focus game gorge offered the activation and the kernel offered only
// pass. Later grant-search and census fixes closed the gap; this pins the recording.
func TestMBCGoliathMassManipulatorReprosLockstep(t *testing.T) {
	t.Parallel()
	// Load the recording's pool first: cardsettest skips on a tree with no corpus, where synthReg
	// (inside stdReproNamed) fails in a fresh export with no .git.
	r, err := ReadRepro(filepath.Join("testdata", "repro", "goliath_mass_manipulator_powerup_offer.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	globSet(t, r.Pool, r)
	stdReproNamed(t, "goliath_mass_manipulator_powerup_offer")
}
