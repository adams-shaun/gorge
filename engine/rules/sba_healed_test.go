package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const umbraZeroBearerSrc = "Name:Umbra Zero Bearer\nManaCost:1 G\nTypes:Creature Bear\nPT:0/0\nOracle:x\n"

// CR 704.3 with CR 702.89a: the umbra save removes the bearer's damage and destroys
// the Aura that was holding its toughness up, so the same check's next pass finds
// a 0/0 and moves it. The attempt memory used to carry it to the next call.
func TestUmbraSaveThenZeroToughnessSameCheck(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	umbra := mustCorpusCard(t, reg, "Bear Umbra")
	e, _ := tokenReplGame(t, 13, umbra)
	bearer := putToken(t, e, 0, umbraZeroBearerSrc, state.ZBattlefield)
	umbraID := attachCorpusAura(t, e, 0, umbra, bearer)
	if got := e.Toughness(bearer); got != 2 {
		t.Fatalf("precondition: bearer toughness %d under Bear Umbra, want 2", got)
	}

	e.emit(events.Event{Kind: events.Damage, Obj: bearer, Amount: 2})
	e.checkStateBased()
	if z := e.G.Obj(umbraID).Zone; z != state.ZGraveyard {
		t.Fatalf("precondition: Bear Umbra zone %s, want graveyard (it answered the destruction)", z)
	}
	if o := e.G.Obj(bearer); o != nil && o.Zone == state.ZBattlefield {
		t.Fatalf("0/0 bearer still on the battlefield after the check that destroyed its Aura")
	}
}
