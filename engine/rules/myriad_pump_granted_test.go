package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestPumpGrantedMyriadFires: a Pump's "gains haste and myriad until end of
// turn" (Duke Ulder Ravengard's TrigPump, CR 702.116a) is a registered
// continuous effect whose AddKeywords carries Myriad. Its trigger is
// synthesized by the granted walk (grantedKeywordTrigger), so the
// registration must end the trigger-grant proof (continuousGrantsTrigger) or
// the walk never runs and the trigger is silently dropped. Two seats: the
// trigger goes on the stack once and resolves with no choice and no token.
func TestPumpGrantedMyriadFires(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	var named [2]*cards.Card
	for i, n := range []string{"Duke Ulder Ravengard", "Grizzly Bears"} {
		c, ok := reg.Lookup(n)
		if !ok {
			t.Fatalf("%s missing from corpus", n)
		}
		if d := c.Link(); len(d) != 0 {
			t.Fatalf("link %s: %v", n, d)
		}
		named[i] = c
	}
	decks := [][]*cards.Card{
		append([]*cards.Card{named[0], named[1]}, mountainDeck(t, 38)...),
		mountainDeck(t, 40),
	}
	cfg := seatZeroStart(Config{Seed: 196, Names: []string{"a", "b"}, Decks: decks, Tokens: map[string]*cards.Card{}})
	e := New(cfg)
	e.Advance()
	var duke, bears state.ObjID
	for _, id := range append(e.G.Zone(state.ZLibrary, 0), e.G.Zone(state.ZHand, 0)...) {
		switch f := e.G.Obj(id).Face(); {
		case f == nil:
		case f.Name == "Duke Ulder Ravengard":
			duke = id
		case f.Name == "Grizzly Bears":
			bears = id
		}
	}
	if duke == 0 || bears == 0 {
		t.Fatalf("Duke Ulder Ravengard %d / Grizzly Bears %d not dealt", duke, bears)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: duke, From: e.G.Obj(duke).Zone, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.MoveZone, Obj: bears, From: e.G.Obj(bears).Zone, To: state.ZBattlefield})
	// Precondition: the Bears print no triggers and no Myriad before the Pump.
	if len(e.G.Obj(bears).Face().Triggers) != 0 {
		t.Fatalf("precondition: Grizzly Bears print %d triggers", len(e.G.Obj(bears).Face().Triggers))
	}
	if slices.Contains(e.Derived(bears).Keywords, "Myriad") {
		t.Fatalf("precondition: Grizzly Bears have Myriad %v before the Pump", e.Derived(bears).Keywords)
	}
	dukeFace := e.G.Obj(duke).Face()
	e.resolveAbility(duke, 0, []state.Target{{Obj: bears}}, resolveSVarOf(t, dukeFace, "TrigPump"), dukeFace.SVars)
	if kws := e.Derived(bears).Keywords; !slices.Contains(kws, "Myriad") || !slices.Contains(kws, "Haste") {
		t.Fatalf("precondition: pumped Grizzly Bears keywords %v lack the granted Haste and Myriad", kws)
	}
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{bears}})
	e.priorityRound()
	if n := grantedMyriadPushes(e); n != 1 {
		t.Fatalf("Pump-granted Myriad pushed %d times, want 1", n)
	}
	for i := 0; i < 20 && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			t.Fatalf("two-seat Pump-granted Myriad asked %+v", d)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
			t.Fatalf("pass priority: %v", err)
		}
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack %v did not empty", e.G.Stack)
	}
	for _, o := range e.G.Objs {
		if o.IsMyriad {
			t.Fatalf("two-seat Pump-granted Myriad created token %d", o.ID)
		}
	}
	replayCheck(t, e, cfg)
}
