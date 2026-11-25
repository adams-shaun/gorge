package oracle

import "testing"

// TestComboAnyZopandrelActivation: Zopandrel's {G/P}{G/P} activation announces one pip as
// {G} and the other as two life, and a Forest under Dawn's Reflection (a Combo Any tap
// extra) pays the {G}. gorge answers the extra's ask after the Forest's {G} reached the
// pool, so no pip is owed and the two mana are {W}{W}; the kernel measured the announced
// cost against the pool before the tap and answered {G}{W} (CR 107.4f, 601.2g).
func TestComboAnyZopandrelActivation(t *testing.T) {
	t.Parallel()
	stdReproNamedServed(t, "phyrexian_act_life_pip_combo_any_extra_answers_lowest")
}
