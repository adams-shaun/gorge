package oracle

import "testing"

// std-msh's repros, on the same terms as repro_fixes_test.go's per-set tables.
func TestStdMshReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-msh", []stdRepro{
		{"target_attacking_lki_after_lethal_damage", false, false},       // CR 608.2h: the enrage rider still sees the attacker's last battlefield status after lethal damage moved it to the graveyard, so the extra combat phase is granted
		{"power_up_printed_witness_composes_cost_statics", false, false}, // CR 601.2f, 702.193b: Abomination's {5}{R/G}{R/G} Power-up beside Hulk is offered when five sources pay the printed cost less Hulk's {3}, the planner's witness
		{"hybrid_ability_plan_composes_cost_statics", false, false},      // CR 601.2f, 602.2b: Abomination's hybrid Power-up beside Hulk taps the lands of the plan for its printed cost less Hulk's {3}, not the manual window's
	})
}
