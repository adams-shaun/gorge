package oracle

import "testing"

// CR 201.4: a tail-61 game (pair 86, seed 777595, autopay) parted at decision
// 1034, Alhammarret, High Arbiter's name choice over the five nonland cards
// the opponent revealed. gorge canonicalises a name option with the count of
// earlier same-hash names among the options it offers; the kernel side counted
// over the whole matchup name table, so Wall of Limbs carried ordinal 1 there
// and 0 here. Both sides now count over the offered options.
func TestMBCBallroomReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamedServed(t, "ballroom_choose_name_offer")
}
