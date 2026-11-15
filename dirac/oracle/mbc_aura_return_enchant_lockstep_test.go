package oracle

import "testing"

// CR 303.4f: an Aura put onto the battlefield without being cast is enchanted
// to what its controller chooses as it enters. Primal Surge exiles the top
// card of its controller's library and, on a "yes", returns it: with Feral
// Invocation (an Aura) on top, gorge posed the enchant ask and the kernel
// panicked reading the Aura's card from an unset object (index out of range
// [255] with length 236) because the remembered Aura of that return was not
// the object the ask looked for. The recording is the first divergence of
// tail-73 pair 969: decision 835, gorge leading, on the autopay surface.
func TestMBCPrimalSurgeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "aura_return_remembered_enchant_ask")
}
