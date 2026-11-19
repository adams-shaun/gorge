package oracle

import "testing"

// Pact of the Titan's upkeep "pay {4}{R}" election while the only red source
// is an untapped Fellwar Stone reflecting an opponent's (tapped) Mountain.
// The modes decision offered by gorge and the kernel must list the same
// options (found with Bloody Betrayal in the deal: tail-41-c000, pair 115).
func TestMBCBloodyBetrayalReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_pay_reflected_red_mana")
}
