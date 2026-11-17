package oracle

import "testing"

// Temur Charm mode 3 registers a resolution-generated continuous effect
// (SVar:KWPump Mode$ Continuous | Affected$ Creature.powerLE3 |
// AddHiddenKeyword$ CARDNAME can't block.). CR 613 orders layer 7 after the
// layer-6 grant, so the grant's Affected$ gate must read the object's
// layer-7 answer, not its printed face: the recording's Mesa Cavalier
// (printed 2/1) carries a Giant Growth (+3/+3) when the charm resolves and
// is power 5, so it must keep blocking. Before the bind it matched on the
// printed 2, the whole block build emptied, and gorge's silent skip asked a
// [pass]-only priority at Step DeclareBlockers while the kernel asked the
// declare-blockers decision offering block(#61{Mesa Cavalier},...) --
// lockstep pair 1232 of the evidence loop at land 3ce538a59c, divergence at
// decision 634, `decision kind ref priority p0, kernel blockers p1`.
//
// The recording is pool custom (a table built from its own decks): the
// ticket's game dealt from the operator corpus's tail-62 as the lowered set
// stood then, and a tail-NN bucket moves whenever the lowered set grows, so
// a standard-pool replay would fail with a card "not in the pool" before it
// played a decision.
func TestMBCTemurCharmPowerleGrantReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "temur_charm_powerle_grant")
}
