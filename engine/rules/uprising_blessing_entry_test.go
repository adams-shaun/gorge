package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// CR 702.131b/d: "any time you control ten or more permanents ... you get the
// city's blessing", and after a player gets the blessing "continuous effects
// are reapplied before the game checks to see if the game state or preceding
// events have matched any trigger conditions". So a Dusk Charger (3/3,
// +2/+2 while its controller has the blessing) entering as the TENTH
// permanent is a 5/5 when Garruk's Uprising's "whenever a creature you
// control with power 4 or greater enters" trigger is matched: the grant and
// the reapplication its BlessingChange performs precede the entry's own
// trigger check. Before the promotion the blessing was granted in the
// post-fold tail, AFTER checkTriggers, so the entry matched at 3/3 and the
// draw never fired (dr-mbc-abaa pair 176, projection NStack).
func TestUprisingEntryTriggerReadsTheBlessingItsEntryGrants(t *testing.T) {
	t.Parallel()
	e, _ := fdnCensusEngine(t, "Garruk's Uprising", "Dusk Charger",
		"Grizzly Bears", "Grizzly Bears", "Grizzly Bears", "Grizzly Bears",
		"Grizzly Bears", "Grizzly Bears", "Grizzly Bears", "Grizzly Bears")
	moveByName(t, e, 0, "Garruk's Uprising", state.ZBattlefield)
	// Uprising plus eight Bears: nine permanents, unblessed -- the Charger's
	// entry is the one that crosses the gate.
	for i := 0; i < 8; i++ {
		moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	}
	settle := func() {
		e.pending = nil
		e.Advance()
		fdnCensusDrain(t, e)
	}
	settle()
	if e.G.Players[0].Blessing {
		t.Fatalf("nine permanents granted the blessing before the Charger entered")
	}
	hand := func() int { return len(e.G.Zone(state.ZHand, 0)) }
	before := hand()
	moveByName(t, e, 0, "Dusk Charger", state.ZBattlefield)
	settle()
	if !e.G.Players[0].Blessing {
		t.Fatalf("the tenth permanent (an Ascend carrier) did not grant the blessing")
	}
	if got := hand(); got != before+1 {
		t.Fatalf("the Charger entered as a blessed 5/5: hand %d -> %d, want one draw", before, got)
	}
}
