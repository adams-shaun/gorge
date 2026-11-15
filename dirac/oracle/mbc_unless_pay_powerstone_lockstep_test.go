package oracle

import "testing"

// Spell Contortion ("counter target spell unless its controller pays {2}")
// resolving against its own controller's spell: at decision 383 of dr tail-41
// pair 12 gorge offered the pay answer (mode 1) beside decline and the kernel
// offered only decline. The recording replays on its own custom table
// (pool "custom"): the tail-41 bucket's membership moves as the land lowers
// more cards, and a synthetic-pool recording dies the day a deck card leaves
// it (High-Rise Sawjack did).
func TestMBCSpellContortionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_pay_powerstone_mana")
}
