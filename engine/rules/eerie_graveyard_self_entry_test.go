package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 603.10 / 603.10a: an "enters" trigger is checked against the game after
// the event; only leaves-the-zone triggers look back. Fear of Infinity's Eerie
// line ("whenever an enchantment you control enters, you may return this card
// from your graveyard") functions only from the graveyard, so when Fear itself
// moves from the graveyard to the battlefield it is no longer in the zone where
// the ability functions and must not trigger on its own entry. A second copy
// still in the graveyard does trigger.
const eerieGraveyardSrc = "Name:Test Eerie Returner\nManaCost:1 U B\nTypes:Enchantment Creature Nightmare\nPT:2/2\n" +
	"T:Mode$ ChangesZone | TriggerZones$ Graveyard | Destination$ Battlefield | Origin$ Any | ValidCard$ Enchantment.YouCtrl | OptionalDecider$ You | Execute$ EerieBack | TriggerDescription$ x\n" +
	"SVar:EerieBack:DB$ ChangeZone | Destination$ Hand | Origin$ Graveyard\n" +
	"Oracle:x\n"

func TestGraveyardEnterTriggerDoesNotSeeItsOwnEntry(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	mover := inZoneCard(t, e, 0, state.ZGraveyard, eerieGraveyardSrc)
	stayer := inZoneCard(t, e, 0, state.ZGraveyard, eerieGraveyardSrc)
	e.pendingTriggers = nil
	e.emit(events.Event{Kind: events.MoveZone, Obj: mover, From: state.ZGraveyard, To: state.ZBattlefield})
	if o := e.G.Obj(mover); o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: the mover is in zone %v, want the battlefield", o.Zone)
	}
	fired := map[state.ObjID]int{}
	for _, pt := range e.pendingTriggers {
		fired[pt.Source]++
	}
	if fired[stayer] != 1 {
		t.Fatalf("precondition: the copy still in the graveyard fired %d times on the enchantment's entry, want 1", fired[stayer])
	}
	if fired[mover] != 0 {
		t.Errorf("the entering card's graveyard-only enters trigger fired %d times on its own entry, want 0", fired[mover])
	}
}
