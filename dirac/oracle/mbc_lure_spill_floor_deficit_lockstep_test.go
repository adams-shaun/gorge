package oracle

import "testing"

// CR 509.1c: Nemesis Mask's "All creatures able to block Bristling Boar do
// so" is a requirement, and Boar's "can't be blocked by more than one
// creature" caps it at one blocker, so the most requirements any legal
// declaration obeys is one. The defender's other creature is Rograkh, Son of
// Rohgahh, a menace attacker: blocking it takes both able creatures, which
// leaves the lure obeyed by none, so no legal declaration contains a Rograkh
// pair and neither duty blocker may be offered one. The kernel once offered
// them: its spill logic saw each duty blocker free (the lure cannot host
// both) and its floor check saw the menace coverable by the other duty
// blocker, missing that the two readings cannot hold together (decision 1012
// of 1012; dr-mbc-baab pair 23).
func TestMBCFaithbearerPaladinReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "lure_spill_floor_deficit")
}
