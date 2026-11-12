package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Random$ on an as-enters ChooseNumber (Haktos the Unscarred: "As Haktos
// enters, choose 2, 3, or 4 at random") is the engine's own draw, not a
// question to the controller. The entry must record exactly one Choose
// "number" event with a value inside the body's own Min$/Max$ bounds, and the
// draw has to come from the seeded rng rather than option 0.

// chooseNumberBody returns the ChooseNumber body on obj's face, failing the
// test when the fixture has none (every assertion below depends on the body
// the entry walker actually reads).
func chooseNumberBody(t *testing.T, e *Engine, id state.ObjID) *cards.SA {
	t.Helper()
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		t.Fatalf("precondition: object %d has no face", id)
	}
	for i := range o.Face().Repls {
		if r := &o.Face().Repls[i]; r.With != nil && r.With.API == "ChooseNumber" {
			return r.With
		}
	}
	t.Fatalf("precondition: %s has no ChooseNumber replacement body", o.Face().Name)
	return nil
}

// chooseNumberEventsFor counts the Choose "number" events recorded on id --
// the one event an answered (or engine-drawn) ChooseNumber folds.
func chooseNumberEventsFor(e *Engine, id state.ObjID) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.Choose && ev.Obj == id && ev.Counter == "number" {
			n++
		}
	}
	return n
}

// drainETBRandomEntry passes priority until id settles on the battlefield,
// failing if the entry poses any as-enters ask. A Random$ True ChooseNumber
// must never produce a Decision with ResumeKind "etb".
func drainETBRandomEntry(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	for i := 0; i < 80 && !e.G.Over; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while draining the random ChooseNumber entry")
		}
		if d.Kind == decision.KChoose && d.ResumeKind == "etb" {
			t.Fatalf("Random$ True ChooseNumber posed an as-enters ask: %+v", d)
		}
		if o := e.G.Obj(id); o != nil && o.Zone == state.ZBattlefield && len(e.G.Stack) == 0 {
			return
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("unexpected non-priority decision while draining the entry: %+v", d)
		}
		pass := -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				pass = o.Index
			}
		}
		if pass < 0 {
			t.Fatalf("priority decision with no pass option: %+v", d)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{pass}}); err != nil {
			t.Fatalf("submit pass: %v", err)
		}
	}
	t.Fatalf("entry for object %d did not settle on the battlefield", id)
}

func TestETBChooseNumberRandomPosesNoAsk(t *testing.T) {
	t.Parallel()
	seen := map[int32]bool{}
	for seed := uint64(1); seed <= 8; seed++ {
		e, _, id := newFixtureDeck(t, seed, talionLike("Min$ 2 | Max$ 4 | Random$ True"))
		body := chooseNumberBody(t, e, id)
		if !effects.ChooseNumberAtRandom(body) {
			t.Fatalf("seed %d precondition: ChooseNumber body does not carry Random$ True: %+v", seed, body.Params)
		}
		addMana(t, e, 0, "WWUUBB")
		castFirst(t, e, "cast")
		drainETBRandomEntry(t, e, id)

		o := e.G.Obj(id)
		if o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("seed %d: entrant is not on the battlefield: %+v", seed, o)
		}
		if o.ChosenNumber < 2 || o.ChosenNumber > 4 {
			t.Fatalf("seed %d: ChosenNumber = %d, want one of the body's 2, 3, 4", seed, o.ChosenNumber)
		}
		if n := chooseNumberEventsFor(e, id); n != 1 {
			t.Fatalf("seed %d: %d Choose number events for the entrant, want exactly 1", seed, n)
		}
		seen[o.ChosenNumber] = true
	}
	if len(seen) < 2 {
		t.Fatalf("8 seeds recorded only ChosenNumber %v: the draw is not using the seeded rng", seen)
	}
}

func TestETBChooseNumberRandomHaktosCorpus(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	haktos := lookup(t, reg, "Haktos the Unscarred")
	e := corpusEngine(t, reg, []*cards.Card{haktos}, nil)
	id := findByName(e, haktos.Faces[0].Name, 0)
	if id == 0 {
		t.Fatal("precondition: Haktos the Unscarred is not in seat 0's hand or library")
	}
	if o := e.G.Obj(id); o.Zone != state.ZHand {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone, To: state.ZHand})
	}
	body := chooseNumberBody(t, e, id)
	if !effects.ChooseNumberAtRandom(body) {
		t.Fatalf("precondition: Haktos ChooseNum body does not carry Random$ True: %+v", body.Params)
	}
	if got := body.ParamStr(cards.PKMin); got != "2" {
		t.Fatalf("precondition: Haktos Min$ = %q, want 2", got)
	}
	if got := body.ParamStr(cards.PKMax); got != "4" {
		t.Fatalf("precondition: Haktos Max$ = %q, want 4", got)
	}
	addMana(t, e, 0, "RRWW")
	castFirst(t, e, "cast")
	drainETBRandomEntry(t, e, id)

	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Haktos is not on the battlefield: %+v", o)
	}
	if o.ChosenNumber < 2 || o.ChosenNumber > 4 {
		t.Fatalf("Haktos ChosenNumber = %d, want one of 2, 3, 4", o.ChosenNumber)
	}
	if n := chooseNumberEventsFor(e, id); n != 1 {
		t.Fatalf("%d Choose number events for Haktos, want exactly 1", n)
	}
}
