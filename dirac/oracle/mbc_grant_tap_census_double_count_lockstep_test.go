package oracle

import "testing"

// CR 605.3b: a source's single tap cannot activate two mana abilities, so a
// granted "{T}: Add one mana of any color" (Urban Utopia on an Island) and the
// land's own {T} ability price one mana between them. The per-tap census that
// bounds an X-bound-target sweep (xBoundPayable) recorded the grant as a unit
// of its own beside the native production, so {3}{U}{U} read payable off a
// four-mana board and Entrancing Melody was offered at X=3 with no target any
// payable X admits (decision 415 of tail-44-c000 pair 5).
func TestMBCEntrancingMelodyReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "grant_tap_census_double_count")
}
