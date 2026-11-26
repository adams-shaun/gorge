package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestStaticZonesAltEquipMutation proves the pool exercises the
// StatAltEquipCost static's alternative (Kíli the Resourceful's, Kor Halberd
// beside it): a kernel that offers no alternative (static_altcost.go
// DebugNoAltEquip) makes the kernel's priority options miss gorge's "(alternate
// cost)" option, so some seed diverges. A kernel that kept offering the
// alternative past the turn's first equip (the count gate read at the charge)
// would diverge the same way; the count gate's own read is the kernel's
// thisTurnActEquip bit, which TestStaticZonesLockstep's games exercise
// whenever a second equip of a turn is offered.
func TestStaticZonesAltEquipMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoAltEquip = true
	defer func() { dirac.DebugNoAltEquip = false }()
	if !lockstepDiverges(t, "staticzones", kstaticzonesDecks, 64) {
		t.Fatal("DebugNoAltEquip: no divergence in 64 seeds")
	}
}
