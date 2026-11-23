package oracle

import "testing"

// CR 616.1 with CR 615.7 in combat: D'Avenant Healer's depleting "prevent the
// next 1 damage" shield sits on Cho-Manno, Revolutionary beside the creature's
// own printed "Prevent all combat damage that would be dealt to CARDNAME."
// Gorge's scan collects the shield (an effect record) before the printed
// lines, so the order ask's options name the shield first and the printed
// line last; the kernel's combat walk collected the shields after the printed
// candidate, so its options came out printed-first.
func TestMBCTail40ReprosPrevnextPreventallCombatLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "prevnext_preventall_combat")
}
