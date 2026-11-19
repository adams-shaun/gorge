package oracle

import "testing"

// CR 107.4c with CR 605.3b: Mystic Gate's "{W/U}, {T}: Add {W}{W}, {W}{U},
// or {U}{U}" is a hybrid-fee filter beside a plain "{T}: Add {C}". With four
// Plains and a Vivid Meadow untapped, activating Atalya, Samite Master's
// "{X}, {T}" (spend only white mana on X) asks X: gorge's castWindowPays
// funds the filter's hybrid fee with a unit of one of its two faces, so the
// largest X is the five white mana minus the one the fee takes plus the two
// the gate makes, 6. The kernel's X-ask bound (filtMaxX's walk) once charged
// the hybrid fee as generic mana, taking a green unit instead, and offered 7.
func TestMBCAtalyaSamiteMasterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "act_hybrid_fee_x_ask")
}
