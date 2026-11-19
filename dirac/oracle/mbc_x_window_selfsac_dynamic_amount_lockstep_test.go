package oracle

import "testing"

// CR 601.2b with CR 605.3b: Lotus Blossom's "{T}, Sacrifice this: Add X mana
// of any one color, where X is its petal counters" is a deterministic
// self-sacrifice mana ability whose Amount$ is a resolvable SVar
// (Count$CardCounters.PETAL), so a cast's X ask must price it in the
// CR 601.2g window: with {R}{B}{B} floating, two petal counters and a blue
// permanent for Torch Breath's "{2} less if it targets a blue permanent",
// X=6 pays {4}{R} from the pool plus the Blossom's two mana. gorge's
// castWindowProbeUnits once withheld the source, capping the menu at 4.
func TestMBCTorchBreathReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "x_window_selfsac_dynamic_amount")
}
