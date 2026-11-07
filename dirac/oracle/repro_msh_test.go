package oracle

import "testing"

// std-msh's repros, on the same terms as repro_fixes_test.go's per-set tables.
func TestStdMshReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-msh", []stdRepro{
		{"target_attacking_lki_after_lethal_damage", false, false}, // CR 608.2h: the enrage rider still sees the attacker's last battlefield status after lethal damage moved it to the graveyard, so the extra combat phase is granted
		{"combo_filter_allocation_needs_first", false, false},      // CR 601.2g, 605.3a: Baxter Building's {4}, {T} makes four mana in any combination; the {G} cast orders the allocation answers that name its pip first, as gorge's scriptOrderAnswers does, so the search finds the script within its 160 nodes (dirac tried all four W first)
	})
}
