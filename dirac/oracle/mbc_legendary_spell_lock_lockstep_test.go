package oracle

import "testing"

// CR 205.4e: a legendary instant or sorcery is cast only while its caster
// controls a legendary creature or planeswalker. The kernel read that lock
// (the spell ability's cast restriction, prepSpellCond) only when the table
// held no CantBeCast static, because the two probes were joined with ||; a
// pool holding one left Jaya's Immolating Inferno offered to a player whose
// opponent alone controlled the legend (decision 558 of tail-58-c024).
func TestMBCJayaSImmolatingInfernoReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "legendary_spell_lock_with_cantcast_static")
}
