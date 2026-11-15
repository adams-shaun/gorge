package oracle

import "testing"

// CR 106.6, 602.2a: mana that "can't be spent to cast a nonartifact spell"
// (a Powerstone) still pays an activated ability, and "Any player may
// activate this ability" (Casey Jones, Asphalt Hooligan's {4}: double its
// power) makes the opponent the activator, priced against the activator's
// own sources. gorge offers p0 the activation of p1's Casey Jones at {4}
// from a Mountain, a Swamp, a Skyclave Relic and a Powerstone after the
// first activation spent the four lands (decision 467 of tail-33 pair 1340);
// the kernel's any-player offer gate left the Powerstone out of its census
// and refused the legal option.
func TestMBCCaseyJonesAsphaltHooliganReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "anyplayer_activation_counts_restricted_sources")
}

// tail-33's recordings, globbed so the pool's recording is replayed like the
// std pools' (TestReproGlobMatchesTable).
func TestTail33ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosGlob(t, "tail-33")
}
