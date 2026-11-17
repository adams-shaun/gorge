package oracle

import "testing"

// TestMBCConfoundReprosLockstep: p1 has one untapped Eldrazi Spawn under Path of
// Annihilation and every land tapped, with Confound ({1}{U}) in hand against a
// spell that targets a creature. The Spawn's granted {T} (any colour) and its
// printed sacrifice ({C}) together pay {1}{U} (CR 605.1a, 602.2b), so the cast
// is offered.
func TestMBCConfoundReprosLockstep(t *testing.T) {
	stdReprosLockstep(t, "dr-mbc-babb", []stdRepro{
		{"confound_single_spawn_tap_then_sac", false, false},
	})
}
