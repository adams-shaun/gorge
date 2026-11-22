package oracle

import "testing"

// CR 605.1a/605.3a: Rikala, Homarid King at three tide counters turns every
// Island tap into two blue mana, so Trapjaw Kelpie ({4}{G/U}{G/U}, Flash) is
// castable from p1's upkeep hand with the floating pool empty: tap the two
// Islands, each bringing the gated TapsForMana trigger's extra {U} as it
// resolves at the tap. gorge's planner proves the script and offers the cast;
// the kernel's payment-window census priced only the ungated tap extras and
// withheld it (the decision-402 options lockstep of tail-02 pair 13).
func TestMBCTrapjawKelpieReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "gated_tapextra_offer")
}
