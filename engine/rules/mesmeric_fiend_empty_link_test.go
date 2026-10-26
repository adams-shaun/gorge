package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// fiendDrive passes priority until the stack empties, answering every other
// ask with the option that targets/names want (the first option otherwise).
func fiendDrive(t *testing.T, e *Engine, want state.ObjID) {
	t.Helper()
	for i := 0; i < 40; i++ {
		d := passUntilResolved(t, e, 20)
		if d.Kind == decision.KPriority {
			return
		}
		idx := d.Options[0].Index
		for _, o := range d.Options {
			if want != 0 && o.Obj == want {
				idx = o.Index
			}
		}
		submitChoices(t, e, idx)
	}
	t.Fatal("resolution never settled")
}

// fiendPair deals seat 0 a Mesmeric Fiend and a Journey to Nowhere, and seat 1
// a hand of Mountains plus, with withCard, a Grizzly Bears the Fiend can take.
// It returns the Fiend and the exiled-card candidate (0 when none).
func fiendPair(t *testing.T, withCard bool) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	reg := searchTestRegistry(t)
	plains := searchCorpusCard(t, reg, "Plains")
	swamp := searchCorpusCard(t, reg, "Swamp")
	mountain := searchCorpusCard(t, reg, "Mountain")
	deck := []*cards.Card{searchCorpusCard(t, reg, "Mesmeric Fiend"), searchCorpusCard(t, reg, "Journey to Nowhere")}
	for len(deck) < 40 {
		deck = append(deck, plains, swamp)
	}
	opp := []*cards.Card{searchCorpusCard(t, reg, "Grizzly Bears")}
	for len(opp) < 40 {
		opp = append(opp, mountain)
	}
	e := New(seatZeroStart(Config{Seed: 4417, Names: []string{"fiend", "opponent"},
		Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens}))
	e.Advance()
	toMain1(t, e)
	var bear state.ObjID
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range e.G.Zone(z, 1) {
			o := e.G.Obj(id)
			if o == nil || o.Face() == nil || o.Face().Name != "Grizzly Bears" || bear != 0 {
				continue
			}
			bear = id
			if z != state.ZHand && withCard {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: z, To: state.ZHand})
			}
			if z == state.ZHand && !withCard {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: z, To: state.ZLibrary})
			}
		}
	}
	e.pending = nil
	e.priorityRound()
	if bear == 0 {
		t.Fatal("no Grizzly Bears staged")
	}
	hasBear := e.G.Obj(bear).Zone == state.ZHand
	if hasBear != withCard {
		t.Fatalf("precondition: opponent's hand holds the nonland card = %v, want %v", hasBear, withCard)
	}
	fiend := searchMoveByName(t, e, "Mesmeric Fiend", state.ZHand)
	addMana(t, e, 0, "1B")
	castByID(t, e, fiend)
	fiendDrive(t, e, bear)
	return e, fiend, bear
}

func fiendJourneyExilesFiend(t *testing.T, e *Engine, fiend state.ObjID) {
	t.Helper()
	if o := e.G.Obj(fiend); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Fiend = %+v, want battlefield", o)
	}
	jn := searchMoveByName(t, e, "Journey to Nowhere", state.ZHand)
	addMana(t, e, 0, "1W")
	castByID(t, e, jn)
	fiendDrive(t, e, fiend)
}

// movedToHand lists the log's MoveZone events into seat 0's or 1's hand
// after seq start.
func fiendMovesToHand(e *Engine, start int) []events.Event {
	var out []events.Event
	for _, ev := range e.L.Events[start:] {
		if ev.Kind == events.MoveZone && ev.To == state.ZHand {
			out = append(out, ev)
		}
	}
	return out
}

// TestMesmericFiendEmptyLinkReturnsNothing pins CR 607.2a: the Fiend's leave
// trigger returns only what its enters trigger exiled. With nothing exiled,
// Journey to Nowhere exiling the Fiend leaves the Fiend in exile -- the
// trigger's own capture (the leaving Fiend) is not a remembered card.
func TestMesmericFiendEmptyLinkReturnsNothing(t *testing.T) {
	t.Parallel()
	e, fiend, _ := fiendPair(t, false)
	start := len(e.L.Events)
	fiendJourneyExilesFiend(t, e, fiend)
	if o := e.G.Obj(fiend); o == nil || o.Zone != state.ZExile {
		t.Fatalf("Fiend = %+v, want exile (precondition: Journey exiled it)", o)
	}
	resolved := false
	for _, ev := range e.L.Events[start:] {
		if ev.Kind == events.Resolve {
			if o := e.G.Obj(ev.Obj); o != nil && o.Source == fiend && o.Ability != nil && o.Ability.API == "ChangeZone" {
				resolved = true
			}
		}
	}
	if !resolved {
		t.Fatal("precondition: the Fiend's leave trigger never resolved")
	}
	if mv := fiendMovesToHand(e, start); len(mv) != 0 {
		t.Fatalf("ChangeZone to hand emitted with nothing exiled: %+v", mv)
	}
}

// TestMesmericFiendLinkedCardReturns is the control: the enters trigger exiled
// the opponent's Grizzly Bears, and the leave trigger returns that card.
func TestMesmericFiendLinkedCardReturns(t *testing.T) {
	t.Parallel()
	e, fiend, bear := fiendPair(t, true)
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZExile {
		t.Fatalf("precondition: bear = %+v, want exile", o)
	}
	fiendJourneyExilesFiend(t, e, fiend)
	if o := e.G.Obj(fiend); o == nil || o.Zone != state.ZExile {
		t.Fatalf("Fiend = %+v, want exile", o)
	}
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZHand {
		t.Fatalf("bear = %+v, want back in hand", o)
	}
}
