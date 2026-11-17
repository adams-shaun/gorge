package oracle

import "testing"

// CR 305.7 and CR 605.1a: Obelisk of Undoing's {6},{T} is paid by Katilda's
// granted "{T}: Add one mana of any of this creature's colors" and a land under
// Contaminated Ground, whose tap fires an unvetted life-loss watcher gorge's
// planner defers (CR 603.2), beside four printed sources. gorge's one prefix
// search activates both gaps and offers the activation. The kernel's grant
// search listed the enchanted land as a candidate gap and then dropped it as one
// whose mana the activation cannot spend (gapSpendable read only the printed
// abilities the land lost, not the intrinsic mana of its new Swamp type, which
// carries no restriction), so the search ran on Katilda alone, found six mana
// short and never offered the activation (tail-55 pair 340 shape).
func TestMBCObeliskOfUndoingReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "obelisk_activation_land_set_gap_beside_grant")
}
