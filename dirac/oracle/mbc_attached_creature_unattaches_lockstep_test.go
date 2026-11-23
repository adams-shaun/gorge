package oracle

import "testing"

// CR 704.5p: a creature attached to an object or player becomes unattached and
// stays on the battlefield. Mercurial Transformation's Octopus animate (mode 1)
// made Robe of Mirrors, an Aura attached to Leaping Master, a blue 4/4 Octopus;
// the kernel kept the attachment where gorge's CR 704.5p state-based action
// unattached it (projection Cards[28].AttachedTo at decision 919,
// pool tail-46 pair 29).
func TestMBCRobeOfMirrorsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "attached_creature_unattaches")
}
