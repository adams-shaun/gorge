package oracle

import "testing"

// CR 602.2 with CR 605.4a: Loxodon Warhammer's Equip {3} (a sorcery-speed
// ability) beside Manabarbs, eight lands and six life. Every land tap queues a
// Manabarbs trigger, so gorge's prefix search taps a subset of the lands, then
// passes to resolve the queued triggers one clone at a time, and a payer who
// dies on the way ends the line. The search is capped at 160 clones; this
// board needs 217, so gorge finds no script and withholds the Equip. The
// kernel's Taps-deferred gap search counted the colour asks and the pass chain
// as nothing and offered it (decision 565, tail-34 pair 3).
func TestMBCLoxodonWarhammerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "tapsdefer_pass_chain_budget")
}
