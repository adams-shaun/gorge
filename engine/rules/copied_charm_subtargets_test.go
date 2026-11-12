package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestCopiedCharmKeepsPerModeTargets pins CR 707.10 for a modal spell: a copy
// of Flash Thompson's "tap target creature; untap target creature" enters
// trigger keeps one target group per chosen mode, so each mode of the copy
// acts on its own target instead of both modes running over the flat list.
func TestCopiedCharmKeepsPerModeTargets(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Flash Thompson, Spider-Fan", "Serra Angel")
	bear := searchMoveByName(t, e, "Grizzly Bears", state.ZBattlefield)
	angel := searchMoveByName(t, e, "Serra Angel", state.ZBattlefield)
	e.emit(events.Event{Kind: events.Tap, Obj: angel})
	e.priorityRound()

	trig := flashThompsonBothModes(t, e, reg, bear, angel)
	// PRECONDITION: the original records one target per selected mode, and the
	// two groups differ. Without that split a copy has nothing per-mode to
	// inherit, and the assertion on the copy could not distinguish a fix.
	orig := e.charmTargets[trig]
	if len(orig) != 2 || len(orig[0]) != 1 || len(orig[1]) != 1 {
		t.Fatalf("original's per-mode target groups: %+v", orig)
	}
	if orig[0][0].Obj == orig[1][0].Obj {
		t.Fatalf("both modes recorded the same target %d; the split is not exercised", orig[0][0].Obj)
	}
	seen := map[state.ObjID]int{}
	for _, group := range orig {
		seen[group[0].Obj]++
	}
	if seen[bear] != 1 || seen[angel] != 1 {
		t.Fatalf("original's groups %+v, want one each for %d and %d", orig, bear, angel)
	}

	copyElecting(e, events.Event{Kind: events.StackCopy, Obj: trig, Player: 0})
	copyID := e.G.Stack[len(e.G.Stack)-1]
	if copyID == trig {
		t.Fatalf("no copy on the stack: %v", e.G.Stack)
	}

	// The copy inherits the split, each group bound to the copy's own id.
	got := e.charmTargets[copyID]
	if len(got) != 2 || len(got[0]) != 1 || len(got[1]) != 1 {
		t.Fatalf("copy's per-mode target groups: %+v (want the original's %+v)", got, orig)
	}
	copied := map[state.ObjID]int{}
	for _, group := range got {
		copied[group[0].Obj]++
	}
	if copied[bear] != 1 || copied[angel] != 1 {
		t.Fatalf("copy's groups %+v, want one each for %d and %d", got, bear, angel)
	}

	for i := 0; i < 8 && e.G.Obj(copyID).Zone == state.ZStack; i++ {
		d := e.Pending()
		if d == nil {
			e.priorityRound()
			continue
		}
		if d.Kind == decision.KTarget {
			submitChoices(t, e, d.Options[0].Index)
			continue
		}
		passOnceP(t, e)
	}
	if e.G.Obj(copyID).Zone == state.ZStack {
		t.Fatalf("copy still on the stack, pending %+v", e.Pending())
	}
	// Each mode acted on its own declaration: the tapped bear is tapped by the
	// tap mode and the untapped angel keeps its state under the untap mode.
	if !e.G.Obj(bear).Tapped || e.G.Obj(angel).Tapped {
		t.Errorf("after the copy: bear tapped=%v (want true), angel tapped=%v (want false)",
			e.G.Obj(bear).Tapped, e.G.Obj(angel).Tapped)
	}
}

// TestCopiedAbilityKeepsSubTargets pins CR 707.10 for an ability's
// SubAbility$ link: a copy of Mechanical Mobster's enters trigger keeps the
// link's announced target, so the copy connives that creature and is never
// asked for the link's target again as it resolves.
func TestCopiedAbilityKeepsSubTargets(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Mechanical Mobster", "Serra Angel")
	bear := searchMoveByName(t, e, "Grizzly Bears", state.ZBattlefield)
	searchMoveByName(t, e, "Serra Angel", state.ZBattlefield)

	trig := mobsterTriggerOnStack(t, e, reg, bear)
	// PRECONDITION: the original's chain target is recorded both on the stack
	// object (SubTargets) and in the per-link record the copy inherits.
	if subs := e.G.Obj(trig).SubTargets; len(subs) != 1 || subs[0].Obj != bear {
		t.Fatalf("original's chain targets: %+v (want %d)", subs, bear)
	}
	if got := e.castSubTargets[trig]; len(got) == 0 {
		t.Fatalf("original has no per-link target record")
	}

	copyElecting(e, events.Event{Kind: events.StackCopy, Obj: trig, Player: 0})
	copyID := e.G.Stack[len(e.G.Stack)-1]
	if copyID == trig {
		t.Fatalf("no copy on the stack: %v", e.G.Stack)
	}
	if subs := e.G.Obj(copyID).SubTargets; len(subs) != 1 || subs[0].Obj != bear {
		t.Fatalf("copy's chain targets on the stack: %+v (want %d)", subs, bear)
	}

	start := len(e.L.Events)
	for i := 0; i < 8 && e.G.Obj(copyID).Zone == state.ZStack; i++ {
		d := e.Pending()
		switch {
		case d == nil:
			e.priorityRound()
		case d.Kind == decision.KPriority:
			passOnceP(t, e)
		case d.Kind == decision.KTarget:
			t.Fatalf("copy asked for the chain target while resolving: %+v", d)
		default:
			submitChoices(t, e, d.Options[0].Index) // the connive discard
		}
	}
	if e.G.Obj(copyID).Zone == state.ZStack {
		t.Fatalf("copy still on the stack, pending %+v", e.Pending())
	}
	connived := false
	for _, ev := range e.L.Events[start:] {
		if ev.Kind == events.Connive && ev.Obj == bear {
			connived = true
		}
	}
	if !connived {
		t.Errorf("the copy did not connive the original's chain target %d", bear)
	}
}
