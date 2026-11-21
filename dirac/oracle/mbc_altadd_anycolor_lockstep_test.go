package oracle

import "testing"

// CR 601.2b, 601.2f, 609.4b: a card cast from exile under an any-colour
// may-play rider (Stolen Strategy) whose either-or additional cost has a part
// with no mana (Morkrut Behemoth: sacrifice a creature or pay {1}{B}) is
// offered when the board's lands pay its {4}{B} with any colour and a creature
// can be sacrificed. Gorge offered the cast; the kernel dropped it.
func TestMBCMorkrutBehemothReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "may_play_any_color_altadd_sac_part")
}
