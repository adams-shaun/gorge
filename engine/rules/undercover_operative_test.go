package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Undercover Operative's as-enters copy: "You may have Undercover Operative
// enter as a copy of any creature on the battlefield, except it enters with a
// shield counter on it if you control that creature" (CR 707.9b, 614.1c,
// 614.12). The shield counter is part of the copy exception: it rides the
// copy election, so a declined election or a copy of an opponent's creature
// gets no shield counter.

// cloneCopyNamed returns the id of the seat's battlefield object named name
// that a clone copied (the one carrying the ClonePermanent event): when the
// copy and its template share a name in the same zone, the name alone cannot
// tell them apart.
func cloneCopyNamed(t *testing.T, e *Engine, seat state.PlayerID, name string) state.ObjID {
	t.Helper()
	found := false
	for _, id := range e.G.Zone(state.ZBattlefield, seat) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			found = true
			if hasEvent(e, events.ClonePermanent, id) {
				return id
			}
		}
	}
	if !found {
		t.Fatalf("no %s on seat %d's battlefield", name, seat)
	}
	t.Fatalf("%s is on seat %d's battlefield but no copy was made (no ClonePermanent event)", name, seat)
	return 0
}

// TestUndercoverOperativeCopiesOwnCreatureWithShield: the controller copies
// their own Grizzly Bears; the Operative enters as a 2/2 Grizzly Bears with
// exactly one shield counter, before its enters triggers see it.
func TestUndercoverOperativeCopiesOwnCreatureWithShield(t *testing.T) {
	t.Parallel()
	raw := `{"name":"undercover-operative-own","cr":["707.9b","614.12"],"why":"copy with shield rider","setup":{` +
		`"p0":{"hand":["Undercover Operative"],"battlefield":["Grizzly Bears"]},` +
		`"p1":{"battlefield":["Hill Giant"]}},` +
		`"steps":[{"op":"cast","seat":0,"card":"p0:Undercover Operative","mana":"CCUU"},` +
		`{"op":"resolve","seat":0,"answers":[{"kind":"choose","pick":["p0:Grizzly Bears"]}]}],"expect":[]}`
	run := runETBCloneScenario(t, raw)
	e := run.e
	copyID := cloneCopyNamed(t, e, 0, "Grizzly Bears")
	if got := e.G.Obj(copyID).Counter("SHIELD"); got != 1 {
		t.Fatalf("copy SHIELD counters = %d, want 1", got)
	}
	der := e.Derived(copyID)
	if der.Power != 2 || der.Toughness != 2 || !slices.Contains(der.Types, "Bear") {
		t.Fatalf("copy should be a 2/2 Grizzly Bears: %+v", der)
	}
	if !hasEvent(e, events.ClonePermanent, copyID) {
		t.Fatal("precondition: no copy was made")
	}
	// The shield counter is part of the entry: it lands before any of the
	// copy's enters triggers are pushed (CR 614.12).
	counterAt, triggerAt := -1, -1
	for i := range e.L.Events {
		ev := &e.L.Events[i]
		if ev.Kind == events.CounterChange && ev.Obj == copyID && ev.Counter == "SHIELD" {
			counterAt = i
		}
		if ev.Kind == events.TriggerPush && ev.Obj == copyID && triggerAt < 0 {
			triggerAt = i
		}
	}
	if counterAt < 0 {
		t.Fatal("no SHIELD CounterChange on the copy")
	}
	if triggerAt >= 0 && counterAt > triggerAt {
		t.Fatalf("copy's shield counter (event %d) landed after its enters trigger (event %d)", counterAt, triggerAt)
	}
}

// TestUndercoverOperativeCopiesOpponentCreatureNoShield: copying an opponent's
// Hill Giant makes a 3/3 Hill Giant with no shield counter.
func TestUndercoverOperativeCopiesOpponentCreatureNoShield(t *testing.T) {
	t.Parallel()
	raw := `{"name":"undercover-operative-opponent","cr":["707.9b"],"why":"opponent creature copy","setup":{` +
		`"p0":{"hand":["Undercover Operative"],"battlefield":["Grizzly Bears"]},` +
		`"p1":{"battlefield":["Hill Giant"]}},` +
		`"steps":[{"op":"cast","seat":0,"card":"p0:Undercover Operative","mana":"CCUU"},` +
		`{"op":"resolve","seat":0,"answers":[{"kind":"choose","pick":["p1:Hill Giant"]}]}],` +
		`"expect":[{"card":"p0:Hill Giant","pt":"3/3","counters":{}}]}`
	run := runETBCloneScenario(t, raw)
	copyID := findOn(t, run.e, state.ZBattlefield, 0, "Hill Giant")
	if !hasEvent(run.e, events.ClonePermanent, copyID) {
		t.Fatal("precondition: no copy was made")
	}
	if got := run.e.G.Obj(copyID).Counter("SHIELD"); got != 0 {
		t.Fatalf("copy of an opponent's creature holds %d SHIELD counters, want 0", got)
	}
	der := run.e.Derived(copyID)
	if der.Power != 3 || der.Toughness != 3 {
		t.Fatalf("copy should be a 3/3 Hill Giant: %+v", der)
	}
}

