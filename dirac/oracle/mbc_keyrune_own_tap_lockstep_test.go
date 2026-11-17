package oracle

import "testing"

// CR 602.2b and 118.3: Izzet Keyrune's "{R}{U}: becomes a creature" is paid
// from other sources; a prefix search must not tap the Keyrune's own {T} mana
// ability for it, so gorge and dirac agree whether the activation is offered
// (decision 862 of tail-19-c000 pair 6 before "keep a prefix search from tapping
// the play's own {T} source" landed: dirac offered an activation gorge did not).
func TestMBCIzzetKeyruneReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "keyrune_own_tap_source_not_prefix_paid")
}
