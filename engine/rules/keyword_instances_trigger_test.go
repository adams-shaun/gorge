package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestKeywordInstancesTriggerSeparately: every instance of Dethrone, Training,
// Ward or Cumulative upkeep on one permanent triggers on its own (CR 113.2c,
// 702.21, 702.24b, 702.105b, 702.149b). A printed line owns one derived entry,
// its own expansion queuing that instance; every other equal entry is a granted
// instance and queues too. The printed-only and granted-once rows hold the
// count from the other side: no instance triggers twice.
func TestKeywordInstancesTriggerSeparately(t *testing.T) {
	t.Parallel()
	keywords := []string{"Dethrone", "Training", "Ward:2", "Cumulative upkeep:1"}
	shapes := []struct {
		name            string
		printed, grants int
	}{
		{"printed and granted", 1, 1},
		{"granted twice", 0, 2},
		{"printed twice and granted", 2, 1},
		{"printed once", 1, 0},
		{"granted once", 0, 1},
	}
	seed := uint64(4810)
	for _, kw := range keywords {
		for _, sh := range shapes {
			seed++
			seed := seed
			t.Run(kw+"/"+sh.name, func(t *testing.T) {
				t.Parallel()
				keywordInstancesCase(t, seed, kw, sh.printed, sh.grants)
			})
		}
	}
}

// keywordInstancesCase puts a 2/2 printing kw printed times on seat 0 under
// grants lords granting it, fires the keyword's event once and requires one
// trigger per instance. The two attack keywords are then resolved, so each
// queued trigger also has to put its counter and survive a log-only replay.
func keywordInstancesCase(t *testing.T, seed uint64, kw string, printed, grants int) {
	t.Helper()
	want := printed + grants
	e, cfg := tokenReplGame(t, seed)
	bear := putToken(t, e, 0, "Name:Instance Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\n"+
		strings.Repeat("K:"+kw+"\n", printed)+"Oracle:x\n", state.ZBattlefield)
	var ox state.ObjID
	if kw == "Training" {
		// Minted before the lords so the grant reaches it too; only the
		// bear's triggers are counted.
		ox = putToken(t, e, 0, "Name:Big Ox\nManaCost:4 G\nTypes:Creature Ox\nPT:6/6\nOracle:x\n", state.ZBattlefield)
	}
	for range grants {
		putToken(t, e, 0, mutateGrantLordSrc(kw), state.ZBattlefield)
	}
	o := e.G.Obj(bear)
	if o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("precondition: the bear is not on seat 0's battlefield: %+v", o)
	}
	held := 0
	for _, k := range e.Derived(bear).Keywords {
		if k == kw {
			held++
		}
	}
	if held != want {
		t.Fatalf("precondition: derived keywords %q hold %q %d times, want %d", e.Derived(bear).Keywords, kw, held, want)
	}

	from := len(e.L.Events)
	switch kw {
	case "Dethrone":
		e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{bear}})
	case "Training":
		if e.Power(ox) <= e.Power(bear) {
			t.Fatalf("precondition: the Ox %d is not stronger than the bear %d", e.Power(ox), e.Power(bear))
		}
		e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{bear, ox}})
	case "Ward:2":
		cause := e.G.Zone(state.ZLibrary, 1)[0]
		e.emit(events.Event{Kind: events.PutOnStack, Obj: cause, Player: 1, From: state.ZLibrary, To: state.ZStack})
		e.emit(events.Event{Kind: events.TargetsChosen, Obj: cause, IDs: []state.ObjID{bear}})
	case "Cumulative upkeep:1":
		if e.G.Active != 0 {
			t.Fatalf("precondition: seat %d is active, want seat 0", e.G.Active)
		}
		e.emit(events.Event{Kind: events.StepChange, Step: state.StepUpkeep})
	default:
		t.Fatalf("no firing event for %q", kw)
	}
	if got := queuedSince(e, bear, from); got != want {
		t.Fatalf("%s printed %d times and granted %d times queued %d triggers, want %d", kw, printed, grants, got, want)
	}

	if kw != "Dethrone" && kw != "Training" {
		return
	}
	e.priorityRound()
	mutateDrain(t, e, 40)
	if len(e.G.Stack) != 0 || len(e.pendingTriggers) != 0 {
		t.Fatalf("%d stack entries and %d pending triggers after the drain", len(e.G.Stack), len(e.pendingTriggers))
	}
	if got := e.G.Obj(bear).Counter("P1P1"); got != int32(want) {
		t.Fatalf("%d %s instances put %d +1/+1 counters, want %d", want, kw, got, want)
	}
	replayCheck(t, e, cfg)
}
