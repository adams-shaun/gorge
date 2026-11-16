package oracle

import "testing"

// Confront the Assault ("Cast this spell only if a creature is attacking you")
// is withheld by both engines while nothing attacks its caster. The kernel
// once offered it anyway in a table that also held a StatCantBeCast static
// (Teferi, Time Raveler): prepareGen's `tableHasStatic(...) || prepSpellCond(...)`
// short-circuited, so anySpellCond stayed false and the spell's cast gate was
// never read (decision 324 of tail-22 pair 4, p0's draw step, no attacker).
func TestMBCConfrontTheAssaultReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "confront_the_assault_attacking_you")
}
