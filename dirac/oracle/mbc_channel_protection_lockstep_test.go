package oracle

import "testing"

// CR 702.16b with CR 602.2b: Shinen of Stars' Light's Channel ability
// ({1}{W}, discard it: target creature gains first strike) is activated from
// the hand, and its source is the white card there. The only creature on the
// battlefield is Empty-Shrine Kannushi, which has protection from the colours
// of the permanents its controller controls; it is white itself, so it has
// protection from white and can't be the target of an ability from a white
// source. With no legal target the ability can't be activated (CR 601.2c), so
// the priority decision offers only the pass. dirac once offered the
// activation (decision 148; tail-06-c000 pair 28 at land 94197a4cea).
func TestMBCShinenOfStarsLightReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "channel_no_target_past_own_colour_protection")
}
