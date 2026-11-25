package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestStaticZonesPlayerKeywordMutation proves the staticzones pool exercises
// the kernel's StatPlayerKeyword read of Defensive Formation (bandDivider's
// defenderAssigns): a kernel that never reads it leaves the attacking player
// dividing a double-blocked attacker's damage, so the decision's player
// differs from gorge's defender and some seed diverges.
func TestStaticZonesPlayerKeywordMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoPlayerKeyword = 1 << kop.PlKwDefenderAssigns
	defer func() { dirac.DebugNoPlayerKeyword = 0 }()
	if !lockstepDiverges(t, "staticzones", kstaticzonesDecks, 200) {
		t.Fatal("DebugNoPlayerKeyword(DefenderAssigns): no divergence in 200 seeds")
	}
}
