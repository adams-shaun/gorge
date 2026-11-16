package oracle

import "testing"

// CR 602.2b with CR 605.3a: Centaur Garden's threshold ability costs {G}, {T}
// and a sacrifice of itself, and its own mana ability (a pain source: one
// damage to its controller) is its controller's only green. At two life the
// painful units sit in a budget group (painTight) rather than the plain
// census, and the activation's own source was still counted in that group, so
// the kernel offered an activation gorge refuses: the Garden cannot tap for the
// {G} of the cost that taps it. Decision 915 of the pair (tail-05 pair 22,
// focus Centaur Garden, seed 4).
func TestMBCCentaurGardenReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "painful_source_cannot_fund_its_own_activation_cost")
}
