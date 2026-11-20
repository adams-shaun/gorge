package oracle

import "testing"

// CR 601.2a, 601.2g: p1 holds See the Unwritten with only the granted
// "{T}: Add {G}" (Summoning Materia on a transformed Ulvenwald Captive) and
// Unknown Shores' paid any-colour mana able to make {G}{G}. Gorge's prefix
// search took the equipped creature's printed {C}{C} before the granted {G}
// and never priced the cast, so only the kernel offered it (tail-34 pair 33,
// decision 598); the ask now orders by the colour the play needs.
func TestMBCSeeTheUnwrittenReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "see_the_unwritten_priority_cast")
}
