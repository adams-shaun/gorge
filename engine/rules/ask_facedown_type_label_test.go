package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 708.5 hides a face-down permanent's NAME, which is the card's identity
// as an object. A ChooseType ask's option labels are type WORDS from a fixed
// vocabulary (Mystic Compass's Basic Land list); the word "Swamp" there is
// the land type, not the face-down card, so hiding it would both narrow the
// legal answers below what the rules offer and tell the seat asked that a
// face-down Swamp exists. An option that names the face-down object still
// reads "a face-down permanent".
func TestChooseTypeMenuKeepsTheBasicLandWordsBehindAFaceDownCard(t *testing.T) {
	c, diags := cards.ParseBytes("swamp.txt", []byte("Name:Swamp\nTypes:Basic Land Swamp\nOracle:x\n"))
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
	o := e.G.Obj(id)
	if !o.FaceDown || o.Face() == nil || o.Face().Name != "Swamp" {
		t.Fatal("precondition: the face-down permanent is not a Swamp")
	}

	d := &decision.Decision{Player: 1, Kind: decision.KChoose, Prompt: "Choose a land type", Options: []decision.Option{
		{Index: 0, Kind: "type", Label: "Forest"},
		{Index: 1, Kind: "type", Label: "Island"},
		{Index: 2, Kind: "type", Label: "Mountain"},
		{Index: 3, Kind: "type", Label: "Plains"},
		{Index: 4, Kind: "type", Label: "Swamp"},
		{Index: 5, Kind: "tap", Label: "Tap Swamp", Obj: id},
	}}
	e.hideFaceDownNames(d)
	for i, want := range []string{"Forest", "Island", "Mountain", "Plains", "Swamp"} {
		if got := d.Options[i].Label; got != want {
			t.Errorf("type option %d reads %q, want %q", i, got, want)
		}
	}
	if got := d.Options[5].Label; got != "Tap a face-down permanent" {
		t.Errorf("the option naming the object reads %q", got)
	}

	d0 := &decision.Decision{Player: 0, Kind: decision.KChoose, Prompt: "Choose a land type", Options: []decision.Option{
		{Index: 0, Kind: "type", Label: "Swamp"},
		{Index: 1, Kind: "tap", Label: "Tap Swamp", Obj: id},
	}}
	e.hideFaceDownNames(d0)
	if got := d0.Options[0].Label; got != "Swamp" {
		t.Errorf("the controller's type option reads %q", got)
	}
	if got := d0.Options[1].Label; got != "Tap Swamp" {
		t.Errorf("the controller's object option reads %q", got)
	}
}
