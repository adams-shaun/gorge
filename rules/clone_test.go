package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/testutil"
)

// drive answers n decisions with the package's own testBot and returns the
// intents it submitted, so the same choices can be replayed elsewhere.
func drive(t *testing.T, e *Engine, b *testBot, n int) []decision.Intent {
	t.Helper()
	var out []decision.Intent
	for i := 0; i < n && !e.G.Over && e.Pending() != nil; i++ {
		d := e.Pending()
		in := b.answer(e.G.Step.IsMain(), d)
		if err := e.Submit(in); err != nil {
			t.Fatalf("intent %d: %v", i, err)
		}
		out = append(out, in)
	}
	return out
}

func TestCloneStaysIndependentAndReplaysInLockstep(t *testing.T) {
	names, decks := testutil.SampleDecks(t, 4)
	e := New(Config{Seed: 7, Names: names, Decks: decks})
	e.Advance()
	bot := newTestBot(7)
	drive(t, e, bot, 40)

	c := e.Clone()
	headBefore, drawsBefore, eventsBefore := e.L.Head(), e.RNGDraws(), len(e.L.Events)
	if got := diffGames(e.G, c.G); got != "" {
		t.Fatalf("clone differs from original at the boundary: %s", got)
	}

	// Diverge the clone by 60 more decisions; the original must not move.
	recorded := drive(t, c, bot, 60)
	if len(recorded) == 0 {
		t.Fatal("clone accepted no intents")
	}
	if e.L.Head() != headBefore || e.RNGDraws() != drawsBefore || len(e.L.Events) != eventsBefore {
		t.Fatal("driving the clone changed the original")
	}

	// Feed the original the very same intents: identical events, chain head
	// and RNG position mean the clone copied every piece of engine state.
	for i, in := range recorded {
		if err := e.Submit(in); err != nil {
			t.Fatalf("original rejected recorded intent %d: %v", i, err)
		}
	}
	if e.L.Head() != c.L.Head() {
		t.Fatalf("chain heads differ after lockstep: %s vs %s", e.L.Head(), c.L.Head())
	}
	if e.RNGDraws() != c.RNGDraws() {
		t.Fatalf("RNG draws differ: %d vs %d", e.RNGDraws(), c.RNGDraws())
	}
	if got := diffGames(e.G, c.G); got != "" {
		t.Fatalf("games differ after lockstep: %s", got)
	}
	if len(e.pendingTriggers) != len(c.pendingTriggers) || len(e.continuous) != len(c.continuous) {
		t.Fatalf("engine-internal queues differ: triggers %d/%d, continuous %d/%d",
			len(e.pendingTriggers), len(c.pendingTriggers), len(e.continuous), len(c.continuous))
	}
}

func TestCloneSharesNoMutableStateWithTheOriginal(t *testing.T) {
	names, decks := testutil.SampleDecks(t, 2)
	e := New(Config{Seed: 3, Names: names, Decks: decks})
	e.Advance()
	drive(t, e, newTestBot(3), 30)
	c := e.Clone()

	// Mutate every cloned collection that has a backing array or map.
	c.G.Players[0].Life = -100
	c.L.Events[0].Kind = 200
	if c.pending != nil && len(c.pending.Options) > 0 {
		c.pending.Options[0].Label = "mutated"
	}
	for i := range c.continuous {
		c.continuous[i].AddKeywords = append(c.continuous[i].AddKeywords, "Mutated")
	}
	for i := range c.pendingTriggers {
		if c.pendingTriggers[i].Ctx.SVars != nil {
			c.pendingTriggers[i].Ctx.SVars["mutated"] = "yes"
		}
	}
	c.triggerFireCount = map[triggerKey]int32{{Source: 1, Idx: 0}: 99}

	if e.G.Players[0].Life == -100 || e.L.Events[0].Kind == 200 {
		t.Fatal("clone shares Game or Log storage with the original")
	}
	if e.pending != nil && len(e.pending.Options) > 0 && e.pending.Options[0].Label == "mutated" {
		t.Fatal("clone shares the pending decision's Options")
	}
	for _, ce := range e.continuous {
		for _, k := range ce.AddKeywords {
			if k == "Mutated" {
				t.Fatal("clone shares a continuous effect's AddKeywords")
			}
		}
	}
	for _, pt := range e.pendingTriggers {
		if pt.Ctx.SVars["mutated"] == "yes" {
			t.Fatal("clone shares a pending trigger's SVars map")
		}
	}
	if e.triggerFireCount[triggerKey{Source: 1, Idx: 0}] == 99 {
		t.Fatal("clone shares triggerFireCount")
	}
}

func TestCloneOfAFinishedGameIsFinished(t *testing.T) {
	names, decks := testutil.SampleDecks(t, 2)
	e := New(Config{Seed: 11, Names: names, Decks: decks})
	e.Advance()
	drive(t, e, newTestBot(11), 400000)
	if !e.G.Over {
		t.Fatal("fixture game did not finish")
	}
	c := e.Clone()
	if !c.G.Over || c.L.Head() != e.L.Head() || c.Pending() != nil {
		t.Fatal("clone of a finished game is not finished with the same head")
	}
	if err := c.Submit(decision.Intent{}); err == nil {
		t.Fatal("clone of a finished game accepted an intent")
	}
}
