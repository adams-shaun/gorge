package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The shared-stack sweep guards (task sharedstack1). state.Game.Zone returns
// g.Stack for EVERY player (state/game.go Zone), so any effect that loops
// AliveFrom(0) and calls g.Zone(z, p) over a zone list containing ZStack
// visits the same stack objects once per alive seat. The count fix
// (bc0d9e43, effects/count.go Count$ValidStack) is the precedent; these tests
// pin the same invariant on the five remaining sweeps: PumpAll's PumpZone$,
// AnimateAll's Zone$, RestartGame's keep-set note, Untap's ConditionZone$, and
// both branches of ChangeZoneAll's Origin$ walk.

// stackScanBoard builds a 2-seat game with one object on the SHARED stack and
// asserts the precondition every test below depends on: both seats alive, and
// g.Zone(ZStack, p) returning the same single object for both seats. A
// vacuous setup (empty stack, one seat) fails loudly here rather than passing
// a per-seat double-count test silently.
func stackScanBoard(t *testing.T, name, types string) (*fakeHost, *state.Object) {
	t.Helper()
	h := newHost(t, 2)
	if alive := h.g.AliveFrom(0); len(alive) != 2 {
		t.Fatalf("fixture precondition: %d alive seats, want 2", len(alive))
	}
	o := h.g.AddObject(mkCard(t, "Name:"+name+"\nTypes:"+types+"\nOracle:x\n"), 0)
	o.Zone = state.ZStack
	h.g.Stack = []state.ObjID{o.ID}
	if got := h.g.Zone(state.ZStack, 0); len(got) != 1 || got[0] != o.ID {
		t.Fatalf("fixture precondition: seat 0 stack = %v, want [%d]", got, o.ID)
	}
	if got := h.g.Zone(state.ZStack, 1); len(got) != 1 || got[0] != o.ID {
		t.Fatalf("fixture precondition: seat 1 stack = %v, want the shared [%d]", got, o.ID)
	}
	return h, o
}

// TestPumpAllPumpZoneAllVisitsSharedStackOnce: PumpZone$ All includes ZStack,
// so the sweep must register the stack card's pump grant exactly once, not
// once per alive seat, and schedule exactly one end-of-turn expiry for it.
// The pre-fix N-seat walk registered two P/T effects (and two AtEOT
// registrations) for the one stack card.
func TestPumpAllPumpZoneAllVisitsSharedStackOnce(t *testing.T) {
	h, o := stackScanBoard(t, "DoomedSpell", "Instant")
	// AtEOT$ makes the scheduled expiry observable; it schedules one
	// DelayedRegister per id in ateotIDs.
	Resolve(h, &Ctx{Controller: 0}, sa(t,
		"DB$ PumpAll | ValidCards$ Card.namedDoomedSpell | PumpZone$ All | NumAtt$ +1 | NumDef$ +1 | AtEOT$ Sacrifice"))

	var pt int
	for _, ce := range h.continuous {
		if ce.Source == o.ID && ce.Layer == state.LPT {
			pt++
		}
	}
	if pt != 1 {
		t.Fatalf("P/T registrations for stack card %d = %d, want 1 (one shared stack scans once)", o.ID, pt)
	}
	var sched int
	for _, ev := range h.log {
		if ev.Kind == events.DelayedRegister && ev.Obj == o.ID {
			sched++
		}
	}
	if sched != 1 {
		t.Fatalf("AtEOT registrations for stack card %d = %d, want 1", o.ID, sched)
	}
}

// TestAnimateAllZoneAllVisitsSharedStackOnce: the analogous AnimateAll Zone$
// All sweep. One stack card gets exactly one Animate registration.
func TestAnimateAllZoneAllVisitsSharedStackOnce(t *testing.T) {
	h, o := stackScanBoard(t, "DoomedSpell", "Instant")
	Resolve(h, &Ctx{Controller: 0}, sa(t,
		"DB$ AnimateAll | ValidCards$ Card.namedDoomedSpell | Zone$ All | Types$ Elemental"))

	var types int
	for _, ce := range h.continuous {
		if ce.Source == o.ID && ce.Layer == state.LType {
			types++
		}
	}
	if types != 1 {
		t.Fatalf("type registrations for stack card %d = %d, want 1 (one shared stack scans once)", o.ID, types)
	}
}

