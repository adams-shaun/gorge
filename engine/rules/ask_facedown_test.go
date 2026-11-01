package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Text built for a decision comes from Face().Name wherever it is built; only
// the seat controlling a face-down permanent reads its name there (CR 708.5).
func TestDecisionTextHidesAFaceDownPermanentFromOtherSeats(t *testing.T) {
	c, diags := cards.ParseBytes("bear.txt", []byte("Name:Hidden Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"))
	if len(diags) != 0 {
		t.Fatal(diags)
	}
	c.Link()
	deck := make([]*cards.Card, 40)
	for i := range deck {
		deck[i] = c
	}
	e := New(Config{Seed: 1, Names: []string{"a", "b"}, Decks: [][]*cards.Card{deck, deck}})
	id := e.G.Zone(state.ZLibrary, 0)[0]
	events.Emit(e.G, e.L, events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield, Counter: events.FaceDownEntryCounter})
	if !e.G.Obj(id).FaceDown {
		t.Fatal("precondition: not face down")
	}

	own := e.G.Zone(state.ZLibrary, 1)[0]
	asked := func(seat state.PlayerID) *decision.Decision {
		d := &decision.Decision{Player: seat, Kind: decision.KChoose, Prompt: "Tap or untap Hidden Bear?", Options: []decision.Option{
			{Kind: "tap", Label: "Tap Hidden Bear", Obj: id},
			{Kind: "block", Label: "Hidden Bear blocks Hidden Bear", Obj: own, Attacker: id},
			{Kind: "target", Label: "Hidden Bear", Obj: own},
		}}
		e.hideFaceDownNames(d)
		return d
	}
	d := asked(1)
	if d.Prompt != "Tap or untap a face-down permanent?" || d.Options[0].Label != "Tap a face-down permanent" {
		t.Errorf("opponent reads %q, %q", d.Prompt, d.Options[0].Label)
	}
	if got := d.Options[1].Label; got != "a face-down permanent blocks a face-down permanent" {
		t.Errorf("the option about the attacker reads %q", got)
	}
	if got := d.Options[2].Label; got != "Hidden Bear" {
		t.Errorf("the seat's own card of that name reads %q", got)
	}
	if d := asked(0); d.Prompt != "Tap or untap Hidden Bear?" || d.Options[1].Label != "Hidden Bear blocks Hidden Bear" {
		t.Errorf("controller reads %q, %q", d.Prompt, d.Options[1].Label)
	}
}
