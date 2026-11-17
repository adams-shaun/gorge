package oracle

import "testing"

// CR 310.11 with CR 712.8: a Siege whose back face is on the battlefield is
// the back-face permanent, not a battle, so exiling it (Excise the Imperfect on
// a transformed Invasion of Ixalan) defeats nothing and its owner is offered no
// transformed cast. gorge's exile feed read the face after the move had reset it
// to the front and posed defeat_cast_yes/no (decision 888 of tail-59-c001
// pair 5, where the game was found). The recording is pinned to the custom
// pool -- the table its own decks need -- so it cannot move with the tail
// buckets as further cards are lowered or refused.
func TestMBCExciseTheImperfectReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "transformed_battle_exiled_not_defeated")
}