// TestUndercoverOperativeDeclineEntersAsItself: a declined election enters the
// Operative as itself, a 0/0 with no shield counter, which dies to the
// state-based actions (CR 614.1c: the optional replacement is not applied).
func TestUndercoverOperativeDeclineEntersAsItself(t *testing.T) {
	t.Parallel()
	raw := `{"name":"undercover-operative-decline","cr":["614.1c","707.9b"],"why":"optional decline","setup":{` +
		`"p0":{"hand":["Undercover Operative"],"battlefield":["Grizzly Bears"]},` +
		`"p1":{"battlefield":["Hill Giant"]}},` +
		`"steps":[{"op":"cast","seat":0,"card":"p0:Undercover Operative","mana":"CCUU"},` +
		`{"op":"resolve","seat":0,"answers":[{"kind":"choose","pick":["Enter as itself"]}]}],"expect":[]}`
	run := runETBCloneScenario(t, raw)
	e := run.e
	id := findOn(t, e, state.ZGraveyard, 0, "Undercover Operative")
	o := e.G.Obj(id)
	if got := o.Counter("SHIELD"); got != 0 {
		t.Fatalf("declined entry holds %d SHIELD counters, want 0", got)
	}
	if hasEvent(e, events.ClonePermanent, id) {
		t.Fatal("a declined election must not copy")
	}
	if findOn(t, e, state.ZBattlefield, 0, "Grizzly Bears") <= 0 {
		t.Fatal("precondition: the controller's Grizzly Bears vanished")
	}
}

// Hulking Metamorph's as-enters copy: "You may have Hulking Metamorph enter as
// a copy of an artifact or creature you control, except it's an artifact
// creature in addition to its other types, and its power and toughness are
// equal to Hulking Metamorph's power and toughness" (CR 707.9b). The P/T
// exception reads the ENTERING object's own power/toughness before the copy
// overwrites it: 3/3 when cast prototyped, 7/7 when cast normally.

// TestHulkingMetamorphPrototypedCopyIs3x3: prototyped ({2}{U}{U}) Hulking
// Metamorph copies its controller's Ornithopter; the copy is a 3/3 artifact
// creature named Ornithopter with flying.
func TestHulkingMetamorphPrototypedCopyIs3x3(t *testing.T) {
	t.Parallel()
	raw := `{"name":"hulking-metamorph-proto","cr":["707.9b","718.3"],"why":"own P/T copy exception","setup":{` +
		`"p0":{"hand":["Hulking Metamorph"],"battlefield":["Ornithopter"]}},` +
		// The pool covers the printed {9} as well: with only the {2}{U}{U}
		// prototype cost covered, the quiet proof claims the seat is quiet
		// while the walk offers the prototype cast (a pre-existing gap, named
		// in the report) and the scenario harness panics before the cast.
		`"steps":[{"op":"cast","seat":0,"card":"p0:Hulking Metamorph","mana":"CCCCCCCCUU","cast_mode":"prototype"},` +
		`{"op":"resolve","seat":0,"answers":[{"kind":"choose","pick":["p0:Ornithopter"]}]}],"expect":[]}`
	run := runETBCloneScenario(t, raw)
	e := run.e
	copyID := cloneCopyNamed(t, e, 0, "Ornithopter")
	der := e.Derived(copyID)
	if der.Power != 3 || der.Toughness != 3 {
		t.Fatalf("prototyped copy should be 3/3: %+v", der)
	}
	for _, ty := range []string{"Artifact", "Creature", "Thopter"} {
		if !slices.Contains(der.Types, ty) {
			t.Fatalf("copy lacks type %q: %v", ty, der.Types)
		}
	}
	if !slices.Contains(der.Keywords, "Flying") {
		t.Fatalf("copy lacks the Ornithopter's flying: %v", der.Keywords)
	}
	if !hasEvent(e, events.ClonePermanent, copyID) {
		t.Fatal("precondition: no copy was made")
	}
}

// TestHulkingMetamorphFullCopyIs7x7: cast for its printed {9}, the copy is a
// 7/7 artifact creature.
func TestHulkingMetamorphFullCopyIs7x7(t *testing.T) {
	t.Parallel()
	raw := `{"name":"hulking-metamorph-full","cr":["707.9b"],"why":"own P/T copy exception","setup":{` +
		`"p0":{"hand":["Hulking Metamorph"],"battlefield":["Ornithopter"]}},` +
		`"steps":[{"op":"cast","seat":0,"card":"p0:Hulking Metamorph","mana":"CCCCCCCCC"},` +
		`{"op":"resolve","seat":0,"answers":[{"kind":"choose","pick":["p0:Ornithopter"]}]}],"expect":[]}`
	run := runETBCloneScenario(t, raw)
	copyID := cloneCopyNamed(t, run.e, 0, "Ornithopter")
	der := run.e.Derived(copyID)
	if der.Power != 7 || der.Toughness != 7 {
		t.Fatalf("full-cost copy should be 7/7: %+v", der)
	}
	if got := run.e.G.Obj(copyID).Counter("SHIELD"); got != 0 {
		t.Fatalf("Metamorph copy holds %d SHIELD counters, want 0", got)
	}
}
