package oracle

import "testing"

// CR 601.2g with CR 702.103a: gorge's planner prices Nyxborn Wolf's bestowed
// cast ({4}{G}) with a witness whose mana activations sacrifice the Eldrazi
// Scion -- the bestowed spell's only enchant-creature target. Executing that
// witness leaves no target, so the play is "unsupported witness_unreachable"
// and gorge offers no bestowed cast; the kernel's avail census counted the
// Scion's sacrifice mana and offered it (tail-27-c000 pair 32, decision 323).
func TestMBCBestowSacWitnessEatsTargetReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "bestow_sac_witness_eats_target")
}
