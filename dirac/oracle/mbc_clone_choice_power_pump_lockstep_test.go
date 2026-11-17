package oracle

import "testing"

// CR 614.12 with CR 707.9: Deceptive Frostkite's "enter as a copy of a
// creature you control with power 4 or greater" reads current power, so a
// Topan Ascetic pumped to 4/4 until end of turn is a legal choice.
func TestMBCTopanAsceticReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "clone_choice_power_pump")
}
