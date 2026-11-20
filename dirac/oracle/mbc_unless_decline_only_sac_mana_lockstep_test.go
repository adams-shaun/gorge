package oracle

import (
	"testing"
)

// Vaporous Djinn's "At the beginning of your upkeep, Vaporous Djinn phases
// out unless you pay {U}{U}" with the payer's only untapped mana route an
// untapped Island and an untapped Chromatic Star ({T}, Sacrifice: Add one
// mana of any color) beside two untapped Plains: gorge offered the election
// decline only (decision 287 of pool tail-62, surface autopay), because its
// unless census prices no sac-mana route; the kernel priced the Island +
// Chromatic Star plan and offered pay and decline. CR 605.3a/b: the payer may
// activate mana abilities to meet a payment, sacrifice-mana ones included.
func TestMBCLeoSGuidanceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_decline_only_sac_mana")
}
