package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// qEnteredEngine puts Q, Stacking the Deck onto seat 0's battlefield, resolves
// its enters trigger and returns the engine with the card it exiled and
// granted its ability to.
func qEnteredEngine(t *testing.T) (*Engine, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	extras := []*cards.Card{lookup(t, reg, "Q, Stacking the Deck")}
	for i := 0; i < 8; i++ {
		extras = append(extras, lookup(t, reg, "Hill Giant"))
	}
	e, _ := corpusEngineCfg(t, reg, extras, nil)
	moveByName(t, e, 0, "Q, Stacking the Deck", state.ZBattlefield)
	e.priorityRound()
	passUntilStackEmpty(t, e, 20)
	var found state.ObjID
	for _, id := range e.G.Zone(state.ZExile, 0) {
		if f := e.G.Obj(id).Face(); f != nil && !f.IsLand() {
			found = id
		}
	}
	if found == 0 {
		t.Fatal("precondition: Q's enters trigger exiled no nonland card")
	}
	e.priorityRound()
	return e, found
}

// TestExileGrantedAbilityOffered: the nonland card Q exiles gains "Exile a
// card from your hand: Create a copy of this card. You may cast the copy.
// Activate only during your turn and only if this card is exiled." The grant
// is offered to the card's owner on their turn, costs a hand card, casts a
// copy and leaves the original exiled; once the card leaves exile the grant
// is gone.
func TestExileGrantedAbilityOffered(t *testing.T) {
	e, found := qEnteredEngine(t)
	if e.G.Active != 0 {
		t.Fatalf("precondition: seat %d is active, want the owner (seat 0)", e.G.Active)
	}
	opt := exileGrantOption(t, e, found)
	if opt.GrantSource == 0 {
		t.Fatalf("granted option %+v: want one naming its grantor", opt)
	}
	hand := len(e.G.Zone(state.ZHand, 0))
	if hand == 0 {
		t.Fatal("precondition: the owner has no hand card to exile")
	}
	addMana(t, e, 0, "RRRR")
	submitChoices(t, e, exileGrantOption(t, e, found).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) != hand || d.Options[0].Kind != "exilecost" {
		t.Fatalf("the cost asks for one of the %d hand cards, got %+v", hand, d)
	}
	submitChoices(t, e, 0)
	if got := len(e.G.Zone(state.ZHand, 0)); got != hand-1 {
		t.Fatalf("hand = %d after the cost, want %d", got, hand-1)
	}
	// resolve the ability: the Play ask, then the copy is cast and resolves
	for i := 0; i < 20 && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		switch {
		case d == nil:
			t.Fatal("no decision while the stack is not empty")
		case d.Kind == decision.KPriority:
			submitChoices(t, e, len(d.Options)-2)
		default:
			submitChoices(t, e, 0)
		}
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack still holds %d objects", len(e.G.Stack))
	}
	if e.G.Obj(found).Zone != state.ZExile {
		t.Fatalf("the original is in %s, want exile", e.G.Obj(found).Zone)
	}
	copies := 0
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o.IsToken && o.Face().Name == "Hill Giant" {
			copies++
		}
	}
	if copies != 1 {
		t.Fatalf("%d Hill Giant copies on the battlefield, want 1", copies)
	}
	// the grant lasts only while the card is exiled
	e.emit(events.Event{Kind: events.MoveZone, Obj: found, From: state.ZExile, To: state.ZHand})
	e.priorityRound()
	if got := exileGrantOptions(e, found); len(got) != 0 {
		t.Fatalf("the ability is still offered after the card left exile: %+v", got)
	}
}

// exileGrantOptions are the pending priority options that activate the
// granted ABPlay of the exiled card id.
func exileGrantOptions(e *Engine, id state.ObjID) []decision.Option {
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		return nil
	}
	var out []decision.Option
	for _, opt := range d.Options {
		if opt.Kind == "ability" && opt.Obj == id && opt.SVar == "ABPlay" {
			out = append(out, opt)
		}
	}
	return out
}

func exileGrantOption(t *testing.T, e *Engine, id state.ObjID) decision.Option {
	t.Helper()
	got := exileGrantOptions(e, id)
	if len(got) != 1 {
		t.Fatalf("%d granted options for obj %d, want exactly one; pending=%+v", len(got), id, e.Pending())
	}
	return got[0]
}
