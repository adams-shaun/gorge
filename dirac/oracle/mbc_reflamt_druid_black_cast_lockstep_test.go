package oracle

import "testing"

// Defiant Salvager's cast ({2}{B}) was offered by the kernel but not by gorge at
// p0's priority decision 1833 (tail-55-c000 pair 14, seed 609160): the only
// untapped black source is Incubation Druid, whose reflected production reads
// the controller's tapped Swamps (CR 106.1, 605.1a).
func TestMBCDefiantSalvagerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reflamt_druid_black_cast_offer")
}
