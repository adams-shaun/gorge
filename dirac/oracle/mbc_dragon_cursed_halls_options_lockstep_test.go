package oracle

import "testing"

// Dragon-Cursed Halls' activation decision parts gorge and the kernel on the
// options offered: gorge lists activate(Dragon-Cursed Halls) beside pass where
// the kernel offers only pass (class `options options`, tail-71-c000 pair 3,
// decision 190, seed 272106, surface autopay).
func TestMBCDragonCursedHallsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "dragon_cursed_halls_options")
	// tail-71-c001 pair 2, decision 40, seed 569081: the same card, the same class.
	stdReproNamedServed(t, "dragon_cursed_halls_activation")
}
