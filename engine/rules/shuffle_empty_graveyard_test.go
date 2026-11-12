package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// "Shuffle your graveyard into your library" makes you shuffle whatever it holds, nothing
// included (CR 701.24d, as zone_changeall.go cites it). Forge's Shuffle$ reaches the
// sweep's players whether anything moved (read, not run).
func TestFeldonsCaneShufflesAnEmptyGraveyard(t *testing.T) {
	e := New(Config{Seed: 717, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}})
	e.Advance()
	toMain1(t, e)
	p := e.G.Active
	cane := onBoardReadyCard(t, e, p, corpusAlternativeCard(t, "Feldon's Cane"))
	e.askPriority(p)
	at := len(e.L.Events)
	activated := false
	for _, o := range e.Pending().Options {
		if o.Kind == "ability" && o.Obj == cane {
			submitChoices(t, e, o.Index)
			activated = true
			break
		}
	}
	if !activated {
		t.Fatal("Feldon's Cane offers no activation")
	}
	passUntilStackEmpty(t, e, 8)
	n := 0
	for _, ev := range e.L.Events[at:] {
		if ev.Kind == events.Shuffle && ev.Player == p {
			n++
		}
	}
	if n != 1 || len(e.G.Zone(state.ZGraveyard, p)) != 0 {
		t.Fatalf("%d shuffles, graveyard %v; want one shuffle of an empty graveyard", n, e.G.Zone(state.ZGraveyard, p))
	}
}
