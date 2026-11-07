package oracle

import "testing"

// std-msh's repros, on the same terms as repro_fixes_test.go's per-set tables.
func TestStdMshReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-msh", []stdRepro{
		{"target_attacking_lki_after_lethal_damage", false, false}, // CR 608.2h: the enrage rider still sees the attacker's last battlefield status after lethal damage moved it to the graveyard, so the extra combat phase is granted
		{"sneak_paid_through_filter_land", true, false},            // CR 702.190a, 605.3a: Elektra sneaked for {1}{B}{B} with a Swamp and Surveillance Room's {1},{T} any-colour filter beside two Plains: the filter's prefix search funds the sneak cast
	})
}
