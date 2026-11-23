package oracle

import "testing"

// Hurloon Battle Hymn's kicker ({W}): gorge and dirac part at the priority
// decision where dirac offers the kicked cast and gorge does not.
func TestMBCHurloonBattleHymnReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "hurloon_battle_hymn_kicker_offer")
}
