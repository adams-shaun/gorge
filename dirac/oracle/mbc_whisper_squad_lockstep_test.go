package oracle

import "testing"

// Whisper Squad's {1}{B} search activation was offered by the kernel but not
// by gorge at p0's priority decision 539 (tail-55-c000 pair 3): the only {B}
// left was an Incubation Druid's reflected mana off tapped Swamps. gorge's
// PotentialMana bound prices a reflected Amount$ source in the colourless slot
// only, so its potential walk never lists a play whose coloured pip only the
// druid funds (CR 106.1, 106.7).
func TestMBCWhisperSquadReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reflamt_druid_colour_beyond_bound_activation")
}
