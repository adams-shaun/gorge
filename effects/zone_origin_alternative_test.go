package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Round-2 pins for the ChangeZone OriginAlternative$ work (review findings).

// TestChangeZoneCompoundOriginWithDefinedMovesTheChosenCard is the Eladamri,
// Korvecdal guard: a compound `Origin$` WITHOUT an OriginAlternative$ keeps
// its existing object dispatcher, so `Defined$ ChosenCard` moves the
// already-chosen hand card -- never a fresh whole-library pick that would
// move the library's first eligible card instead (a free any-card fetch).
func TestChangeZoneCompoundOriginWithDefinedMovesTheChosenCard(t *testing.T) {
	h, c := fixtureHost(t)
	elf := mkCard(t, "Name:Chosen Elf\nTypes:Creature Elf\nPT:2/2\nOracle:x\n")
	chosen := h.g.AddObject(elf, 0)
	chosen.Zone = state.ZHand
	fodder := mkCard(t, "Name:Library Fodder\nTypes:Creature\nPT:1/1\nOracle:x\n")
	fodderObj := h.g.AddObject(fodder, 0)
	fodderObj.Zone = state.ZLibrary
	// Eladamri's chain sub, params verbatim.
	s := sa(t, "DB$ ChangeZone | Defined$ ChosenCard | Origin$ Library,Hand | ConditionDefined$ ChosenCard | ConditionPresent$ Creature | Destination$ Battlefield")
	cc := &Ctx{Source: c.Source, Controller: 0,
		Chosen: []state.Target{{Obj: chosen.ID}}, ChosenValid: true}
	effChangeZone(h, cc, s)

	if o := h.g.Obj(chosen.ID); o.Zone != state.ZBattlefield {
		t.Fatalf("chosen hand card zone = %v, want battlefield", o.Zone)
	}
	if o := h.g.Obj(fodderObj.ID); o.Zone != state.ZLibrary {
		t.Fatalf("library card moved: compound Origin$ with Defined$ must never pose a fresh library pick")
	}
	for _, e := range h.log {
		if e.Kind == events.MoveZone && e.Obj == fodderObj.ID {
			t.Fatalf("library card moved: compound Origin$ with Defined$ must never pose a fresh library pick")
		}
	}
}

// TestChangeZoneUnknownOriginAlternativeNotesAndKeepsKnownZones is the
// invasion_of_arcavios shape: `Origin$ Library | OriginAlternative$
// Graveyard,Sideboard`. Sideboard parses to no modelled zone; the effect
// must note the unmodelled alternative LOUDLY and still search every KNOWN
// zone (library + graveyard), never bail the whole effect -- bailing would
// lose the library half the pre-OriginAlternative engine still searched.
func TestChangeZoneUnknownOriginAlternativeNotesAndKeepsKnownZones(t *testing.T) {
	h, c := fixtureHost(t)
	s := sa(t, "DB$ ChangeZone | Hidden$ True | Origin$ Library | Destination$ Hand | OriginAlternative$ Graveyard,Sideboard | ChangeType$ Sorcery.YouOwn,Instant.YouOwn | ShuffleNonMandatory$ True")
	effChangeZone(h, c, s)

	var altNotes, bailNotes int
	for _, e := range h.log {
		if e.Kind != events.Note {
			continue
		}
		if e.Text == "unrecognised ChangeZone OriginAlternative Graveyard,Sideboard" {
			altNotes++
		}
		if e.Text == "unrecognised ChangeZone Origin Library" {
			bailNotes++
		}
	}
	if altNotes != 1 {
		t.Fatalf("unrecognised-OriginAlternative notes = %d, want 1: %+v", altNotes, h.log)
	}
	if bailNotes != 0 {
		t.Fatalf("the whole effect bailed on the unmodelled alternative zone; the known zones must keep searching: %+v", h.log)
	}
}
