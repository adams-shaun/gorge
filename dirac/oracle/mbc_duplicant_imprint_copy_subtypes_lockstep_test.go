package oracle

import "testing"

// The `projection Cards[i].Chars.Subtypes` class, Duplicant's imprint copy:
// gorge and the dirac kernel disagree on the battlefield copy's creature
// subtypes after the {2} activation ("becomes a copy of the exiled card,
// except it's an artifact in addition to its other types"). Recorded from the
// tail-75 evidence loop (pair 4, seed 116832, decision 225, implicating
// Duplicant): the imprint exiled the graveyard's Blind Creeper
// (Creature Zombie Beast) and the next decision's projection reads
// ref=Zombie,Beast kernel=one bit.
func TestMBCDuplicantReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "duplicant_imprint_copy_subtypes")
}
