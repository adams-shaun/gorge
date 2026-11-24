package oracle

import "testing"

// CR 608.2c with CR 113.7a: Riptide Shapeshifter sacrifices itself as a cost,
// then its resolution chooses a creature type and reveals the library until a
// creature card of that type. The choice is made while the source is in the
// graveyard, so the reveal reads the answer just made, not the (empty)
// last-known "chosen type" of the battlefield object. A Skeletal Changeling is
// every creature type and ends the reveal on the battlefield (decision 572,
// tail-20 pair 12 seed 767051).
func TestMBCSkeletalChangelingReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "chosen_type_choice_survives_sacrificed_source")
}
