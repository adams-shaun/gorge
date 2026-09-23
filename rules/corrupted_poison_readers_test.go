package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

func TestCorruptedReadersUseCorpusDefinitions(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	feed, ok := reg.Lookup("Feed the Infection")
	if !ok {
		t.Fatal("corpus missing Feed the Infection")
	}
	wurm, ok := reg.Lookup("Wurmquake")
	if !ok {
		t.Fatal("corpus missing Wurmquake")
	}
	e := layerEngine(t)
	feedBody := svarBodyOf(t, feed.Faces[0], "DBPoisoned")
	feedEffect := resolveSVarOf(t, feed.Faces[0], "DBPoisoned")
	if feedEffect.API != "LoseLife" || feedEffect.Params["Defined"] != "Opponent.IsCorrupted" {
		t.Fatalf("Feed precondition: DBPoisoned = %s (%v)", feedBody, feedEffect.Params)
	}
	poison := func(p state.PlayerID, n int32) {
		e.emit(events.Event{Kind: events.PlayerCounterChange, Player: p, Counter: "POISON", Amount: n})
	}
	poison(0, 2)
	poison(1, 3)
	if e.G.Players[0].Counter("POISON") != 2 || e.G.Players[1].Counter("POISON") != 3 {
		t.Fatal("test precondition: poison counters did not seed")
	}
	if !effects.MatchesPlayerSpec(e.G, "Opponent.IsCorrupted", 1, 0) || effects.MatchesPlayerSpec(e.G, "You.IsCorrupted", 0, 0) {
		t.Fatal("Feed the Infection's real Corrupted definition did not distinguish 3 poison from 2")
	}
	if !effects.MatchesPlayerSpec(e.G, "Player.Opponent+IsCorrupted", 1, 0) || effects.MatchesPlayerSpec(e.G, "Player.Opponent+!IsCorrupted", 1, 0) {
		t.Fatal("compound Corrupted player predicate did not match / negate")
	}

	wurmCount := svarBodyOf(t, wurm.Faces[0], "Y")
	if wurmCount != "PlayerCountOpponents$HasPropertyIsCorrupted" {
		t.Fatalf("Wurmquake precondition: SVar:Y = %q", wurmCount)
	}
	ctx := &effects.Ctx{Controller: 0}
	if got, valid := effects.EvalCountOK(e, ctx, wurmCount); !valid || got != 1 {
		t.Fatalf("Wurmquake corrupted-opponent count = %d (valid %v), want 1", got, valid)
	}
	if got, valid := effects.EvalCountOK(e, ctx, "PlayerCountOpponents$HighestCounters.Poison"); !valid || got != 3 {
		t.Fatalf("highest opponent poison = %d (valid %v), want 3", got, valid)
	}
}
