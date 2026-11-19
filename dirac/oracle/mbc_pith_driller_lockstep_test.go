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
	// CR 601.2f: Sphere of Resistance's {1} is part of Pith Driller's
	// pip-free mana half, and the autopay surface keeps a Phyrexian plain
	// cast only when gorge's planner prices that half (phyHalfPlanned). The
	// kernel priced the printed half ({4}) instead of the composed one ({5}):
	// the board's four untapped lands fund four mana, and a Rakdos Signet
	// reaches five only by paying its own {1} fee out of that mana, which the
	// planner's printed sources do not price. So the kernel offered cast(#19)
	// where gorge did not (decision 1105 of 1105, tail-18 pair 16).
	stdReproNamed(t, "phyrexian_half_composed_cost")
}
