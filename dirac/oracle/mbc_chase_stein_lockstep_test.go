package oracle

import "testing"

// CR 603.3c, 605.1a: the opponent's Relic Bind enchants the payer's Treasure,
// so tapping it for mana queues a Taps trigger whose mode its controller (the
// opponent) must choose. Gorge's prefix search answers only the payer's own
// asks, so no continuation of the Treasure's activation is offered Chase
// Stein, Runaway: its autopay surface leaves the cast out (unsupported,
// census_incomplete). The kernel offered the cast anyway (tail-08-c004,
// pair 109, decision 359).
func TestMBCChaseSteinRunawayReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "relic_bind_on_treasure_foreign_ask_ends_prefix")
}
