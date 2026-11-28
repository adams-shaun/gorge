package rules

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
)

// ordinalGameDeck is a four-seat test deck: lands and a vanilla creature, so
// the game asks priority, attack and block decisions besides the mulligans.
func ordinalGameDeck(t *testing.T) []*cards.Card {
	t.Helper()
	bear := card(t, "Name:Ordinal Bear\nManaCost:R\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	deck := mountainDeck(t, 40)
	for i := 0; i < len(deck); i += 3 {
		deck[i] = bear
	}
	return deck
}

// ordinalIntent answers d with a random legal answer from rng, else the first
// Min options.
func ordinalIntent(e *Engine, d *decision.Decision, rng *rand.Rand) decision.Intent {
	if len(d.Options) > 0 && d.Max >= 1 {
		i := rng.IntN(len(d.Options))
		in := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{i}}
		if d.Options[i].Kind != optConcede && d.Validate(in) == nil && submitValidate(e, d, in) == nil {
			return in
		}
	}
	ch := make([]int, 0, d.Min)
	for i := 0; i < d.Min && i < len(d.Options); i++ {
		ch = append(ch, i)
	}
	return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: ch}
}

func countDecisionMade(l *events.Log) uint64 {
	n := uint64(0)
	for i := range l.Events {
		if l.Events[i].Kind == events.DecisionMade {
			n++
		}
	}
	return n
}

// TestDecisionOrdinalCountsAnswered: over one four-seat game, every ask's
// Ordinal is the number of decisions answered before it (0 at the first ask,
// one more per answer, the log's DecisionMade count), and a replay of the
// recorded intents asks with the same Ordinals.
func TestDecisionOrdinalCountsAnswered(t *testing.T) {
	t.Parallel()
	deck := ordinalGameDeck(t)
	cfg := Config{Seed: 20261128, Mulligans: 1, Names: []string{"a", "b", "c", "d"},
		Decks: [][]*cards.Card{deck, deck, deck, deck}}
	e := New(cfg)
	e.Advance()
	rng := rand.New(rand.NewPCG(11, 22))
	var asked []uint64
	kinds := map[decision.Kind]bool{}
	for len(asked) < 600 && !e.G.Over {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no pending decision at answer %d", len(asked))
		}
		if want := uint64(len(asked)); d.Ordinal != want {
			t.Fatalf("answer %d (%s for p%d): Ordinal %d, want %d", len(asked), d.Kind, d.Player, d.Ordinal, want)
		}
		if n := countDecisionMade(e.L); d.Ordinal != n {
			t.Fatalf("answer %d: Ordinal %d, log has %d DecisionMade", len(asked), d.Ordinal, n)
		}
		asked = append(asked, d.Ordinal)
		kinds[d.Kind] = true
		if err := e.Submit(ordinalIntent(e, d, rng)); err != nil {
			t.Fatalf("answer %d: %v", len(asked)-1, err)
		}
	}
	// the precondition the counts depend on: a real game, past the mulligans,
	// with combat asked and Seq (event-log length) a different count
	if len(asked) < 200 || !kinds[decision.KAttackers] || !kinds[decision.KPriority] {
		t.Fatalf("game too thin: %d answers, kinds %v", len(asked), kinds)
	}
	if last := e.Pending(); last != nil && last.Seq == last.Ordinal {
		t.Fatalf("precondition: Seq %d equals Ordinal (the counts must differ)", last.Seq)
	}

	r := New(cfg)
	r.Advance()
	for i, in := range e.L.Intents {
		d := r.Pending()
		if d == nil {
			t.Fatalf("replay: no pending decision at intent %d", i)
		}
		if d.Ordinal != asked[i] {
			t.Fatalf("replay intent %d: Ordinal %d, recorded %d", i, d.Ordinal, asked[i])
		}
		if err := r.Submit(in); err != nil {
			t.Fatalf("replay intent %d: %v", i, err)
		}
	}
	if d, c := r.Pending(), e.Pending(); (d == nil) != (c == nil) || (d != nil && d.Ordinal != c.Ordinal) {
		t.Fatalf("replay ends at a different ask")
	}
	// a clone counts on from the original's answers
	c := e.Clone()
	if d := c.Pending(); d != nil {
		if err := c.Submit(ordinalIntent(c, d, rng)); err != nil {
			t.Fatal(err)
		}
		if d := c.Pending(); d != nil && d.Ordinal != uint64(len(asked))+1 {
			t.Fatalf("clone's next Ordinal %d, want %d", d.Ordinal, len(asked)+1)
		}
	}
}
