package oracle

import "testing"

// CR 118.9d with CR 601.2f: a card cast without paying its mana cost still
// pays the cost statics' increases, and that increase may be paid with a
// mana ability that taps another permanent. Urza, Lord High Artificer's
// "Tap an untapped artifact you control: Add {U}" pays Thorn of Amethyst's
// {1} on the free may-play cast of Battle of Wits (tail-64 pair 22, decision
// 702: gorge offered cast(#40,-,3), the kernel only pass and land).
func TestMBCBattleOfWitsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mayplay_free_raise_tap_artifact")
}
