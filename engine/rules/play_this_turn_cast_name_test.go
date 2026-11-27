package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPlayThisTurnCastNameOffersSameName pins Twinning Glass's
// `Valid$ Card.nonLand+sharesNameWith ThisTurnCast` population: after one
// Grizzly Bears is cast this turn, the Glass's {1},{T} Play offers the second
// Grizzly Bears in hand, and never the Lightning Bolt (a nonland card whose
// name no spell cast this turn has).
func TestPlayThisTurnCastNameOffersSameName(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	glass := lookup(t, reg, "Twinning Glass")
	bear := lookup(t, reg, "Grizzly Bears")
	bolt := lookup(t, reg, "Lightning Bolt")
	mountain := lookup(t, reg, "Mountain")
	deck0 := []*cards.Card{glass, bear, bear, bolt}
	for len(deck0) < 40 {
		deck0 = append(deck0, mountain)
	}
	deck1 := make([]*cards.Card, 40)
	for i := range deck1 {
		deck1[i] = mountain
	}
	cfg := seatZeroStart(Config{Seed: 7, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{deck0, deck1}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	driveToStep(t, e, 3, 0, state.StepMain1)
	glassID := pmcMoveTo(t, e, 0, "Twinning Glass", state.ZBattlefield)
	cast := pmcMoveTo(t, e, 0, "Grizzly Bears", state.ZHand)
	boltID := pmcMoveTo(t, e, 0, "Lightning Bolt", state.ZHand)
	var held state.ObjID
	for _, id := range e.G.Zone(state.ZLibrary, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Grizzly Bears" {
			held = id
		}
	}
	if held == 0 {
		t.Fatal("precondition: the second Grizzly Bears is not in the library")
	}
	addMana(t, e, 0, "GGG")

	d := e.Pending()
	idx := castOptionIdx(d, "cast", cast)
	if idx < 0 {
		t.Fatalf("the first Grizzly Bears is not castable: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	for i := 0; len(e.G.Stack) > 0; i++ {
		if i > 20 {
			t.Fatal("the Grizzly Bears never resolved")
		}
		submitPass(t, e)
	}
	if o := e.G.Obj(cast); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("the cast Grizzly Bears rests in %v, want the battlefield", o)
	}
	// The second Bears enters the hand only now, so it was never castable
	// alongside the first and the cast ledger holds exactly one Bears.
	pmcMoveTo(t, e, 0, "Grizzly Bears", state.ZHand)
	if o := e.G.Obj(held); o == nil || o.Zone != state.ZHand {
		t.Fatalf("precondition: the held Grizzly Bears is in %v, want hand", o)
	}
	for _, id := range e.G.Zone(state.ZHand, 0) {
		o := e.G.Obj(id)
		if id != held && o.Face().Name == "Grizzly Bears" {
			t.Fatalf("precondition: a second same-name hand card %d", id)
		}
	}
	if e.G.Obj(boltID).Face().Name == e.G.Obj(held).Face().Name {
		t.Fatal("precondition: the Bolt and the Bears share a name")
	}

	e.priorityRound()
	d = e.Pending()
	idx = castOptionIdx(d, "ability", glassID)
	if idx < 0 {
		t.Fatalf("Twinning Glass's ability is not offered: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	var play *decision.Decision
	for i := 0; i < 20 && play == nil; i++ {
		d = e.Pending()
		switch {
		case d == nil:
			t.Fatal("no decision pending while resolving the Glass")
		case d.Kind == decision.KModes && d.ResumeKind == "play":
			play = d
		case d.Kind == decision.KPriority && len(e.G.Stack) > 0:
			submitPass(t, e)
		default:
			t.Fatalf("unexpected decision kind=%v resume=%q; notes: Play found nothing=%v",
				d.Kind, d.ResumeKind, hasNote(e, "Play found no card to play"))
		}
	}
	if play == nil {
		t.Fatalf("the Play ask was never posed (found nothing=%v)", hasNote(e, "Play found no card to play"))
	}
	if len(play.Options) != 1 || play.Options[0].Obj != held {
		t.Fatalf("Play offers %+v, want only the held Grizzly Bears %d", play.Options, held)
	}
}
