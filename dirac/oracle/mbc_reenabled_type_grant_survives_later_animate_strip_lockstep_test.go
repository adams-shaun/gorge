package oracle

import "testing"

// CR 613.8a: a subtype grant an animate re-enabled applies after every
// animate that re-enables it, so a second animate's RemoveCreatureTypes strip
// must not erase the granted type -- the animated permanent keeps the granted
// subtype beside the animate's own (Hivestone over Gruul War Plow, three
// grants and two activations; gorge once dropped the Sliver at the second
// strip, tail-24).
func TestMBCReenabledTypeGrantSurvivesLaterAnimateStripReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reenabled_type_grant_survives_later_animate_strip")
}
