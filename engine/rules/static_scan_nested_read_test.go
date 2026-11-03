package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A static scan reads Derived through a spec match (Agatha's Soul Cauldron's
// GainsAbilitiesOf$ names the creature cards it exiled), and Derived builds
// active() while the scan is still writing the memo. That nested build must
// see the previous complete static list, never the half-written one: it
// shares the unchanged-list shortcut's key, and a list kept against a memo
// that has already moved disagrees with a full build. The permanent entering
// below scans before the Cauldron (seat order), so the memo holds only its
// effect when the Cauldron's spec is matched.
func TestStaticScanNestedReadSeesThePreviousList(t *testing.T) {
	e := layerEngine(t)
	// A deck holding a layer-4 type changer and a SetName carrier arms the
	// derived-type table, whose
	// read is what reaches active() from the scan; the fixture's deck has none.
	e.layer4InPool, e.setNameInPool = true, true
	cauldron := onBoardCard(t, e, 1, corpusCard(t, "Agatha's Soul Cauldron"))
	keep := inZoneCard(t, e, 1, state.ZGraveyard, "Name:Exiled body\nTypes:Creature Human\nPT:1/1\nOracle:x\n")
	e.emit(events.Event{Kind: events.MoveZone, Obj: keep, From: state.ZGraveyard, To: state.ZExile, IDs: []state.ObjID{cauldron}})
	bear := onBoardCard(t, e, 1, corpusCard(t, "Grizzly Bears"))
	e.emit(events.Event{Kind: events.CounterChange, Obj: bear, Counter: "P1P1", Amount: 1})
	gear := inZoneCard(t, e, 0, state.ZHand, corpusCardText(t, "b/bespoke_battlegarb.txt"))
	e.AddContinuous(ContinuousEffect{Source: bear, Timestamp: 1, Layer: LType, Affects: "Card.Self",
		AddTypes: []string{"Artifact"}})
	e.active()
	if len(e.staticContinuous) == 0 {
		t.Fatal("precondition: the Cauldron contributes no static effect")
	}
	before := len(e.staticContinuous)
	e.emit(events.Event{Kind: events.MoveZone, Obj: gear, From: state.ZHand, To: state.ZBattlefield})
	e.active()
	if len(e.staticContinuous) != before+1 {
		t.Fatalf("static effects = %d, want %d", len(e.staticContinuous), before+1)
	}
}
