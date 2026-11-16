package oracle

import (
	"testing"
)

// An activated ability's own {T} cost is paid from its source (CR 602.2b,
// 601.2h), so the source is no mana source for the payment that funds the
// rest of the cost. gorge's witness for a hybrid-only activation the pool does
// not pay (Azorius Locket's {W/U}x4, {T}, Sacrifice) plans over the other
// sources and taps the second Locket; the kernel's lowered witness left the
// activated Locket in the census and tapped it for mana first (decision 636 of
// pair 13, surface autopay). The recording is on a custom pool, the table its
// own decks need: the tail buckets a synthetic pool deals re-cut whenever the
// lowered card set moves, and tail-75 stopped holding Azorius Locket.
func TestMBCHybridActivationSelfTapReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "hybrid_activation_self_tap_is_not_a_witness_source")
}
