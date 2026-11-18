package oracle

import "testing"

// Fangs of Kalonia's overload cast ({4}{G}{G}) beside a Lotus Blossom: gorge's
// priority menu offered the overloaded cast (mode 15) at decision 345 of the
// recording and the kernel's offer left it out (tail-74-c000 pair 7).
func TestMBCFangsOfKaloniaReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "overload_cast_dynamic_mana_source")
}
