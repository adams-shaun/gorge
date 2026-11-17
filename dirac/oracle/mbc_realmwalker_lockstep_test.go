package oracle

import "testing"

// CR 601.3 with CR 305.1: Realmwalker lets its controller cast creature spells
// of the chosen type from the top of their library. gorge offers the cast of
// the top card (mayplay mode) at priority in an autopay game while dirac's
// menu holds only pass and the land (decision 637 of tail-40-c058).
func TestMBCRealmwalkerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "realmwalker_top_library_chosen_type_cast")
}
