package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// "If this would be put into a graveyard from anywhere" reads a surveil's
// graveyard pile as it reads a mill: the card goes back and its owner shuffles
// (CR 614.1a), and the rest of the pile still lands.
func TestSurveilGraveyardPileTakesTheCardsOwnReplacement(t *testing.T) {
	t.Parallel()
	for _, carrier := range []string{"Darksteel Colossus", "Progenitus"} {
		t.Run(carrier, func(t *testing.T) {
			t.Parallel()
			spell := card(t, surveil3Src)
			big := corpusAlternativeCard(t, carrier)
			filler := card(t, "Name:Alpha\nManaCost:no cost\nTypes:Sorcery\nOracle:x\n")
			deck := append([]*cards.Card{spell, big, filler}, mountainDeck(t, 37)...)
			cfg := seatZeroStart(Config{Seed: 7, Names: []string{"a", "b"},
				Decks: [][]*cards.Card{deck, mountainDeck(t, 40)}, Tokens: testutil.CorpusRegistry(t).Tokens})
			e := New(cfg)
			e.Advance()
			toMain1(t, e)
			named := func(name string) state.ObjID {
				t.Helper()
				for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
					for _, id := range e.G.Zone(z, 0) {
						if e.G.Obj(id).Face().Name == name {
							if z == state.ZHand && name != "Surveil3Me" {
								e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZLibrary})
							}
							if z == state.ZLibrary && name == "Surveil3Me" {
								e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZHand})
							}
							return id
						}
					}
				}
				t.Fatalf("%s is not in hand or library", name)
				return 0
			}
			id, bigID, alpha := named("Surveil3Me"), named(carrier), named("Alpha")
			lib := []state.ObjID{bigID, alpha}
			for _, o := range e.G.Zone(state.ZLibrary, 0) {
				if !slices.Contains(lib, o) {
					lib = append(lib, o)
				}
			}
			e.emit(events.Event{Kind: events.LibraryOrder, Player: 0, IDs: lib})
			addMana(t, e, 0, "U")
			d := castFixture(t, e, id, -1)
			if d == nil || len(d.Options) != 3 || d.Options[0].Obj != bigID {
				t.Fatalf("expected the surveil's window with %s first, got %+v", carrier, d)
			}
			libBefore := len(e.G.Zone(state.ZLibrary, 0))
			submitRest(t, e, []int{1, 2}, []int{0})
			if z := e.G.Obj(bigID).Zone; z != state.ZLibrary {
				t.Fatalf("%s is in %s, want the library", carrier, z)
			}
			if got := len(e.G.Zone(state.ZLibrary, 0)); got != libBefore {
				t.Fatalf("library %d cards, want %d", got, libBefore)
			}
			replayCheck(t, e, cfg)
		})
	}
}
