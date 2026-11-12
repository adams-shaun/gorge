package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 702.62b: a card is suspended while it is in exile, has suspend and has a
// time counter on it -- however it got there. A spell that exiles itself with
// time counters as it resolves (Rousing Refrain, Inspiring Refrain) is
// suspended too: its upkeep tick removes the counters and the last one offers
// the free cast.
func TestSelfExiledSuspendSpellTicksAndIsOfferedItsCast(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, "Name:Refrainer\nManaCost:1 U\nTypes:Sorcery\nK:Suspend:1:U\n"+
		"A:SP$ Draw | NumCards$ 1 | SubAbility$ DBExile\n"+
		"SVar:DBExile:DB$ ChangeZone | Origin$ Stack | Destination$ Exile | WithCountersType$ TIME | WithCountersAmount$ 2\nOracle:x\n"))
	id := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MU], e.G.Players[0].Pool[state.MC] = 1, 1
	castMode(t, e, id, "")
	e.resolveTop()
	o := e.G.Obj(id)
	if o.Zone != state.ZExile || o.Counter("TIME") != 2 {
		t.Fatalf("precondition: the spell did not exile itself with two time counters: %+v", o)
	}
	e.beginTurn(0)
	if got := e.G.Obj(id).Counter("TIME"); got != 1 {
		t.Fatalf("first upkeep TIME=%d, want 1", got)
	}
	e.beginTurn(0)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) != 2 || d.Options[0].Kind != "suspend_cast_yes" {
		t.Fatalf("the last counter did not offer the free cast: %+v", d)
	}
}
