package oracle

import "testing"

// CR 106.6 / 601.2g: Paralyze's "may pay {4}" is a non-spell payment, so
// Woodland Weavemaster's Elf-only mana is no payment for it. gorge's trigger
// cost window still lists that printed ability ahead of Citanul Hierophants'
// granted {T}: Add {G} and takes it, floating the unspendable {W} and leaving
// the cost unpaid; the kernel tapped the granted ability and paid.
func TestMBCPayFourRestrictedSourceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "restricted_source_beside_grant_floats_in_trigger_pay")
}
