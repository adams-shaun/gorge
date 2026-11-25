package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCR704_5iLoyaltyXWalkerAtZeroDies: Nissa, Steward of Elements has printed
// loyalty X. CR 107.3/208.x makes X 0 anywhere but the stack, so a Nissa put
// onto the battlefield with no loyalty counters has loyalty 0 and CR 704.5i
// puts her into her owner's graveyard.
func TestCR704_5iLoyaltyXWalkerAtZeroDies(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _, ids := dsBoard(t, reg, "Nissa, Steward of Elements")
	id := ids["Nissa, Steward of Elements"]
	o := e.G.Obj(id)
	if o.Zone != state.ZBattlefield || o.FaceDown {
		t.Fatalf("precondition: zone %v face-down %v, want a face-up battlefield permanent", o.Zone, o.FaceDown)
	}
	if f := o.Face(); f == nil || !f.IsPlaneswalker() || f.Loyalty != "X" {
		t.Fatalf("precondition: face %+v, want a printed planeswalker with Loyalty:X", f)
	}
	if got := o.Counter("LOYALTY"); got != 0 {
		t.Fatalf("precondition: loyalty counters %d, want 0", got)
	}
	e.checkStateBased()
	if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
		t.Fatalf("CR 704.5i: Nissa with loyalty X (0 outside the stack) and no loyalty counters is in %v, want the graveyard", z)
	}
}
