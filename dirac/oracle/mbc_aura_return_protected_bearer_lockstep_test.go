package oracle

import "testing"

// CR 303.4f with CR 702.16c: Nomad Mythmaker returns Heliod's Punishment (a
// white Aura) from a graveyard attached to a creature its controller chooses,
// and "a legal object" excludes a creature with protection from white. The
// controller's token copy of Empty-Shrine Kannushi has protection from the
// colours of the permanents they control, Nomad Mythmaker among them, so it
// is no bearer. gorge once offered it in the choose ask beside the two legal
// creatures (decision 499 of tail-06-c000 pair 2) and the kernel did not.
func TestMBCNomadMythmakerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "aura_return_skips_protected_bearer")
}
