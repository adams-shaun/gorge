package botpolicy_test

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestBotConvokeCastsAreNotStranded plays a convoke X spell's deck to the end of
// its games: the bot's announcements leave every cast payable, so none reverses.
func TestBotConvokeCastsAreNotStranded(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	var deck []*cards.Card
	for _, n := range []struct {
		name  string
		count int
	}{{"Plains", 12}, {"Mountain", 12}, {"Grizzly Bears", 20}, {"Wildfire Awakener", 16}} {
		c, ok := reg.Lookup(n.name)
		if !ok {
			t.Fatalf("no %s in the corpus", n.name)
		}
		for range n.count {
			deck = append(deck, c)
		}
	}
	casts := 0
	for game := range uint64(6) {
		e := rules.New(rules.Config{Seed: 5000 + game, Names: []string{"a", "b"}, Decks: [][]*cards.Card{deck, deck}, Tokens: reg.Tokens})
		e.Advance()
		rngs := []*rand.Rand{rand.New(rand.NewPCG(game, 0)), rand.New(rand.NewPCG(game, 1))}
		board := botpolicy.NewBoard(2)
		for i := 0; i < 3000 && !e.G.Over; i++ {
			d := e.Pending()
			if d == nil {
				break
			}
			in := botpolicy.Decide(botpolicy.BoardFromGameInto(e.G, e, d.Player, &board), d, rngs[d.Player])
			if err := e.Submit(in); err != nil {
				t.Fatal(err)
			}
		}
		for _, ev := range e.L.Events {
			switch ev.Kind {
			case events.SpellCastReversed:
				t.Fatalf("game %d: a cast reversed: %+v", game, ev)
			case events.PutOnStack:
				casts++
			}
		}
	}
	if casts == 0 {
		t.Fatal("precondition: no spell was cast")
	}
}
