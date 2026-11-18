package oracle

import "testing"

// Scald ("whenever a player taps an Island for mana, Scald deals 1 damage to
// that player") makes every Island a Taps-deferred gap (CR 605.1a, 603.2).
// With two life, three untapped Islands and three Forests, Fallowsage ({3}{U})
// is a play only a prefix pays: gorge's search taps the first Island and the
// Forests, lets the one trigger resolve (life 1) and offers the cast, after the
// deeper lines that tap every Island have died to the triggers (CR 704.5a).
// The kernel resolved only the prefix that tapped every gap, found it fatal and
// dropped the cast (pair 30, pool tail-49 focus, decision 481 of 481).
func TestMBCFallowsageReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "taps_defer_partial_prefix_survives_resolve")
}
