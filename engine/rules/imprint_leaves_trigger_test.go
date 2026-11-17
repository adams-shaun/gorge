package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const leaveNexusSrc = `Name:Test Nexus
ManaCost:1
Types:Enchantment
T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Any | ValidCard$ Card.Self | Execute$ TrigReturn
SVar:TrigReturn:DB$ ChangeZone | Defined$ Imprinted | Origin$ Exile | Destination$ Hand | SubAbility$ DBWipe
SVar:DBWipe:DB$ Cleanup | ClearImprinted$ True
Oracle:probe
`

// nexusFixture puts a Test Nexus on seat 0's battlefield with a creature
// imprinted (exiled) beneath it, and a sweeper in hand that destroys it.
func nexusFixture(t *testing.T) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	sweep := card(t, "Name:Test Sweep\nManaCost:W\nTypes:Sorcery\nA:SP$ DestroyAll | ValidCards$ Enchantment\nOracle:probe\n")
	e := handEngine(t, sweep)
	nexus := onBoard(t, e, 0, leaveNexusSrc)
	c := e.G.AddObject(card(t, "Name:Linked Bear\nTypes:Creature\nPT:2/2\nOracle:x\n"), 0)
	c.Zone = state.ZLibrary
	e.G.SetZone(state.ZLibrary, 0, append(e.G.Zone(state.ZLibrary, 0), c.ID))
	e.emit(events.Event{Kind: events.MoveZone, Obj: c.ID, From: state.ZLibrary, To: state.ZExile, Player: 0})
	e.emit(events.Event{Kind: events.Imprint, Obj: nexus, IDs: []state.ObjID{c.ID}})
	if o := e.G.Obj(c.ID); o == nil || o.Zone != state.ZExile {
		t.Fatalf("precondition: linked creature must be in exile, got %+v", o)
	}
	if n := e.G.Obj(nexus); len(n.Imprinted) != 1 || n.Imprinted[0] != c.ID {
		t.Fatalf("precondition: Nexus Imprinted=%v, want [%d]", n.Imprinted, c.ID)
	}
	e.G.Players[0].Pool[state.MW] = 1
	e.askPriority(0)
	return e, nexus, c.ID
}

func TestImprintedReadByLeavesTrigger(t *testing.T) {
	t.Parallel()
	e, nexus, linked := nexusFixture(t)
	castFirst(t, e, "cast")
	for i := 0; i < 20; i++ {
		d := e.Pending()
		if d == nil || (len(e.G.Stack) == 0 && e.G.Obj(nexus).Zone != state.ZBattlefield) {
			break
		}
		submitChoices(t, e, passPriorityOption(t, d))
	}
	if z := e.G.Obj(nexus).Zone; z == state.ZBattlefield {
		t.Fatalf("precondition: the sweeper must have destroyed the Nexus (zone %v)", z)
	}
	if z := e.G.Obj(linked).Zone; z != state.ZHand {
		t.Fatalf("linked creature zone=%v, want hand (leaves trigger reads Imprinted)", z)
	}
	if n := e.G.Obj(nexus); len(n.Imprinted) != 0 {
		t.Fatalf("Cleanup ClearImprinted$ must empty the list, got %v", n.Imprinted)
	}
}

func TestImprintedClearedOnReentry(t *testing.T) {
	t.Parallel()
	e, nexus, linked := nexusFixture(t)
	e.emit(events.Event{Kind: events.MoveZone, Obj: nexus, From: state.ZBattlefield, To: state.ZGraveyard, Player: 0})
	if n := e.G.Obj(nexus); len(n.Imprinted) != 1 {
		t.Fatalf("precondition: the leave keeps the list for the trigger, got %v", n.Imprinted)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: nexus, From: state.ZGraveyard, To: state.ZBattlefield, Player: 0})
	n := e.G.Obj(nexus)
	if n.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Nexus must have re-entered, zone %v", n.Zone)
	}
	if len(n.Imprinted) != 0 || len(n.ImprintTokens) != 0 {
		t.Fatalf("re-entered Nexus sees old imprints %v / %v (linked %d)", n.Imprinted, n.ImprintTokens, linked)
	}
}
