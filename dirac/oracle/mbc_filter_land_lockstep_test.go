package oracle

import "testing"

// CR 106.1 and CR 601.2h: p1 holds {3}{U}{U} Messenger Drake with an Island,
// two Plains, Llanowar Wastes and a Flooded Grove ({G/U},{T}: add {U}{U})
// untapped. The Island pays the filter's {G/U}, the Grove adds {U}{U} and the
// other three lands pay the generic, so the cast is affordable; the kernel
// once left it out of the options (tail-15-c000 pair 9, decision 299).
func TestMBCMessengerDrakeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_land_hybrid_pay_in_cast")
}
