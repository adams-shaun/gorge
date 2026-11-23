package oracle

import "testing"

// Fangs of Kalonia's overload cast ({4}{G}{G}) beside a Lotus Blossom: gorge's
// priority menu offered the overloaded cast (mode 15) at decision 345 of the
// recording and the kernel's offer left it out (tail-74-c000 pair 7).
//
// The reverse: with two petal counters on the Lotus, four Swamps and a
// Celestial Prism (a fee-carrying any-colour gap), gorge's prefix search spends
// its 160-clone budget on the Prism's fee branches before it reaches the Lotus,
// prices the overloaded cast census_incomplete and does not offer it, so the
// kernel must not offer it either (tail-74-c001 pair 195, decision 156).
func TestMBCFangsOfKaloniaReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "overload_cast_dynamic_mana_source")
	stdReproNamed(t, "overload_cast_without_target")
	stdReproNamed(t, "overload_cast_funded_by_granted_double_mana")
}
