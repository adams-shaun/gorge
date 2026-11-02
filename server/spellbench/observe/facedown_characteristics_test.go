package observe_test

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/spellbench/catalog"
	"github.com/adams-shaun/gorge/server/spellbench/gamecfg"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
	"github.com/adams-shaun/gorge/server/spellbench/secrets"
	"github.com/adams-shaun/gorge/server/spellbench/wire"
)

// A face-down permanent is a 2/2 only until something changes that (CR 708.2): an
// opponent reads the counter on it, and no printed fact.
func TestHiddenFaceDownPermanentProjectsItsDerivedCharacteristics(t *testing.T) {
	reg := testcorpus.Registry(t)
	deck := catalog.Deck{CatalogID: "FaceDownCharacteristics", Rows: []wire.DeckRow{{Name: "Serra Angel", Count: 40}}}
	cardsInDeck, err := catalog.Resolve(reg, deck)
	if err != nil {
		t.Fatal(err)
	}
	g, err := gamecfg.New(reg, secrets.NewGame(make([]byte, 32)), [2][]*cards.Card{cardsInDeck, cardsInDeck},
		gamecfg.Rules{Mulligan: "none", StartingSeat: 0})
	if err != nil {
		t.Fatal(err)
	}
	id := g.E.G.Zone(state.ZLibrary, 0)[0]
	events.Emit(g.E.G, g.E.L, events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary,
		To: state.ZBattlefield, Counter: events.FaceDownEntryCounter})
	p := &observe.Projector{R: g.R}
	c := p.Characteristics(1, g.E.G.Obj(id))
	if c == nil || *c.Power != 2 || *c.Toughness != 2 || c.ManaValue != 0 || len(c.Keywords) != 0 {
		t.Fatalf("no counter: %+v", c)
	}
	events.Emit(g.E.G, g.E.L, events.Event{Kind: events.CounterChange, Obj: id, Counter: "P1P1", Amount: 1})
	c = p.Characteristics(1, g.E.G.Obj(id))
	if *c.Power != 3 || *c.Toughness != 3 || c.ManaValue != 0 || len(c.Keywords) != 0 {
		t.Fatalf("with a +1/+1 counter: power %d toughness %d mana value %d keywords %v", *c.Power, *c.Toughness, c.ManaValue, c.Keywords)
	}
}
