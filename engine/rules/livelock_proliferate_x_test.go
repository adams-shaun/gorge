package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// "Proliferate X times" (CR 701.34a) is X separate proliferate actions. With
// an unchanged recipient set answered "all" every iteration logs the same
// signatures (counter events, the Proliferate marker, the ask, the answer),
// and before Proliferate joined mintingKinds' ordinal fold the watcher read
// one large-X Expansion Algorithm resolution as a repeating cycle.

// proliferateIteration feeds one 27-event proliferate iteration (24 distinct
// counter recipients, the marker, the ask, the answer) to w; marker is the
// kind logged in the marker's slot.
func proliferateIteration(w *livelockWatcher, seq *uint64, marker events.Kind) {
	next := func(ev events.Event) {
		*seq++
		ev.Seq = *seq
		w.observeFrom(&ev, 0, 0)
	}
	for i := 0; i < 24; i++ {
		next(events.Event{Kind: events.CounterChange, Obj: state.ObjID(100 + i), Counter: "P1P1", Amount: 1})
	}
	next(events.Event{Kind: marker, Player: 0})
	next(events.Event{Kind: events.DecisionAsk, Player: 0, Text: "choose"})
	next(events.Event{Kind: events.DecisionMade, Player: 0, Text: "choose:[0 ... 23]"})
}

// TestLivelockWatcherProliferateXIsNotACycle: 30 identical proliferate
// iterations (810 events, past the default 400-event cycle threshold) are
// one finite resolution, not a loop. The same stream with the marker
// replaced by a non-ordinal kind still trips, so the detector is not
// weakened for the rest of the iteration's shape.
func TestLivelockWatcherProliferateXIsNotACycle(t *testing.T) {
	const reps = 30
	if reps*27 < defaultCycleEvents*2 {
		t.Fatalf("precondition: %d events do not cross twice the %d-event cycle threshold", reps*27, defaultCycleEvents)
	}
	func() {
		w := newLivelockWatcher(nil)
		var seq uint64
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("%d proliferate iterations tripped the watcher at seq %d: %v", reps, seq, r)
			}
		}()
		for i := 0; i < reps; i++ {
			proliferateIteration(&w, &seq, events.Proliferate)
		}
	}()
	w := newLivelockWatcher(nil)
	var seq uint64
	defer func() {
		r := recover()
		le, ok := r.(*LivelockError)
		if !ok || le.Reason != "repeating cycle" {
			t.Fatalf("the same stream with a Note marker did not trip as a repeating cycle (recovered %v)", r)
		}
	}()
	for i := 0; i < reps; i++ {
		proliferateIteration(&w, &seq, events.Note)
	}
}

// TestLivelockProliferateXTimesCompletes casts the real Expansion Algorithm
// with X=40 over 12 Grizzly Bears each holding one +1/+1 counter and answers
// every proliferate ask with every option: the resolution completes, no
// *LivelockError escapes, and each bear ends with 41 counters.
func TestLivelockProliferateXTimesCompletes(t *testing.T) {
	t.Parallel()
	const bears, x = 12, 40
	reg := testutil.CorpusRegistry(t)
	e, cfg := proliferateEngine(t, reg, "Expansion Algorithm")
	ids := make([]state.ObjID, 0, bears)
	for i := 0; i < bears; i++ {
		id := putNamedOnBattlefield(t, e, "Grizzly Bears")
		e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "P1P1", Amount: 1})
		ids = append(ids, id)
	}
	e.priorityRound()
	for _, id := range ids {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield || o.Counter("P1P1") != 1 {
			t.Fatalf("precondition: bear %d not on the battlefield with one +1/+1 counter", id)
		}
	}
	algo := searchMoveByName(t, e, "Expansion Algorithm", state.ZHand)
	addMana(t, e, 0, "UU"+strings.Repeat("C", x))
	var asks int
	func() {
		defer func() {
			if r := recover(); r != nil {
				if le, ok := r.(*LivelockError); ok {
					t.Fatalf("Expansion Algorithm X=%d after %d asks: %v", x, asks, le)
				}
				panic(r)
			}
		}()
		opt := castByName(t, e, 0, "Expansion Algorithm")
		if opt == nil {
			t.Fatal("Expansion Algorithm not castable with UU + 40 C")
		}
		submitChoices(t, e, opt.Index)
		announceXCast(t, e, algo, x)
		for d := passUntilNonPriority(t, e, 60); d != nil && d.Kind == decision.KChoose && d.ResumeKind == "proliferate"; asks++ {
			all := make([]int, len(d.Options))
			for i := range all {
				all[i] = i
			}
			d = answerProliferate(t, e, d, all...)
		}
		passUntilStackEmpty(t, e, 60)
	}()
	if asks != x {
		t.Errorf("answered %d proliferate asks, want %d", asks, x)
	}
	if len(e.G.Stack) != 0 {
		t.Errorf("stack depth %d after the resolution, want 0", len(e.G.Stack))
	}
	for _, id := range ids {
		if got := e.G.Obj(id).Counter("P1P1"); got != 1+x {
			t.Errorf("bear %d holds %d +1/+1 counters, want %d", id, got, 1+x)
		}
	}
	replayCheck(t, e, cfg)
}
