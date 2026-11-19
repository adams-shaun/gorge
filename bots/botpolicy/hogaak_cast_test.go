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

// TestBotHogaakCastsAreNotStranded plays a deck holding Hogaak, Arisen
// Necropolis ("You can't spend mana to cast this spell"): the bot's convoke
// announcement must cover the whole cost (stat:ManaRestriction), so every
// answer it submits is accepted and no cast reverses.
func TestBotHogaakCastsAreNotStranded(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	var deck []*cards.Card
	for _, n := range []struct {
		name  string
		count int
	}{{"Swamp", 10}, {"Forest", 10}, {"Grizzly Bears", 20}, {"Hogaak, Arisen Necropolis", 20}} {
		c, ok := reg.Lookup(n.name)
		if !ok {
			t.Fatalf("no %s in the corpus", n.name)
		}
		for range n.count {
			deck = append(deck, c)
		}
	}
	hogaakCasts := 0
	for game := range uint64(6) {
		e := rules.New(rules.Config{Seed: 7000 + game, Names: []string{"a", "b"}, Decks: [][]*cards.Card{deck, deck}, Tokens: reg.Tokens})
		e.Advance()
		rngs := []*rand.Rand{rand.New(rand.NewPCG(game, 0)), rand.New(rand.NewPCG(game, 1))}
		board := botpolicy.NewBoard(2)
		for i := 0; i < 4000 && !e.G.Over; i++ {
			d := e.Pending()
			if d == nil {
				break
			}
			in := botpolicy.Decide(botpolicy.BoardFromGameInto(e.G, e, d.Player, &board), d, rngs[d.Player])
			if err := e.Submit(in); err != nil {
				t.Fatalf("game %d decision %d (%q, kind %v): the bot's answer %v was refused: %v", game, i, d.Prompt, d.Kind, in.Choices, err)
			}
		}
		for _, ev := range e.L.Events {
			switch ev.Kind {
			case events.SpellCastReversed:
				t.Fatalf("game %d: a cast reversed: %+v", game, ev)
			case events.PutOnStack:
				if o := e.G.Obj(ev.Obj); o != nil && o.Face().Name == "Hogaak, Arisen Necropolis" {
					hogaakCasts++
				}
			}
		}
	}
	if hogaakCasts == 0 {
		t.Fatal("precondition: Hogaak was never cast")
	}
}
