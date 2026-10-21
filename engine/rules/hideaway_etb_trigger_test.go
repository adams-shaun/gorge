package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestHideawayETBTriggerClivesHideaway(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	clive, ok := reg.Lookup("Clive's Hideaway")
	if !ok {
		t.Fatal("Clive's Hideaway missing from corpus")
	}
	if d := clive.Link(); len(d) != 0 {
		t.Fatalf("link Clive's Hideaway: %v", d)
	}
	deck := make([]*cards.Card, 40)
	for i := range deck {
		deck[i] = clive
	}
	e := New(seatZeroStart(Config{Seed: 1881, Names: []string{"clive", "other"}, Decks: [][]*cards.Card{deck, deck}}))
	land := e.G.Objs[0].ID
	if o := e.G.Obj(land); o == nil || o.Zone != state.ZLibrary {
		t.Fatalf("precondition: Clive's Hideaway = %+v, want library", o)
	}
	beforeLibrary := len(e.G.Zone(state.ZLibrary, 0))
	if beforeLibrary < 4 {
		t.Fatalf("precondition: library has %d cards, need at least Hideaway's four", beforeLibrary)
	}

	e.emit(events.Event{Kind: events.MoveZone, Obj: land, From: state.ZLibrary, To: state.ZBattlefield})
	e.putTriggersOnStack()
	if o := e.G.Obj(land); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Clive's Hideaway did not enter before its trigger: %+v", o)
	}
	if got := len(e.G.Zone(state.ZLibrary, 0)); got != beforeLibrary-1 {
		t.Fatalf("library after land entry = %d, want %d", got, beforeLibrary-1)
	}
	if got := len(e.G.Zone(state.ZExile, 0)); got != 0 {
		t.Fatalf("precondition: exile before Hideaway resolves has %d cards, want 0", got)
	}
	if len(e.G.Stack) != 1 {
		t.Fatalf("Clive's Hideaway ETB stack = %v, want one trigger", e.G.Stack)
	}

	kr6ResolveTop(e)
	pick := e.Pending()
	if pick == nil || pick.Kind != decision.KChoose || len(pick.Options) != 4 {
		t.Fatalf("Clive's Hideaway pick = %+v, want four-card choice after resolving ETB trigger", pick)
	}
	chosen := pick.Options[1].Obj
	if err := e.Submit(decision.Intent{Seq: pick.Seq, Player: pick.Player, Choices: []int{pick.Options[1].Index}}); err != nil {
		t.Fatalf("submit Hideaway pick: %v", err)
	}
	rest := map[state.ObjID]bool{}
	for i, o := range pick.Options {
		if i != 1 {
			rest[o.Obj] = true
		}
	}
	// CR 702.75a: the rest go to the bottom in a random order; nothing is asked.
	if bottom := e.Pending(); bottom != nil && bottom.Kind == decision.KArrange {
		t.Fatalf("Clive's Hideaway asked for a bottom order %+v; CR 702.75a randomises it", bottom)
	}
	if got := e.G.Zone(state.ZExile, 0); len(got) != 1 || got[0] != chosen {
		t.Fatalf("Clive's Hideaway exile = %v, want chosen card %d", got, chosen)
	}
	if o := e.G.Obj(chosen); o == nil || !o.FaceDown || o.ExiledWith != land {
		t.Fatalf("chosen Hideaway card = %+v, want face down with provenance %d", o, land)
	}
	library := e.G.Zone(state.ZLibrary, 0)
	for i, id := range library[len(library)-len(rest):] {
		if !rest[id] {
			t.Fatalf("Clive's Hideaway bottom[%d] = %d, want one of the three unexiled looked-at cards", i, id)
		}
	}
}
