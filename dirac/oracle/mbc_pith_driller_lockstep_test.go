package oracle

import "testing"

// CR 107.4f, 119.4: Pith Driller's {4}{B/P} pip may be paid with {B} or two
// life, and a player at 1 life cannot pay two. The board's four untapped
// sources pay the {4} but not a fifth {B}, so neither face settles the cast;
// the kernel withheld it (decision 476, tail-18 focus game), while gorge's
// pip-free mana-half witness offered it. The recording pins the cast neither
// engine may offer.
func TestMBCPithDrillerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "pith_driller")
}
