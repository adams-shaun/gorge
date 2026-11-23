package oracle

import "testing"

// CR 602.2b, 605.3a: Svogthos, the Restless Tomb's {3}{B}{G} animate is payable
// with Urban Utopia's granted "{T}: Add one mana of any color" on a Mountain
// (the black), the Forest (the green) and the other lands. gorge offers the
// activation at decision 499 of tail-44 pair 70; the kernel did not.
func TestMBCSvogthosTheRestlessTombReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "svogthos_animate_unoffered_over_granted_any_colour")
}
