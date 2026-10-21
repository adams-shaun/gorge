package rules

// Hideaway's remainder (cr-g63). CR 702.75a: "look at the top N cards of
// your library. Exile one of them face down and put the rest on the bottom
// of your library in a random order." The order is the engine's seeded
// draw, never a player's arrangement.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCRHideawayRestGoesToBottomInRandomOrder: Clive's Hideaway (Hideaway
// 4) resolves; after the exile pick no order is asked, the three other
// looked-at cards are the library's bottom three, and the game replays.
func TestCRHideawayRestGoesToBottomInRandomOrder(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	clive, ok := reg.Lookup("Clive's Hideaway")
	if !ok {
		t.Fatal("Clive's Hideaway missing from corpus")
	}
	deck := make([]*cards.Card, 40)
	for i := range deck {
		deck[i] = clive
	}
	cfg := seatZeroStart(Config{Seed: 818, Names: []string{"clive", "other"}, Decks: [][]*cards.Card{deck, deck}})
	e := New(cfg)
	id := e.G.Objs[0].ID
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield})
	e.putTriggersOnStack()
	kr6ResolveTop(e)
	pick := e.Pending()
	if pick == nil || pick.Kind != decision.KChoose || len(pick.Options) != 4 {
		t.Fatalf("Hideaway pick = %+v, want the four-card exile choice", pick)
	}
	rest := map[state.ObjID]bool{}
	for i, o := range pick.Options {
		if i != 1 {
			rest[o.Obj] = true
		}
	}
	if err := e.Submit(decision.Intent{Seq: pick.Seq, Player: pick.Player, Choices: []int{pick.Options[1].Index}}); err != nil {
		t.Fatalf("submit Hideaway choice: %v", err)
	}
	if d := e.Pending(); d != nil && d.Kind == decision.KArrange {
		t.Fatalf("CR 702.75a: Hideaway asked seat %d to order the rest (%q); the rest go to the bottom in a random order", d.Player, d.Prompt)
	}
	lib := e.G.Zone(state.ZLibrary, 0)
	if len(lib) < 3 {
		t.Fatalf("library = %d cards, want at least the three bottomed", len(lib))
	}
	for _, b := range lib[len(lib)-3:] {
		if !rest[b] {
			t.Fatalf("library bottom three = %v, want the three unexiled looked-at cards %v", lib[len(lib)-3:], rest)
		}
	}
	replayCheck(t, e, cfg)
}
