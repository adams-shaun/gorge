package oracle

import "testing"

// std-msh's repros, on the same terms as repro_fixes_test.go's per-set tables.
func TestStdMshReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-msh", []stdRepro{
		{"target_attacking_lki_after_lethal_damage", false, false}, // CR 608.2h: the enrage rider still sees the attacker's last battlefield status after lethal damage moved it to the graveyard, so the extra combat phase is granted
		{"power_up_planner_prices_cost_statics", true, false},      // CR 601.2f, 602.2b, 702.193a: Hulk, Gamma Goliath's {3} off She-Hulk, Jade Defender's {4}{G}{G} Power-up is in the planner's printed cost, so five lands with three Forests offer it
	})
}
