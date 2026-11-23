package oracle

import "testing"

// Memnarch's {1}{U}{U} Animate ability needs both of p1's Shimmering Grottos
// as filters (CR 605.3a), one of them the Grotto Memnarch stole earlier (a
// layer-2 control record). gorge's prefix search treats the two Grottos as
// twins (scriptTwin and scriptManaTwins read their mana abilities, which the
// control change leaves alone) and finds the payment inside its node budget;
// the kernel's twin cut refused the pair, so its search ran out of the 160
// nodes and withheld the ability (decision 642 of tail-56 pair 196).
func TestMBCMemnarchReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "memnarch_animate_offer")
}
