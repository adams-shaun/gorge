package oracle

import "testing"

// CR 400.7: an object that leaves the battlefield and returns is a new
// object, so a pending ability's "return this permanent to its owner's hand"
// cannot find the permanent that came back. The game below activates
// Wydwen, the Biting Gale's {U}{B}, Pay 1 life bounce twice; the first
// bounce sends Wydwen to its owner's hand, a later cast puts a new Wydwen
// on the battlefield, and the older activation resolves onto that new
// object. gorge moved it; the kernel's RefSelf move (dirac/interp.go
// moveRef) no-ops on the source's changed version and stays there.
func TestSelfMoveSourceLeftSincePushReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "selfmove_source_left_since_push")
}
