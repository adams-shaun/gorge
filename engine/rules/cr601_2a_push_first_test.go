package rules

// CR 601.2a: "To propose the casting of a spell, a player first moves that
// card ... from where it is to the stack." Every later step (601.2b's
// announcements, 601.2c's targets, 601.2h's cost choices) happens with the
// spell already on the stack, so a cost can never be paid with the card being
// cast. Real corpus cards, read as script text (corpusCardText); the board is
// built eventlessly by handEngine/battlePerm and funded through the pool.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// cr601Choose answers the pending decision with the options pick selects.
func cr601Choose(t *testing.T, e *Engine, what string, pick func(decision.Option) bool) {
	t.Helper()
	d := e.Pending()
	if d == nil {
		t.Fatalf("%s: no decision pending", what)
	}
	var ch []int
	for _, o := range d.Options {
		if pick(o) {
			ch = append(ch, o.Index)
		}
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: ch}); err != nil {
		t.Fatalf("%s: %v", what, err)
	}
}

// TestCR601_2aDelveCannotExileTheSpellBeingCast: Treasure Cruise cast from
// the graveyard through Kess, Dissident Mage, with {U} floating and seven
// other cards in the graveyard. Treasure Cruise is on the stack (601.2a)
// before delve's exile choice, so delve may offer only the seven OTHER
// cards, and paying all seven leaves Treasure Cruise on the stack.
func TestCR601_2aDelveCannotExileTheSpellBeingCast(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, corpusCardText(t, "t/treasure_cruise.txt")))
	cruise := e.G.Zone(state.ZHand, 0)[0]
	e.emit(events.Event{Kind: events.MoveZone, Obj: cruise, From: state.ZHand, To: state.ZGraveyard})
	for i := 0; i < 7; i++ {
		id := e.G.Zone(state.ZLibrary, 0)[0]
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZGraveyard})
	}
	others := append([]state.ObjID(nil), e.G.Zone(state.ZGraveyard, 0)...)
	battlePerm(t, e, 0, corpusCardText(t, "k/kess_dissident_mage.txt"))
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "U", Amount: 1})
	e.askPriority(0)
	cr601Choose(t, e, "cast Treasure Cruise", func(o decision.Option) bool { return o.Kind == "cast" && o.Obj == cruise })

	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "exile" {
		t.Fatalf("want the delve exile choice, got %+v", d)
	}
	if z := e.G.Obj(cruise).Zone; z != state.ZStack {
		t.Errorf("CR 601.2a: Treasure Cruise is in %s at the delve choice, want the stack", z)
	}
	for _, o := range d.Options {
		if o.Obj == cruise {
			t.Errorf("CR 601.2a/702.66a: delve offers Treasure Cruise itself (option %d); the spell is on the stack, not in the graveyard", o.Index)
		}
	}
	if len(d.Options) != 7 || d.Max != 7 {
		t.Errorf("delve offers %d options (max %d), want the 7 other graveyard cards", len(d.Options), d.Max)
	}
	cr601Choose(t, e, "delve all", func(o decision.Option) bool { return o.Kind == "exile" && o.Obj != cruise })
	if z := e.G.Obj(cruise).Zone; z != state.ZStack {
		t.Fatalf("after paying, Treasure Cruise is in %s, want the stack", z)
	}
	exiled := map[state.ObjID]bool{}
	for _, id := range e.G.Zone(state.ZExile, 0) {
		exiled[id] = true
	}
	for _, id := range others {
		if id != cruise && !exiled[id] {
			t.Errorf("graveyard card %d was not exiled by delve", id)
		}
	}
}

// TestCR601_2aSpellIsOnTheStackAtItsFirstCostChoice: Fling (an additional
// cost of sacrificing a creature) with {R}{R} floating and Grizzly Bears.
// When the first choice of the cast is posed, Fling is already the topmost
// object on the stack.
func TestCR601_2aSpellIsOnTheStackAtItsFirstCostChoice(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, corpusCardText(t, "f/fling.txt")))
	fling := e.G.Zone(state.ZHand, 0)[0]
	bear := battlePerm(t, e, 0, corpusCardText(t, "g/grizzly_bears.txt"))
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "R", Amount: 2})
	e.askPriority(0)
	cr601Choose(t, e, "cast Fling", func(o decision.Option) bool { return o.Kind == "cast" && o.Obj == fling })
	d := e.Pending()
	if d == nil || d.Kind == decision.KPriority {
		t.Fatalf("want Fling's first cast-time choice, got %+v", d)
	}
	if z := e.G.Obj(fling).Zone; z != state.ZStack {
		t.Errorf("CR 601.2a: Fling is in %s at its first choice (%q), want the stack", z, d.Prompt)
	}
	if n := len(e.G.Stack); n == 0 || e.G.Stack[n-1] != fling {
		t.Errorf("CR 601.2a: stack %v at the first choice, want Fling on top", e.G.Stack)
	}
	// Finish the cast: sacrifice the Bear, target the opponent.
	for i := 0; i < 4; i++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			break
		}
		switch d.Kind {
		case decision.KTarget:
			cr601Choose(t, e, "target", func(o decision.Option) bool { return o.Kind == "player" && o.Label == "b" })
		default:
			cr601Choose(t, e, "sacrifice", func(o decision.Option) bool { return o.Obj == bear })
		}
	}
	if z := e.G.Obj(bear).Zone; z != state.ZGraveyard {
		t.Errorf("Grizzly Bears is in %s after the cast, want the graveyard (sacrificed)", z)
	}
	if z := e.G.Obj(fling).Zone; z != state.ZStack {
		t.Errorf("Fling is in %s after the cast, want the stack", z)
	}
}
