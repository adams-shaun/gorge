package oracle

import "testing"

// CR 106.6: mana from a restricted source keeps its spend restriction when it
// is left floating. Dust Elemental's cast ({2}{W}{W}, flash) is the option
// only one engine offered at decision 533: Vodalian Arcanist's {C} (spendable
// only on an instant or sorcery spell) was tapped to cast Pull from Tomorrow
// and left in the pool when the announced X was 0, and the kernel then spent
// it as an ordinary {C} on the creature spell. tail-34 pair 15, seed 591790.
func TestMBCDustElementalReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "restricted_leftover_floats_plain")
}
