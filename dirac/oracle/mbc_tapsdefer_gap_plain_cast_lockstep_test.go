package oracle

import (
	"testing"
)

// CR 601.2b with CR 605.4a: a mana source whose tap fires a Taps watcher the
// payment planner cannot vet is a deferred census gap, and on the lowering
// surface gorge's list still carries the plain hand cast the potential
// planner lowers over the gap -- the prefix search activates the source
// (Burning Earth's 1 damage to the tapper prices in), then prices the play.
// Dragon-Cursed Halls' {U}/{B}/{R} beside Burning Earth is the gap Planar
// Despair's {3}{B}{B} needed; the kernel's plain census left the Halls out
// and dropped the cast gorge offered.
func TestMBCPlanarDespairReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "tapsdefer_gap_plain_cast")
}
