package oracle

import "testing"

// Incubation Druid's mana ability was offered as an activation by gorge but not
// by the kernel at p1's priority decision 393 (tail-55-c000 pair 67): the
// reflected-amount source's mana funding was priced differently by the two
// engines (CR 106.1, 605.1a).
func TestMBCIncubationDruidReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reflamt_incubation_druid_priority_offer")
}
