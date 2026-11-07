package oracle

import "testing"

// std-msh's repros, on the same terms as repro_fixes_test.go's per-set tables.
func TestStdMshReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-msh", []stdRepro{
		{"target_attacking_lki_after_lethal_damage", false, false},        // CR 608.2h: the enrage rider still sees the attacker's last battlefield status after lethal damage moved it to the graveyard, so the extra combat phase is granted
		{"powerup_cost_static_planner_prices_reduced_print", true, false}, // CR 601.2f, 702.193: Hulk, Gamma Goliath's {3} Power-up reduction composes into the autopay planner's printed cost, so Pet Avengers' {6}{G} is offered at {3}{G} from the sources
	})
}
