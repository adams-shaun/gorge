package oracle

import "testing"

// Grab the Reins (CR 702.42a) is an entwine spell. Dr-mbc-bbbb pair 67: an
// untapped Krark-Clan Ironworks and two artifacts make the entwined cast
// (printed {3}{R} plus entwine {2}{R}) payable by activating the Ironworks
// twice ({C}{C} each, sacrificing The Rack and then the Ironworks), but
// gorge's potential bound counts a sacrifice-mana ability once and its
// potential walk therefore never lists the entwined mode. The kernel's script
// gap walk listed it, so the engines parted at the priority decision.
func TestMBCGrabTheReinsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "entwine_gap")
}
