package oracle

import "testing"

// CR 603.4 with CR 309.7: Ravenloft Adventurer's Attacks trigger carries the
// intervening if "if you've completed a dungeon" (the SVar gate reads the
// controller's completed-dungeon count, Player.CompletedDungeons) and loses
// the defending player life for each card they own in exile with a hit
// counter on it (a TriggeredDefendingPlayer$ ValidExile count). Gorge's count
// evaluator models neither head, so the gate fails closed and the trigger
// never fires, while the kernel queues it once the count reads one: two
// attacking Ravenloft Adventurers part the pool at the trigger_order decision
// (decision 478 of the recorded game; pool tail-32, lead gorge).
func TestMBCRavenloftAdventurerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "completed_dungeon_gate_unread")
}
