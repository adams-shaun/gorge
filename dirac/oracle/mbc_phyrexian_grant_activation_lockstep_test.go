package oracle

import "testing"

// CR 107.4f, CR 119.4, CR 602.2b: Trespassing Souleater's "{U/P}: can't be
// blocked this turn" with its controller at 1 life and Chromatic Lantern tapped.
// The only blue left is a Mountain's granted any-colour ability. gorge's planner
// prices the mana half of a Phyrexian activation (nothing) and never witnesses a
// grant, so the potential play it lists cannot be lowered (the two life the pip
// would cost are not there), and no scripted prefix is searched for it: gorge
// offers no activation. The kernel once offered it, counting the grant
// (decision 392 of tail-72 pair 14).
func TestMBCTrespassingSouleaterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "phyrexian_activation_granted_only_unlowerable")
}
