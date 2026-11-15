package oracle

import "testing"

// CR 118.3 with CR 605.3b: Well of Lost Dreams' "may pay {X}" (X = 1) while the
// payer's only untapped source is Gemstone Mine ({T}, remove a mining counter).
// gorge's unless window (unlessWindowUnits) lists only bare-tap mana abilities,
// so it poses the decline-only ask; the kernel priced the depletion rider in
// the window and also offered the pay (tail-62 pair 330, decision 437). The
// repro plays pool "custom": a table built from its own decks, because the
// tail-NN pools repartition whenever a card is lowered.
func TestMBCTail62ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_window_skips_counter_cost_mana")
}
