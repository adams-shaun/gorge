package oracle

import "testing"

// CR 605.3a and 601.2g-h: Bosh, Iron Golem's "{3}{R}, Sacrifice an artifact:
// deal damage equal to the sacrificed artifact's mana value to any target" is
// payable by filtering an Island through Cascade Bluffs while Mage-Ring
// Network holds a storage counter. The kernel's filter-funding census read the
// battery as a lone one, whose announced X has no relaxed alternative, and
// funded nothing, so the activation was missing from the priority menu
// (decision 1130 of 1167, pool tail-55 pair 3).
func TestMBCBoshIronGolemReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "bosh_iron_golem_options")
}
