package oracle

import "testing"

// CR 605.1a, 605.2a with gorge's planner: a Spirit Guide's hand mana ability prices
// no activated ability at priority. Ghitu War Cry's {R} was affordable to the kernel
// only through Simian Spirit Guide in p1's hand (every red land tapped), so the
// kernel offered activate(Ghitu War Cry) where gorge offered pass alone
// (tail-45-c000 pair 1, decision 614).
func TestMBCGhituWarCryReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "hand_guide_funded_activation_not_offered")
}
