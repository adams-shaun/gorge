package oracle

import "testing"

// CR 601.2g-h, 107.3: Gnottvold Slumbermound's {3}{R}{G}{G}, {T}, Sacrifice:
// Destroy target land is paid after its target ask, from a board holding
// Mage-Ring Network. The kernel's script prefix for the sacrifice-cost play
// floated Mage-Ring Network's {C} before the target ask where gorge floats
// nothing (decision 652 of tail-55 pair 4, autopay, seed 158630: projection
// Cards[23].Flags and Pool[0][5]). The filter prefix search now asks the
// storage battery's X as gorge does (c8e7202741), which closed it; the
// recording is that game's 652-action prefix, recorded on the unfixed kernel
// (946ba0cb92).
func TestMBCGnottvoldSlumbermoundPrefixReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamedServed(t, "storage_battery_x_ask_before_sacrifice_cost_target")
}
