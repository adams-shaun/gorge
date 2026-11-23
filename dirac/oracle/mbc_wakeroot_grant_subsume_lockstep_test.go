package oracle

import "testing"

// Wakeroot Elemental's {G}{G}{G}{G}{G} activation beside Dune Chanter ("lands
// you control have {T}: Add one mana of any color"): the floating pool is
// empty and the printed sources alone make two green, so the payment is the
// granted any-colour taps. gorge's offer pipeline lists the activation where
// its script search finds the prefix (its subsume run, 143 nodes at turn 26)
// and withholds it where that search exhausts its 160-node budget (turn 20),
// while the kernel's grant search drops the play at both points. The pair
// parts at decision 610, gorge [pass, activate(#43 Dune Chanter),
// activate(#78 Wakeroot Elemental)], kernel [pass, activate(#43 Dune Chanter)]
// (class `options options`, pool tail-71, pair 6, seed 17421456951445507279,
// surface full).
func TestMBCWakerootElementalReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "wakeroot_grant_subsume")
}
