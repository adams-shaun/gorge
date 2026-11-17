package oracle

import "testing"

// CR 106.1b, 106.6 and 602.5b: Shrine of the Forsaken Gods' second ability,
// {T}: Add {C}{C} for colorless spells only while its controller has seven or
// more lands, is a larger alternative of the {C} beside it. gorge's census
// lists both (pay.appendUnitAlternatives) and pays a colorless {2} Necropede
// from the Shrine alone. The kernel's restricted-pair shape matched one colour
// of WUBRG only, so the {C}{C} lowered as a widened {C}: it tapped the Shrine
// for one and a Forest beside it, and the projections parted on the Forest's
// tapped bit (tail-08 pair 2 seed 1, decision 420, the basic-land Flags class).
// The recording is pooled custom: the tail bucket re-cuts as the lowered set moves.
func TestMBCShrineGatedRestrictedPairReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "shrine_forsaken_gods_restricted_pair")
}
