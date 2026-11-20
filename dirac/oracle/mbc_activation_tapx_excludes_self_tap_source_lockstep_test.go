package oracle

import "testing"

// TestMBCSkrelvDefectorMiteReprosLockstep replays the synthetic tail-14 game
// the evidence loop recorded: Skrelv, Defector Mite's {W/P}, {T} ability is
// activated and its Phyrexian pip is funded through Jaspera Sentinel's "{T},
// Tap an untapped creature you control: Add one mana of any color". Skrelv is
// the activation's own source and its {T} is still owed, so it cannot also be
// the creature Jaspera taps (CR 601.2g-h: the whole cost is paid at once, and
// a permanent cannot be tapped twice for one payment). gorge's
// pay.ManaTapCandidates withholds the source through CastCommitted's
// pc.cost.Tap arm; the kernel's tapXCands only withheld Jaspera itself and
// offered Skrelv (divergence at decision 715: options options, the kernel's
// tapcost list held one extra option).
func TestMBCSkrelvDefectorMiteReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "activation_tapx_excludes_self_tap_source")
}
