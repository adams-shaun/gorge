package oracle

import "testing"

// CR 616.1e with CR 615.7: Wellgabber Apothecary's "prevent all damage that
// would be dealt to target tapped Merfolk" activated twice on the one attacker
// leaves two unbounded shields on it. Each is a bare full prevention, so
// whichever applies first prevents the whole event and the other has nothing
// left: gorge applies them in scan order without the CR 616.1 order ask
// (engine/rules allBarePreventions) and the kernel must too, not stall at
// prevnext on the blockers' combat damage (tail-20-c000 pair 7).
func TestMBCTail20PrevNextUnboundedShieldsLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "prevnext_two_unbounded_shields")
}
