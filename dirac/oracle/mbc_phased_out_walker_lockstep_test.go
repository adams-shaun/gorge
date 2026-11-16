package oracle

import "testing"

// CR 702.26b: a phased-out permanent is treated as though it does not exist,
// so a planeswalker that has phased out cannot be attacked. Kaito Shizuki
// "phases out" at its controller's end step on the turn it entered, and stays
// out through the opponent's turn. gorge's attack offers listed every
// planeswalker on the defender's battlefield without reading PhasedOut, so
// Faerie Mastermind was offered the attack on the phased-out Kaito that the
// kernel (whose phased permanents sit off every mask) never offered (decision
// 265 of tail-67 pair 205; the same class held Rescuer Chwinga and Valiant
// Guard in the loop's tail-27 pool). The recording replays on a table built
// from its own decks, so a landing that reshuffles the tail pools leaves it be.
func TestMBCRescuerChwingaReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "attack_phased_out_planeswalker_unoffered")
}
