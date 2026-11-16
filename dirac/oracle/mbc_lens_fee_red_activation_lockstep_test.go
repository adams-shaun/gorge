package oracle

import "testing"

// A land whose activation ({4}{R}, {T}, sacrifice it: Memorial to War's destroy
// target land) is payable only through a filter's fee ({1}, {T}: any colour,
// Prismatic Lens supplying the {R}) beside sacrifice mana sources (Eldrazi
// Scions): gorge offers the activation; the kernel withheld it (pool tail-56
// pair 181 of the evidence loop, replayed at pair 609 of a Memorial-focused
// deal after the pool shifted).
func TestMBCMemorialToWarReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "lens_fee_red_activation_sac_self")
}
