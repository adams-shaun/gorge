package oracle

import "testing"

// Baboon Spirit's "exile another target creature you control" can target a
// Spirit token. The two engines offered different targets for it
// (tail-57 pair 779: decision 390 of the recording, a choose:type
// decision for p0).
func TestMBCBaboonSpiritReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "baboon_spirit_blink_token")
}
