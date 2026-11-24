package oracle

import "testing"

// Blue Sun's Twilight offered as a cast by one engine only at priority
// (decision 1092 of tail-28 pair 8).
func TestMBCBlueSunSTwilightCastOfferReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "blue_suns_twilight_cast_offer")
}
