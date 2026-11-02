package botpolicy_test

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// landStayGame plays two bots on the same deck for a fixed number of decisions
// and returns the engine.
func landStayGame(t *testing.T, names ...string) *rules.Engine {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	var deck []*cards.Card
	for len(deck) < 60 {
		for _, n := range names {
			c, ok := reg.Lookup(n)
			if !ok {
				t.Fatalf("no %s in the corpus", n)
			}
			deck = append(deck, c)
		}
	}
	e := rules.New(rules.Config{Seed: 9, Names: []string{"a", "b"}, Decks: [][]*cards.Card{deck, deck}, Tokens: reg.Tokens})
	e.Advance()
	rng := rand.New(rand.NewPCG(9, 0))
	board := botpolicy.NewBoard(2)
	for range 400 {
		d := e.Pending()
		if d == nil || e.G.Over {
			break
		}
		if err := e.Submit(botpolicy.Decide(botpolicy.BoardFromGameInto(e.G, e, d.Player, &board), d, rng)); err != nil {
			t.Fatal(err)
		}
	}
	return e
}

// landPlays counts the land plays of the named land, or of every land for "".
func landPlays(e *rules.Engine, name string) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.MoveZone && ev.From == state.ZHand && ev.To == state.ZBattlefield && (name == "" || e.G.Obj(ev.Obj).Face().Name == name) {
			n++
		}
	}
	return n
}

// controlled counts the named permanents p controls.
func controlled(e *rules.Engine, p state.PlayerID, name string) int {
	n := 0
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			n++
		}
	}
	return n
}

// A land that would have to return itself, or be sacrificed, is held when it is
// all the seat has.
func TestBotHoldsALandThatWouldNotStay(t *testing.T) {
	for _, land := range []string{"Selesnya Sanctuary", "Arid Archway", "Crosis's Catacombs", "Coral Atoll"} {
		t.Run(land, func(t *testing.T) {
			if n := landPlays(landStayGame(t, land), ""); n != 0 {
				t.Fatalf("%d land plays with nothing to return", n)
			}
		})
	}
}

// The basic goes first and the bounce land then stays, so the seat develops
// mana instead of returning the bounce land to hand every turn.
func TestBotPlaysABasicBeforeABounceLand(t *testing.T) {
	e := landStayGame(t, "Selesnya Sanctuary", "Selesnya Sanctuary", "Selesnya Sanctuary", "Selesnya Sanctuary", "Forest")
	stayed := 0
	for p := range state.PlayerID(2) {
		stayed += controlled(e, p, "Selesnya Sanctuary")
		if controlled(e, p, "Selesnya Sanctuary")+controlled(e, p, "Forest") == 0 {
			t.Errorf("seat %d holds no land", p)
		}
	}
	if stayed == 0 {
		t.Error("no Selesnya Sanctuary stayed on the battlefield")
	}
}

// A sacrifice-unless land waits for the land it names.
func TestBotWaitsForTheLandASacrificeUnlessLandNames(t *testing.T) {
	e := landStayGame(t, "Coral Atoll", "Forest")
	if landPlays(e, "Forest") == 0 {
		t.Fatal("precondition: no Forest was played")
	}
	if n := landPlays(e, "Coral Atoll"); n != 0 {
		t.Fatalf("%d Coral Atolls played with no Island to return", n)
	}
	e = landStayGame(t, "Coral Atoll", "Island")
	if landPlays(e, "Coral Atoll") == 0 {
		t.Fatal("no Coral Atoll played beside an Island")
	}
	e = landStayGame(t, "Crosis's Catacombs", "Forest")
	for p := range state.PlayerID(2) {
		if controlled(e, p, "Crosis's Catacombs") == 0 {
			t.Errorf("seat %d kept no Catacombs beside a Forest", p)
		}
	}
}
