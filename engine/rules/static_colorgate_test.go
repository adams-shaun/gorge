package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Knight of Grace's gate reads layer 5: a creature an Aura made black turns the
// +1/+0 on, and the only black creature an Aura recoloured turns it off. The
// bonus follows the Aura off the battlefield.
func TestContinuousColorGateReadsDerivedColors(t *testing.T) {
	t.Parallel()
	knight := corpusAlternativeCard(t, "Knight of Grace")
	cfg := seatZeroStart(Config{Seed: 7, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}, Tokens: testutil.CorpusRegistry(t).Tokens})
	power := func(e *Engine, id state.ObjID) int32 { return e.Derived(id).Power }
	for _, c := range []struct {
		name, creature, aura string
		before, with         int32
	}{
		{"made black", "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n", "Eaten by Piranhas", 2, 3},
		{"no longer black", "Name:Rat\nManaCost:1 B\nTypes:Creature Rat\nPT:2/2\nOracle:x\n", "Witness Protection", 3, 2},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			e := New(cfg)
			e.Advance()
			// Enough permanents that the recoloured creature is not the first read.
			for range 3 {
				onBoard(t, e, 0, "Name:Goat\nManaCost:W\nTypes:Creature Goat\nPT:0/1\nOracle:x\n")
			}
			k := onBoardCard(t, e, 0, knight)
			creature := onBoard(t, e, 1, c.creature)
			e.emit(events.Event{Kind: events.Tap, Obj: creature})
			if got := power(e, k); got != c.before {
				t.Fatalf("before the Aura: power %d, want %d", got, c.before)
			}
			aura := onBoardCard(t, e, 1, corpusAlternativeCard(t, c.aura))
			e.emit(events.Event{Kind: events.Attach, Obj: aura, IDs: []state.ObjID{creature}})
			if got := power(e, k); got != c.with {
				t.Fatalf("with %s: power %d, want %d", c.aura, got, c.with)
			}
			e.emit(events.Event{Kind: events.MoveZone, Obj: aura, From: state.ZBattlefield, To: state.ZGraveyard})
			if got := power(e, k); got != c.before {
				t.Fatalf("after the Aura left: power %d, want %d", got, c.before)
			}
		})
	}
}
