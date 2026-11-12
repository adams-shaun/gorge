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

// A granted Myriad (CR 702.116a, 613.1f): Blade of Selves' "Equipped
// creature has myriad" gives the equipped creature the same triggered
// ability a printed K:Myriad does. The printed expansion only covers printed
// lines, so the granted keyword's trigger is synthesized from the keyword
// line (scriptfacts.GrantedKeywordTrigger) and rides the GrantTriggerPush
// path with the __kwMyriadGranted payload.

// grantedMyriadCombat equips seat 0's Grizzly Bears with Blade of Selves (the
// real corpus scripts), declares it attacking seat 1 and leaves the queued
// triggers ready to be put on the stack.
func grantedMyriadCombat(t *testing.T, seats int) (*Engine, Config, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	var named [2]*cards.Card
	for i, n := range []string{"Blade of Selves", "Grizzly Bears"} {
		c, ok := reg.Lookup(n)
		if !ok {
			t.Fatalf("%s missing from corpus", n)
		}
		if d := c.Link(); len(d) != 0 {
			t.Fatalf("link %s: %v", n, d)
		}
		named[i] = c
	}
	names := make([]string, seats)
	decks := make([][]*cards.Card, seats)
	for i := range names {
		names[i] = string(rune('a' + i))
		decks[i] = mountainDeck(t, 40)
	}
	decks[0] = append([]*cards.Card{named[0], named[1]}, mountainDeck(t, 38)...)
	cfg := seatZeroStart(Config{Seed: 196, Names: names, Decks: decks, Tokens: map[string]*cards.Card{}})
	e := New(cfg)
	e.Advance()
	var blade, bears state.ObjID
	for _, id := range append(e.G.Zone(state.ZLibrary, 0), e.G.Zone(state.ZHand, 0)...) {
		switch f := e.G.Obj(id).Face(); {
		case f == nil:
		case f.Name == "Blade of Selves":
			blade = id
		case f.Name == "Grizzly Bears":
			bears = id
		}
	}
	if blade == 0 || bears == 0 {
		t.Fatalf("Blade of Selves %d / Grizzly Bears %d not dealt", blade, bears)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: blade, From: e.G.Obj(blade).Zone, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.MoveZone, Obj: bears, From: e.G.Obj(bears).Zone, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.Attach, Obj: blade, IDs: []state.ObjID{bears}})
	// Precondition: the grant reached the layer system, and the Bears print
	// no Myriad of their own (the trigger can only be the granted one).
	if !slices.Contains(e.Derived(bears).Keywords, "Myriad") {
		t.Fatalf("precondition: equipped Grizzly Bears keywords %v lack the granted Myriad", e.Derived(bears).Keywords)
	}
	if len(e.G.Obj(bears).Face().Triggers) != 0 {
		t.Fatalf("precondition: Grizzly Bears print %d triggers", len(e.G.Obj(bears).Face().Triggers))
	}
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{bears}})
	e.priorityRound()
	return e, cfg, bears
}

// grantedMyriadPushes counts the granted Myriad triggers put on the stack.
func grantedMyriadPushes(e *Engine) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.GrantTriggerPush && ev.Counter == "__kwMyriadGranted" {
			n++
		}
	}
	return n
}

// TestGrantedMyriadCreatesCopyAttackingTheOtherOpponent: in a three-seat
// game the equipped Bears attack seat 1, so the granted Myriad offers one
// token copy, attacking seat 2 (the defending player is excluded), and the
// token is exiled as combat ends.
func TestGrantedMyriadCreatesCopyAttackingTheOtherOpponent(t *testing.T) {
	e, cfg, bears := grantedMyriadCombat(t, 3)
	if n := grantedMyriadPushes(e); n != 1 {
		t.Fatalf("granted Myriad pushed %d times, want 1", n)
	}
	d := passToDecision(t, e, 8)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "myriad" || d.Player != 0 || len(d.Options) != 2 {
		t.Fatalf("granted Myriad choice = %+v", d)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{0}}); err != nil {
		t.Fatalf("create Myriad copy: %v", err)
	}
	passUntilStackEmpty(t, e, 20)
	var tokens []state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o.IsToken && o.IsMyriad {
			tokens = append(tokens, id)
		}
	}
	if len(tokens) != 1 {
		t.Fatalf("granted Myriad created %d tokens, want 1", len(tokens))
	}
	tok := e.G.Obj(tokens[0])
	if tok.Face() == nil || tok.Face().Name != "Grizzly Bears" || !tok.IsAttacking || !tok.Tapped {
		t.Fatalf("Myriad token %+v: want a tapped attacking copy of the equipped Grizzly Bears", tok)
	}
	if tok.ID == bears {
		t.Fatal("the token is the equipped creature itself")
	}
	e.pending = nil
	e.setStep(state.StepEndCombat)
	e.advanceStep()
	if o := e.G.Obj(tokens[0]); o != nil && o.Zone == state.ZBattlefield {
		t.Fatal("granted Myriad token remained on the battlefield after combat")
	}
	replayCheck(t, e, cfg)
}

// TestGrantedMyriadTwoSeatsAsksNothing: with two players there is no
// opponent other than the defending player, so the granted trigger goes on
// the stack and resolves without a choice or a token -- the behaviour the
// two-player dirac kernel plays.
func TestGrantedMyriadTwoSeatsAsksNothing(t *testing.T) {
	e, cfg, _ := grantedMyriadCombat(t, 2)
	if n := grantedMyriadPushes(e); n != 1 {
		t.Fatalf("granted Myriad pushed %d times, want 1", n)
	}
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack %v, want the granted Myriad trigger alone", e.G.Stack)
	}
	for i := 0; i < 20 && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			t.Fatalf("two-seat granted Myriad asked %+v", d)
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
			t.Fatalf("two-seat granted Myriad created token %d", o.ID)
		}
	}
	replayCheck(t, e, cfg)
}
