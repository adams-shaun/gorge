package oracle

import "testing"

// TestActivationRestrictedBoundOptionsLockstep: a plain battlefield activation
// the payment-restricted potential bound cannot afford is no offer on the
// any-mana walk. gorge prices an activation offer against the bound built for
// that payment (pay.AvailableFor, CR 106.6: a restricted source whose
// RestrictValid$ refuses the payment adds nothing to it), so Jeweled Lotus's
// commander-only mana is not in the bound an Urza, Lord High Artificer {5}
// shuffle is priced against. The kernel's open any-mana walk counted the
// three mana for the activation, so it offered activate(Urza) at a priority
// decision gorge ended with [pass] (synth tail-64 pair 137, decision 575,
// class options options). The cast of the class is restrBoundDrops
// (cast_restrbound.go); this is its activation arm.
func TestActivationRestrictedBoundOptionsLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "activation_offer_restricted_bound_refusal")
}
