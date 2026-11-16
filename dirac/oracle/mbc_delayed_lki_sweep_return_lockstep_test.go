package oracle

import (
	"testing"
)

// CR 603.7: Planar Guide's "{3}{W}, Exile Planar Guide: Exile all creatures.
// At the beginning of the next end step, return those cards to the battlefield"
// registers one delayed trigger over the whole remembered set, and its return
// moves every remembered card still in exile — here three live creatures
// (Goldmeadow Lookout, the werewolf Burly Breaker, The Fabulous Frog-Man; the
// three Goldmeadow Harrier tokens exiled among them cease to exist). The
// kernel's delayed record once held only the sweep's first object, so Burly
// Breaker and The Fabulous Frog-Man stayed in exile (decision 656 of pair 545
// of the tail-53 focus pool; projection Cards[50].Face).
func TestMBCInvasionOfMoagReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "delayed_lki_sweep_return")
}
