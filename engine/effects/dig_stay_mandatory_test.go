package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// digStayHost stands in for the rules engine's CR 303.4g gate: the fakeHost
// double applies every event it is handed, so this wrapper swallows the
// battlefield-entry MoveZone of the objects in declined the way
// engine/rules/aura_entry.go's auraEntryGate does -- the Aura stays in its
// zone and no fold, replacement or trigger sees it.
type digStayHost struct {
	*fakeHost
	declined map[state.ObjID]bool
}

func (h *digStayHost) Emit(e events.Event) {
	if e.Kind == events.MoveZone && e.To == state.ZBattlefield && h.declined[e.Obj] {
		return
	}
	h.fakeHost.Emit(e)
}

// TestDigSilentTakeLeavesAStayedAuraInTheRest pins the mandatory (silent)
// take: a Dig with no more eligible cards than it takes runs no ask, and its
// take of an Aura whose battlefield entry the engine declined (CR 303.4g --
// nothing it can legally enchant) must not count that Aura as taken. The
// Aura stays in the library window and joins "the rest", so the remainder
// move keeps its top-len(ids)-are-the-ids contract (CR 303.4g, 701.29a): the
// Aura goes to the bottom with the rest, once, instead of being dropped from
// the library while a window card below it is duplicated.
func TestDigSilentTakeLeavesAStayedAuraInTheRest(t *testing.T) {
	h := newHost(t, 2)
	aura := mkCard(t, "Name:Stay\nTypes:Enchantment Aura\nK:Enchant:Creature\nOracle:x\n")
	land := mkCard(t, "Name:Isle\nTypes:Basic Land Island\nOracle:x\n")
	// The window is [Aura, land] and a third card sits under it: the fix's
	// rest move (the Aura to the bottom) and the bug's (nothing, with the
	// Aura still marked taken) differ in the library's order.
	ids := []state.ObjID{
		h.g.AddObject(aura, 0).ID,
		h.g.AddObject(land, 0).ID,
		h.g.AddObject(land, 0).ID,
	}
	h.g.SetZone(state.ZLibrary, 0, ids)
	// Precondition: the window's Aura has nothing it can legally enchant --
	// no creature is on any battlefield -- so its entry is the declined one.
	// ChangeNum$ All over a fully matching two-card window is no choice, so
	// the take is the silent one and no ask runs.
	d := &digStayHost{fakeHost: h, declined: map[state.ObjID]bool{ids[0]: true}}
	Resolve(d, &Ctx{Controller: 0}, sa(t, "SP$ Dig | Defined$ You | DigNum$ 2 | ChangeNum$ All | ChangeValid$ Card | DestinationZone$ Battlefield"))
	if bf := h.g.Zone(state.ZBattlefield, 0); len(bf) != 1 || bf[0] != ids[1] {
		t.Fatalf("battlefield = %v, want [%d] (the window's land: the take ran)", bf, ids[1])
	}
	if lib := h.g.Zone(state.ZLibrary, 0); len(lib) != 2 || lib[0] != ids[2] || lib[1] != ids[0] {
		t.Fatalf("library = %v, want [%d %d] (the declined Aura went to the bottom once, with the rest)", lib, ids[2], ids[0])
	}
}

// TestDigStayedAuraRunsNoEntryRiders pins the declined take's missing entry:
// a card whose move the engine declined never entered the battlefield, so no
// entry rider runs for it -- no entered-tapped Tap (CR 614.12 reads an entry
// that did not happen) and no RememberChanged$ Remember (the card did not
// change zone; a chained effect reading Remembered would see a library card).
// The take that DID move still gets its riders.
func TestDigStayedAuraRunsNoEntryRiders(t *testing.T) {
	h := newHost(t, 2)
	aura := mkCard(t, "Name:Stay\nTypes:Enchantment Aura\nK:Enchant:Creature\nOracle:x\n")
	land := mkCard(t, "Name:Isle\nTypes:Basic Land Island\nOracle:x\n")
	ids := []state.ObjID{h.g.AddObject(aura, 0).ID, h.g.AddObject(land, 0).ID}
	h.g.SetZone(state.ZLibrary, 0, ids)
	d := &digStayHost{fakeHost: h, declined: map[state.ObjID]bool{ids[0]: true}}
	c := &Ctx{Controller: 0}
	Resolve(d, c, sa(t, "SP$ Dig | Defined$ You | DigNum$ 2 | ChangeNum$ All | ChangeValid$ Card | DestinationZone$ Battlefield | Tapped$ True | RememberChanged$ True"))
	if bf := h.g.Zone(state.ZBattlefield, 0); len(bf) != 1 || bf[0] != ids[1] {
		t.Fatalf("battlefield = %v, want [%d] (the take ran)", bf, ids[1])
	}
	taps := 0
	for i := range d.log {
		e := &d.log[i]
		if e.Kind == events.Tap {
			taps++
			if e.Obj != ids[1] {
				t.Fatalf("Tap event %+v: the declined Aura (%d) must not be tapped for an entry it never made", e, ids[0])
			}
		}
	}
	if taps != 1 {
		t.Fatalf("%d Tap events, want 1 (the moved land's entered-tapped)", taps)
	}
	if len(c.Remembered) != 1 || c.Remembered[0].Obj != ids[1] {
		t.Fatalf("Remembered = %+v, want only the moved land (%d); the declined Aura never left the library", c.Remembered, ids[1])
	}
}
