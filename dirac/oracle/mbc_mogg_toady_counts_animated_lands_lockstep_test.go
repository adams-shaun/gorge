package oracle

import "testing"

// CR 508.1c with CR 613.1d: Mogg Toady "can't attack unless you control more
// creatures than defending player", and a land a type-changing effect made a
// creature counts as one. p0 controls Llanowar Elite and two Swamps that p1's
// Kormus Bell makes 1/1 creatures, three creatures to p1's two (Mogg Toady and
// Rakdos Pit Dragon), so Mogg Toady can't attack. gorge counted the defender's
// creatures off their printed type lines (one) and offered the attack
// (decision 479 of tail-52 pair 12 at land 81a6ccdb4b); the kernel's count
// reads the derived types and refused it. The recording plays on the table its
// own decks need (pool custom): a tail pool's deal moves as evidence is
// credited.
func TestMBCMoggToadyReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mogg_toady_counts_animated_lands")
}
