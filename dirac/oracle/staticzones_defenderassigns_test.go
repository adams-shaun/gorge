package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestStaticZonesDefenderAssignsMutation proves the staticzones pool exercises
// the kernel's StatDefenderAssigns read of Defensive Formation (bandDivider's
// defenderAssigns): a kernel that never reads it leaves the attacking player
// dividing a double-blocked attacker's damage, so the decision's player
// differs from gorge's defender and some seed diverges.
func TestStaticZonesDefenderAssignsMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoDefenderAssigns = true
	defer func() { dirac.DebugNoDefenderAssigns = false }()
	if !lockstepDiverges(t, "staticzones", kstaticzonesDecks, 200) {
		t.Fatal("DebugNoDefenderAssigns: no divergence in 200 seeds")
	}
}
