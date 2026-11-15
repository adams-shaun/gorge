package oracle

import (
	"testing"
)

// CR 601.2g-h with CR 603.3b: p0's only way to pay Dynacharge's overload
// ({2}{R} over a lone Mountain) is its untapped Eldrazi Spawn token, whose
// activation sacrifices it; the sacrifice fires the opponent's two
// Dreadhound death triggers, and the opponent must order them before p0 can
// finish paying. The gorge potential-play witness once refused the replay
// there (its other-player walk passed only priority decisions), so gorge
// never offered the overloaded cast the kernel offers.
func TestMBCDynachargeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "overload_witness_trigger_order")
}
