package oracle

import (
	"testing"
)

// A manland's animate ability grants the card a subtype only an effect
// carries (CR 613.1f; Blinkmoth Nexus's Types$ Creature,Artifact,Blinkmoth,
// never on a Types line), and the land's pump targets "Blinkmoth creature".
// The kernel offered the second Blinkmoth Nexus activation -- the pump, whose
// target census finds the animated Nexus -- while gorge's filter vocabulary
// did not know the granted word, so its offer-time census found zero targets
// and the option vanished (decision 674, pair 15 of the tail-33 focus sweep;
// class `options options`, lead gorge).
func TestMBCBlinkmothReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "blinkmoth")
}
