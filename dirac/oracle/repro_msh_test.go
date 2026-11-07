package oracle

import "testing"

// std-msh's repros, on the same terms as repro_fixes_test.go's per-set tables.
func TestStdMshReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-msh", []stdRepro{
		{"target_attacking_lki_after_lethal_damage", false, false}, // CR 608.2h: the enrage rider still sees the attacker's last battlefield status after lethal damage moved it to the graveyard, so the extra combat phase is granted
		{"powerup_static_reduction_offer", true, false},            // CR 601.2f, 602.2b, 702.193a: Hulk's {3} static lowers Viv Vision's power-up enough for six sources to pay it, though neither the pool nor the printed {7} is affordable
	})
}
