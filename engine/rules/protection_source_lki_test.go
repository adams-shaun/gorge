package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestProtectionFromCreaturesReadsACeasedCopySource pins CR 113.7a on the
// protection source: a token copy that left the battlefield (ZCeased) still
// carries the copied face, so an ability whose source it was is an ability
// "from a source with the stated quality" (CR 702.16b) and cannot target a
// permanent with protection from creatures. Before the last-known-source
// read, copyCeased rejected every spec against the tombstoned copy, so the
// source stopped counting as a creature and the protected target was offered.
func TestProtectionFromCreaturesReadsACeasedCopySource(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	eesha := e.G.AddObject(card(t, "Name:Commander Eesha\nTypes:Creature Bird Soldier\nPT:2/4\nK:Protection:Creature\nOracle:x\n"), 0)
	eesha.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{eesha.ID})

	// A token copy of a creature is a real permanent only while it is on the
	// battlefield (CR 707.10g); off the battlefield it ceases (CR 707.10h).
	copied := e.G.AddObject(card(t, "Name:Grizzly Bears\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 1)
	copied.Zone = state.ZBattlefield
	copied.IsToken, copied.IsCopy = true, true
	e.G.SetZone(state.ZBattlefield, 1, []state.ObjID{copied.ID})

	// Precondition: while the copy is a live creature, protection withholds
	// the target from its ability's source.
	if !e.protectedFrom(eesha.ID, copied.ID) {
		t.Fatal("precondition: protection from creatures did not withhold Commander Eesha from a live creature source")
	}
	e.emit(events.Cease(copied.ID, state.ZBattlefield))
	o := e.G.Obj(copied.ID)
	if o == nil || o.Zone != state.ZCeased || !o.IsCopy {
		t.Fatalf("precondition: the copy did not cease to exist in ZCeased: %+v", o)
	}
	if !e.protectedFrom(eesha.ID, copied.ID) {
		t.Fatalf("CR 702.16b/113.7a: Commander Eesha was not protected from the ceased copy source (last-known types %v)", e.derivedTypesOf(copied.ID))
	}
}
