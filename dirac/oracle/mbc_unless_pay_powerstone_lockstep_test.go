package oracle

import "testing"

// Spell Contortion ("counter target spell unless its controller pays {2}")
// resolving against its own controller's spell: at decision 383 of dr tail-41
// pair 12 gorge offered the pay answer (mode 1) beside decline and the kernel
// offered only decline.
func TestMBCSpellContortionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_pay_powerstone_mana")
}
