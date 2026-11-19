package oracle

import "testing"

// CR 601.2g and CR 106.6: Ghitu Encampment's animation ({1}{R}) is payable
// from Power Depot's {T}: Add {C} and Relic of Legends' "Tap an untapped
// legendary creature you control: Add one mana of any color" (Lu Su, Wu
// Advisor is the legendary creature; that ability does not tap Relic, so
// Relic's own tapped state does not stop it). The kernel's tap-another search
// used to be skipped beside the board's filter (Overflowing Basin) and
// restricted (Power Depot) mana sources, so it missed the animation, and the
// restricted/filter census then offered Belltoll Dragon's plain cast and
// Laquatus's Creativity instead. Neither is payable: the restricted mana may
// not pay a non-artifact ability (CR 106.6), and the board makes only four
// mana. The recording is the tail-49 evidence game at decision 852.
func TestMBCGhituAnimationBesideRestrictedFilterLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "tapx_animation_beside_restricted_filter")
}

// The same recording's next divergence, at decision 862: once the animation
// is offered, its prefix must tap Relic of Legends' legendary creature (Lu
// Su) for {R} and let the payment plan take Razortide Bridge for the {1},
// exactly as gorge's prefix does. Without that apply the kernel tapped Power
// Depot and pushed no ability, a projection/stack divergence one decision
// later.
func TestMBCGhituAnimationPaymentLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "tapx_animation_payment_relic")
}
