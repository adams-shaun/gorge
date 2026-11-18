package oracle

import "testing"

// CR 601.2b/h: Disruption Protocol's either-or additional cost (tap an untapped
// artifact you control, or pay {1}) with {U}{U} to pay, and a controller whose
// only mana sources are an Island and the artifact Izzet Cluestone. Tapping the
// Cluestone as the cost leaves one mana for {U}{U}; paying {1} needs three.
// Neither part is payable, so the cast isn't offered.
func TestMBCDisruptionProtocolReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "alt_add_tap_part_needs_its_artifact_off_the_mana")
}
