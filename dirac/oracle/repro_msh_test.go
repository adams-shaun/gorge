package oracle

import "testing"

// std-msh's repros, on the same terms as repro_fixes_test.go's per-set tables.
func TestStdMshReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-msh", []stdRepro{
		{"target_attacking_lki_after_lethal_damage", false, false}, // CR 608.2h: the enrage rider still sees the attacker's last battlefield status after lethal damage moved it to the graveyard, so the extra combat phase is granted
		{"powerup_witness_composes_cost_statics", true, false},     // CR 601.2f, 702.193b: gorge's autopay witness for a power-up prices the printed cost after the cost statics (Hulk, Gamma Goliath's {3}), so Quicksilver's {4}{R} is offered from four sources
		{"powerup_window_plan_composes_cost_statics", true, false}, // CR 601.2f, 702.193b: the activation's plan is armed by that same statics-composed witness, so the reduced {1} taps the Forest the plan picks, not the first source in the window
	})
}
