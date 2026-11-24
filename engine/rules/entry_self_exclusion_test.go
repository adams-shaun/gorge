package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// xproof gorge-suspect:orb-of-dreams:cast:permanents: Orb of Dreams'
// "Permanents enter tapped." made Orb of Dreams itself enter tapped. CR
// 614.12: a replacement that modifies how a permanent enters applies to that
// permanent's own entry only if it affects that permanent alone, not a general
// subset that includes it; Forge and XMage both put the Orb in untapped.
func TestOrbOfDreamsEntersUntapped(t *testing.T) {
	t.Parallel()
	reg := freshCorpusRegistry(t, "o/orb_of_dreams.txt", "g/grizzly_bears.txt", "f/forest.txt")
	e, _ := etbreplEngine(t, reg, "Orb of Dreams")
	orb := castFromHand(t, e, "Orb of Dreams", "CCC")
	if o := e.G.Obj(orb); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Orb of Dreams did not reach the battlefield: %+v", o)
	}
	if e.G.Obj(orb).Tapped {
		t.Fatal("Orb of Dreams entered tapped: its own general \"Permanents enter tapped\" does not apply to its entry (CR 614.12)")
	}
	// The line is live for every later entrant, so the self case is not vacuous.
	searchMoveByName(t, e, "Forest", state.ZHand)
	land := playLandFromHand(t, e, "Forest")
	if o := e.G.Obj(land); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Forest did not reach the battlefield: %+v", o)
	}
	if !e.G.Obj(land).Tapped {
		t.Fatal("Forest played beside Orb of Dreams entered untapped, want tapped")
	}
}
