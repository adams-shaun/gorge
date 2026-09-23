package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// phasesNotes counts the Note events in h.log whose text contains fragment.
func phasesNotes(h *fakeHost, fragment string) int {
	n := 0
	for _, ev := range h.log {
		if ev.Kind == events.Note && strings.Contains(ev.Text, fragment) {
			n++
		}
	}
	return n
}

// phasesRemembered counts the Choose "remembered" events (the persistent
// event-backed Remembered riders) in h.log.
func phasesRemembered(h *fakeHost) int {
	n := 0
	for _, ev := range h.log {
		if ev.Kind == events.Choose && ev.Counter == "remembered" {
			n++
		}
	}
	return n
}

// TestPhasesRememberAffectedCountsAffected pins the shape the bare Vanishing
// carrier Out of Time reads: AllValid$ Creature + RememberAffected$ True
// remembers every affected battlefield creature onto the source object's
// persistent list (the Choose "remembered" events) and the chain ctx, and the
// unrepresentable phase-out itself degrades to ONE loud note -- not a silent
// no-op and not an "unimplemented API" note.
func TestPhasesRememberAffectedCountsAffected(t *testing.T) {
	h := newHost(t, 2)
	srcObj := h.g.AddObject(mkCard(t, "Name:Fixture Enchantment\nTypes:Enchantment\nOracle:x\n"), 0)
	var creatures []state.ObjID
	card := mkCard(t, "Name:Fixture Creature\nTypes:Creature\nOracle:x\n")
	for i := 0; i < 2; i++ {
		o := h.g.AddObject(card, 0)
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
		creatures = append(creatures, o.ID)
	}
	c := &Ctx{Source: srcObj.ID, Controller: 0}
	Resolve(h, c, sa(t, "DB$ Phases | AllValid$ Creature | RememberAffected$ True | WontPhaseInNormal$ True"))

	if got := phasesRemembered(h); got != 2 {
		t.Fatalf("remembered events = %d, want one per affected creature (2)", got)
	}
	if got := len(c.Remembered); got != 2 {
		t.Fatalf("ctx Remembered = %d, want 2 (the chain's Count$RememberedSize read)", got)
	}
	if got := phasesNotes(h, "unimplemented API"); got != 0 {
		t.Fatalf("Phases is registered: got %d unimplemented-API notes, want 0", got)
	}
	if got := phasesNotes(h, "Phases: phase-out not represented"); got != 1 {
		t.Fatalf("phase-out degradation notes = %d, want exactly one loud note", got)
	}
}

// TestPhasesPhaseInIsLoudNoOp: the phase-in half (PhaseInOrOut$ True, the
// DBPhaseIn step Out of Time grants when it leaves) reverses nothing -- this
// build marks nothing phased out -- and says so once, remembering nothing.
func TestPhasesPhaseInIsLoudNoOp(t *testing.T) {
	h := newHost(t, 2)
	srcObj := h.g.AddObject(mkCard(t, "Name:Fixture Enchantment\nTypes:Enchantment\nOracle:x\n"), 0)
	card := mkCard(t, "Name:Fixture Creature\nTypes:Creature\nOracle:x\n")
	var creatures []state.ObjID
	for i := 0; i < 2; i++ {
		o := h.g.AddObject(card, 0)
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
		creatures = append(creatures, o.ID)
	}
	c := &Ctx{Source: srcObj.ID, Controller: 0, Remembered: []state.Target{{Obj: creatures[0]}, {Obj: creatures[1]}}}
	Resolve(h, c, sa(t, "DB$ Phases | Defined$ Remembered | PhaseInOrOut$ True"))

	if got := phasesRemembered(h); got != 0 {
		t.Fatalf("phase-in remembered %d object(s), want 0", got)
	}
	if got := phasesNotes(h, "Phases: phase-in runs as a no-op"); got != 1 {
		t.Fatalf("phase-in no-op notes = %d, want exactly one loud note", got)
	}
}
