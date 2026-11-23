package oracle

import "testing"

// CR 704.3 with CR 605.3a and CR 614.9: Cephalid Coliseum's {U} deals 1
// damage to its controller, and Pariah's Shield redirects it to the equipped
// Apprentice Wizard (0/1), the only filter that turns that {U} into {C}{C}{C}.
// gorge prices a play by activating sources at priority, so the state-based
// check between the Coliseum's tap and the Wizard's kills the Wizard and Jester's Cap
// and Pariah's Shield's equip stay unpayable. The kernel's search kept the
// damaged Wizard and offered both (decision 383, tail-66-c000 pair 70).
func TestMBCJesterSCapReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "pain_redirect_kills_filter_before_play")
}
