package oracle

import "testing"

// CR 602.2b with CR 605.3a: Absorb Vis's {1}{B} basic landcycling is payable
// from Three Tree City's {2},{T} "add X mana of any one color" arm (X is the
// creatures of the chosen type: one Changeling Mistwalker) when the arm's {2}
// is funded by an untapped Island and the opponent's Mana Flare doubles each
// tap: the arm's pick plus Flare's reflected extra is {B}{B}. The kernel offers
// the cycle (decision 171) while gorge's potential-mana census prices no
// paid, indeterminate-amount arm and withheld it.
func TestMBCAbsorbVisReprosLockstep(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"absorb_vis_cycle_ttc_arm_flare"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			stdReproNamed(t, name)
		})
	}
}