// TestRestartGameAllVisitsSharedStackOnce: RestartGame's RestrictFromZone$ All
// keep-set note walks the shared stack too. The stack card is kept (it does
// not match the discard spec), so its name must appear exactly once in the
// emitted Note on a 2-seat table. This pins the latent branch; no corpus
// carrier uses All.
func TestRestartGameAllVisitsSharedStackOnce(t *testing.T) {
	h, o := stackScanBoard(t, "DoomedSpell", "Instant")
	if face := h.g.Obj(o.ID).Face(); face == nil || face.Name != "DoomedSpell" {
		t.Fatalf("fixture precondition: stack object has no DoomedSpell face")
	}
	// RestrictFromValid$ names what the restart DISCARDS; "Creature" matches
	// neither the Instant stack card nor anything else here, so the stack
	// card is in the keep-set.
	Resolve(h, &Ctx{Controller: 0}, sa(t,
		"SP$ RestartGame | RestrictFromZone$ All | RestrictFromValid$ Creature"))

	var note string
	for _, ev := range h.log {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "restart would keep") {
			note = ev.Text
		}
	}
	if note == "" {
		t.Fatalf("no restart keep-set note emitted; log = %+v", h.log)
	}
	if n := strings.Count(note, "DoomedSpell"); n != 1 {
		t.Fatalf("stack card named %d times in keep-set note, want 1: %q", n, note)
	}
}

// TestUntapConditionZoneStackVisitsSharedStackOnce: Untap's ConditionZone$
// Stack counts the shared stack. With the counter correctly visited once, a
// single matching stack card meets ConditionCompare$ EQ1 and the remembered
// tapped permanent untaps. The pre-fix N-seat count made EQ1 false (the
// stack card was counted twice), so the permanent stayed tapped.
func TestUntapConditionZoneStackVisitsSharedStackOnce(t *testing.T) {
	h, o := stackScanBoard(t, "DoomedSpell", "Instant")
	// The permanent Untap's Defined$ Remembered names, tapped on the
	// battlefield: the compared value (untap happened) is observable.
	perm := h.g.AddObject(mkCard(t, "Name:Fixture Land\nTypes:Land\nOracle:x\n"), 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: perm.ID, From: state.ZLibrary, To: state.ZBattlefield})
	h.Emit(events.Event{Kind: events.Tap, Obj: perm.ID})
	if !h.g.Obj(perm.ID).Tapped {
		t.Fatalf("fixture precondition: remembered permanent is not tapped")
	}
	Resolve(h, &Ctx{Controller: 0, Remembered: []state.Target{{Obj: perm.ID}}}, sa(t,
		"DB$ Untap | Defined$ Remembered | ConditionPresent$ Card | ConditionZone$ Stack | ConditionCompare$ EQ1"))
	_ = o
	if h.g.Obj(perm.ID).Tapped {
		t.Fatalf("permanent stayed tapped: ConditionPresent$ Card + ConditionCompare$ EQ1 over Stack " +
			"must see the one shared stack card exactly once")
	}
}

// TestChangeZoneAllStackVisitsSharedStackOnce: both ChangeZoneAll branches
// (RandomOrder$ false and true) must emit exactly one MoveZone for a single
// shared stack object and reach the destination. The RandomOrder$ true branch
// is the currently observable duplicate (it collects every snapshot before
// emitting, so pre-fix it enqueued the object once per player); the false
// branch guards the same invariant for the ordinary loop.
func TestChangeZoneAllStackVisitsSharedStackOnce(t *testing.T) {
	for _, tc := range []struct {
		name string
		line string
	}{
		{"ordinary", "SP$ ChangeZoneAll | Origin$ Stack | Destination$ Exile | ChangeType$ Card"},
		{"random order", "SP$ ChangeZoneAll | Origin$ Stack | Destination$ Exile | ChangeType$ Card | RandomOrder$ True"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, o := stackScanBoard(t, "DoomedSpell", "Instant")
			Resolve(h, &Ctx{Controller: 0}, sa(t, tc.line))
			var moves int
			for _, ev := range h.log {
				if ev.Kind == events.MoveZone && ev.Obj == o.ID && ev.From == state.ZStack {
					moves++
				}
			}
			if moves != 1 {
				t.Fatalf("MoveZone events for stack card %d = %d, want 1 (one shared stack scans once)", o.ID, moves)
			}
			if got := h.g.Obj(o.ID).Zone; got != state.ZExile {
				t.Fatalf("stack card zone = %v, want Exile (destination reached)", got)
			}
		})
	}
}
