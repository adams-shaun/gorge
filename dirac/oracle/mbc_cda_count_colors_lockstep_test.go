package oracle

import "testing"

// An-Havva Constable's toughness is 1 plus the number of green creatures on
// the battlefield (a characteristic-defining layer-7a ability), and Hissing
// Quagmire's {1}{B}{G} activation makes it "a 2/2 black and green Elemental
// creature" for the turn (a layer-5 colour set, CR 613.1e). CR 613.3 orders
// layer 5 before layer 7a, so the animated Quagmire is green when the CDA's
// count reads the board: gorge's count once read each candidate's PRINTED
// colours (the count fold binds no layer-5 colour table mid-derivation), so it
// held the Constable at 3 toughness while the kernel read 4. The recording
// diverged at decision 198: projection Cards[1].Toughness, gorge leading,
// autopay surface. It replays on a custom pool (a table built from its own
// decks), not tail-38: that pool's membership moves each time the evidence
// loop credits a card, and a recording must not depend on it.
func TestMBCAnHavvaConstableReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cda_count_sees_layer5_colors")
}
