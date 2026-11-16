package oracle

import "testing"

// The first divergence of the tail-62 evidence pool, pair 13 (seed 5, autopay,
// gorge leading): Crimson Hellkite's "{X}, {T}: deals X damage ... Spend only
// red mana on X" activation asks its X, and the chooser holds five Mountains,
// a Teetering Peaks ({T}: Add {R}) and two Mana Prisms ({1}, {T}: Add one mana
// of any color) beside a floating pool of three {C}. Eight red mana is
// reachable (the pool pays both Mana Prism fees), so X = 8 is announceable
// (CR 601.2b); the kernel offered choose:x 0..8, gorge offered 0..7 -- one red
// source short of the CR 601.2g window's reach.
func TestMBCOptionsXColorRedCapReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "options_xcolor_red_cap")
}
