package oracle

import "testing"

// CR 702.26b: a phased-out permanent is treated as though it does not exist,
// so an effect that moves it does nothing. The game below activates Wydwen,
// the Biting Gale's {U}{B}, Pay 1 life bounce, then Vodalian Illusionist's
// "target creature phases out" at Wydwen in response; the phase-out resolves
// first and the bounce then finds a phased-out Wydwen. gorge's ChangeZone
// skips it (changeZoneMoveSkip); the kernel's moveRef read the phased-out
// object as an off-battlefield card and moved it to the hand.
func TestPhasedOutTargetIgnoresBounceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "phased_out_target_ignores_bounce")
}
