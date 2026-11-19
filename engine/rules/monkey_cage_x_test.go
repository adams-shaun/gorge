package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMonkeyCageMakesTheEnteringCreaturesManaValue pins Monkey Cage's "When
// a creature enters, sacrifice Monkey Cage and create X 2/2 green Monkey
// creature tokens, where X is that creature's mana value": a Grizzly Bears
// (mana value 2) entering makes exactly two Monkeys, never X plus the
// sacrificed Cage's own mana value (the Remembered sacrifice must not join
// TriggeredCard$CardManaCost's sum).
func TestMonkeyCageMakesTheEnteringCreaturesManaValue(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	cage := searchCorpusCard(t, reg, "Monkey Cage")
	bear := searchCorpusCard(t, reg, "Grizzly Bears")
	forest := searchCorpusCard(t, reg, "Forest")
	mountain := searchCorpusCard(t, reg, "Mountain")
	deck := []*cards.Card{cage, bear, bear, bear}
	for i := 0; i < 8; i++ {
		deck = append(deck, forest, mountain)
	}
	for len(deck) < 40 {
		deck = append(deck, bear)
	}
	opp := make([]*cards.Card, 40)
	for i := range opp {
		opp[i] = mountain
	}
	cfg := seatZeroStart(Config{Seed: 9210, Names: []string{"cage", "opponent"},
		Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	cid := searchMoveByName(t, e, "Monkey Cage", state.ZBattlefield)
	bears := handBears(t, e)
	if len(bears) == 0 {
		t.Fatal("precondition: no bear in hand")
	}
	castBearInHand(t, e, bears[0])
	drainStackEmpty(t, e, 20)
	if o := e.G.Obj(cid); o != nil && o.Zone == state.ZBattlefield {
		t.Fatalf("Monkey Cage still on the battlefield")
	}
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.TokenCreate {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("Monkey Cage made %d tokens for a mana value 2 creature, want 2", n)
	}
}
