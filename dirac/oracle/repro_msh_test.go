package oracle

import "testing"

// std-msh's repros, on the same terms as repro_fixes_test.go's per-set tables.
func TestStdMshReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-msh", []stdRepro{
		{"target_attacking_lki_after_lethal_damage", false, false}, // CR 608.2h: the enrage rider still sees the attacker's last battlefield status after lethal damage moved it to the graveyard, so the extra combat phase is granted
		{"filter_combo_allocates_needed_colours", false, false},    // CR 601.2g-h, 605.3b: Baxter Building's {4},{T} combination filter pays Truck Toss's {2}{R}{R} once its allocation names the red the search needs
	})
}
