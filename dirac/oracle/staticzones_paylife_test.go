package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestStaticZonesPayLifeMutation proves the pool exercises K'rrik's
// PayLifeInsteadOf:B grant: a kernel that reads no grant leaves every {B}
// pip a plain black pip, so the life-paid casts (Vampire Nighthawk off two
// Swamps at low life, Typhoid Rats off one, the K'rrik body itself) either
// become unaffordable or pay no life, and some seed diverges from gorge.
func TestStaticZonesPayLifeMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoPayLifeInsteadOfB = true
	defer func() { dirac.DebugNoPayLifeInsteadOfB = false }()
	if !lockstepDiverges(t, "staticzones", kstaticzonesDecks, 64) {
		t.Fatal("DebugNoPayLifeInsteadOfB: no divergence in 64 seeds")
	}
}
