package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// cantloselife_test.go pins cr-g158: `Mode$ CantLoseLife` delivered by a
// DB$ Effect (Everybody Lives!, Courageous Resolve). Damage is still dealt
// but no life is lost (CR 119.8); a LoseLife effect is swallowed; gain is
// unaffected; a positive life payment is refused.

// cantLoseLifeCast puts the named seat-0 card in hand, funds it and casts it,
// then passes priority until it leaves the stack.
func cantLoseLifeCast(t *testing.T, e *Engine, name, mana string) {
	t.Helper()
	id := moveByName(t, e, 0, name, state.ZHand)
	addMana(t, e, 0, mana)
	idx := castOptionIdx(e.Pending(), "cast", id)
	if idx < 0 {
		t.Fatalf("no cast option for %s: %+v", name, e.Pending())
	}
	submitChoices(t, e, idx)
	for i := 0; i < 20 && e.G.Obj(id).Zone == state.ZStack; i++ {
		d := e.Pending()
		if d.Kind == decision.KPriority {
			passPriorityOnce(t, e)
		} else {
			submitChoices(t, e, 0)
		}
	}
	if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
		t.Fatalf("precondition: %s did not resolve (zone %s)", name, z)
	}
	found := false
	for _, ce := range e.active() {
		if ce.Restriction == "CantLoseLife" {
			found = true
		}
	}
	if !found {
		t.Fatalf("precondition: no CantLoseLife restriction registered after %s resolved", name)
	}
}

var cllCfg Config

// cantLoseLifeEngine is corpusEngine with the Config kept for replayCheck.
func cantLoseLifeEngine(t *testing.T, card string) *Engine {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	m := lookup(t, reg, "Mountain")
	deck := func(extra ...*cards.Card) []*cards.Card {
		d := append([]*cards.Card{}, extra...)
		for len(d) < 40 {
			d = append(d, m)
		}
		return d
	}
	cllCfg = Config{Seed: 42, Names: []string{"a", "b"}, Tokens: reg.Tokens,
		Decks: [][]*cards.Card{deck(lookup(t, reg, card)), deck()}}
	e := New(cllCfg)
	e.Advance()
	toMain1(t, e)
	return e
}

func cantLoseLifeEverybody(t *testing.T) *Engine {
	t.Helper()
	e := cantLoseLifeEngine(t, "Everybody Lives!")
	cantLoseLifeCast(t, e, "Everybody Lives!", "WW")
	return e
}

func cllNotes(e *Engine, text string) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && ev.Text == text {
			n++
		}
	}
	return n
}

func TestCantLoseLifeDamageDealtNoLoss(t *testing.T) {
	e := cantLoseLifeEverybody(t)
	mark := len(e.L.Events)
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 3})
	if got := e.G.Players[1].Life; got != 20 {
		t.Fatalf("seat 1 life = %d, want 20", got)
	}
	if got := e.G.Players[1].LifeLostThisTurn; got != 0 {
		t.Fatalf("LifeLostThisTurn = %d, want 0", got)
	}
	dealt := false
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.Damage && ev.Player == 1 && ev.Amount == 3 {
			dealt = true
		}
	}
	if !dealt {
		t.Fatalf("Damage event missing from log: %+v", e.L.Events[mark:])
	}
	replayCheck(t, e, cllCfg)
}

func TestCantLoseLifeLoseLifeEffect(t *testing.T) {
	e := cantLoseLifeEverybody(t)
	e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: -4})
	if got := e.G.Players[0].Life; got != 20 {
		t.Fatalf("seat 0 life = %d, want 20", got)
	}
	if cllNotes(e, "prevented: can't lose life") != 1 {
		t.Fatal("no 'prevented: can't lose life' note")
	}
	replayCheck(t, e, cllCfg)
}

func TestCantLoseLifeGainStillWorks(t *testing.T) {
	e := cantLoseLifeEverybody(t)
	e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: 3})
	if got := e.G.Players[0].Life; got != 23 {
		t.Fatalf("seat 0 life = %d, want 23", got)
	}
	replayCheck(t, e, cllCfg)
}

func TestCantLoseLifeBlocksLifeCost(t *testing.T) {
	e := cantLoseLifeEverybody(t)
	if !e.playerLifeLossForbidden(0) || !e.playerLifeLossForbidden(1) {
		t.Fatal("precondition: Everybody Lives! scopes both seats")
	}
	id := e.G.Zone(state.ZLibrary, 0)[0]
	two, zero := pay.Cost{Life: 2}, pay.Cost{}
	for p := state.PlayerID(0); p < 2; p++ {
		if pay.CostPayablePool(asPayer(e), p, id, true, two, e.G.Players[p].Pool, e.G.Players[p].ManaUnits()) {
			t.Fatalf("seat %d: PayLife<2> still payable", p)
		}
		if !pay.CostPayablePool(asPayer(e), p, id, true, zero, e.G.Players[p].Pool, e.G.Players[p].ManaUnits()) {
			t.Fatalf("seat %d: a 0-life cost must stay payable", p)
		}
	}
	replayCheck(t, e, cllCfg)
}

func TestCantLoseLifeCourageousResolveScopedToYou(t *testing.T) {
	e := cantLoseLifeEngine(t, "Courageous Resolve")
	e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: -15})
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: -15})
	if e.G.Players[0].Life != 5 {
		t.Fatalf("precondition: seat 0 life = %d, want 5", e.G.Players[0].Life)
	}
	cantLoseLifeCast(t, e, "Courageous Resolve", "WWW")
	if !e.playerLifeLossForbidden(0) || e.playerLifeLossForbidden(1) {
		t.Fatal("precondition: restriction must scope to seat 0 alone")
	}
	e.emit(events.Event{Kind: events.Damage, Player: 0, Amount: 4})
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 4})
	if got := e.G.Players[0].Life; got != 5 {
		t.Fatalf("seat 0 life = %d, want 5", got)
	}
	if got := e.G.Players[1].Life; got != 1 {
		t.Fatalf("seat 1 life = %d, want 1", got)
	}
	replayCheck(t, e, cllCfg)
}

func TestCantLoseLifeExpires(t *testing.T) {
	e := cantLoseLifeEverybody(t)
	driveToStep(t, e, e.G.Turn+1, 1, state.StepMain1)
	e.emit(events.Event{Kind: events.Damage, Player: 0, Amount: 3})
	if got := e.G.Players[0].Life; got != 17 {
		t.Fatalf("next turn: seat 0 life = %d, want 17", got)
	}
	replayCheck(t, e, cllCfg)
}
