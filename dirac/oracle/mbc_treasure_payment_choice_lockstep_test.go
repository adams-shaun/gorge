package oracle

import "testing"

// TestMBCTreasurePaymentChoiceReprosLockstep replays the tail-13 lockstep game
// whose payment plans, paying the same cast of Jocasta, Automaton Avenger
// ({3}) from Tainted Field, a Swamp and one of three Treasures, chose
// DIFFERENT Treasures to sacrifice (both legal, CR 601.2b): one Treasure
// carried gorge's Relic Putrescence ("whenever enchanted artifact becomes
// tapped, its controller gets a poison counter"), gorge's plan sacrificed the
// enchanted one and Relic Putrescence went to its owner's graveyard (CR
// 704.5f), while the kernel's plan deferred the enchanted Treasure's tap (its
// poison Taps watcher read Defer) and sacrificed another, leaving the Aura on
// the battlefield. The poison row is a post-payment consequence on both
// engines -- the trigger resolves after the payment that caused it (CR
// 405.6c/603.2) and its only effect is a poison counter on a player the
// payment cannot move -- so the enchanted Treasure is an ordinary planner
// unit and both plans sacrifice the battlefield-first Treasure (the enchanted
// one), gorge pay/plancost.go's search order and the kernel plan's key 8.
func TestMBCTreasurePaymentChoiceReprosLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	stdReproNamed(t, "treasure_payment_choice")
}
