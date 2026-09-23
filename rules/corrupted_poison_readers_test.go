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

// TestFeedTheInfectionCorruptedArmResolves proves the DBPoisoned arm resolves
// END TO END: with the Feed the Infection permanent on the battlefield under
// seat 0 and seat 1 at 3 poison, resolving the corpus SVar moves seat 1's life
// to 17 and leaves the caster (seat 0) untouched — before the definedSpec
// Opponent.IsCorrupted bridge, Defined fell through to the source fallback and
// the CASTER lost the 3 life instead.
func TestFeedTheInfectionCorruptedArmResolves(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	feed, ok := reg.Lookup("Feed the Infection")
	if !ok {
		t.Fatal("corpus missing Feed the Infection")
	}
	e := layerEngine(t)
	feedID := onBoardCard(t, e, 0, feed)
	if o := e.G.Obj(feedID); o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatal("test precondition: Feed the Infection not on seat 0's battlefield")
	}
	e.emit(events.Event{Kind: events.PlayerCounterChange, Player: 1, Counter: "POISON", Amount: 3})
	if e.G.Players[1].Counter("POISON") != 3 {
		t.Fatal("test precondition: opponent poison did not seed")
	}
	if e.G.Players[0].Life != 20 || e.G.Players[1].Life != 20 {
		t.Fatalf("test precondition: life 20/20 expected, got %d/%d", e.G.Players[0].Life, e.G.Players[1].Life)
	}
	dbPoisoned := resolveSVarOf(t, feed.Faces[0], "DBPoisoned")
	if got := effects.Defined(e, &effects.Ctx{Source: feedID, Controller: 0}, dbPoisoned); len(got) != 1 ||
		!got[0].IsPlayer || got[0].Player != 1 {
		t.Fatalf("Defined$ Opponent.IsCorrupted = %v, want [player 1]", got)
	}
	effects.Resolve(e, &effects.Ctx{Source: feedID, Controller: 0}, dbPoisoned)
	if got := e.G.Players[1].Life; got != 17 {
		t.Fatalf("corrupted opponent life = %d, want 17 (lost 3)", got)
	}
	if got := e.G.Players[0].Life; got != 20 {
		t.Fatalf("caster life = %d, want 20 (unchanged; the pre-fix source fallback made the CASTER lose the life)", got)
	}
}

// TestFeedTheInfectionBelowThresholdLosesNothing pins the "nothing otherwise"
// half: a 2-poison opponent is not Corrupted, so the arm resolves to the empty
// acting set and NO life moves. The empty result must come from the bridge's
// own fail-closed filter (Defined returning an empty player set), not from the
// historical source fallback, which would hand back the caster's own object.
func TestFeedTheInfectionBelowThresholdLosesNothing(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	feed, ok := reg.Lookup("Feed the Infection")
	if !ok {
		t.Fatal("corpus missing Feed the Infection")
	}
	e := layerEngine(t)
	feedID := onBoardCard(t, e, 0, feed)
	e.emit(events.Event{Kind: events.PlayerCounterChange, Player: 1, Counter: "POISON", Amount: 2})
	if e.G.Players[1].Counter("POISON") != 2 {
		t.Fatal("test precondition: opponent poison did not seed")
	}
	dbPoisoned := resolveSVarOf(t, feed.Faces[0], "DBPoisoned")
	got := effects.Defined(e, &effects.Ctx{Source: feedID, Controller: 0}, dbPoisoned)
	for _, tgt := range got {
		if tgt.IsPlayer || tgt.Obj == feedID {
			t.Fatalf("below threshold Defined = %v, want empty (source fallback is the pre-fix bug)", got)
		}
	}
	effects.Resolve(e, &effects.Ctx{Source: feedID, Controller: 0}, dbPoisoned)
	if e.G.Players[0].Life != 20 || e.G.Players[1].Life != 20 {
		t.Fatalf("below-threshold lives = %d/%d, want 20/20 (no seat loses life)", e.G.Players[0].Life, e.G.Players[1].Life)
	}
}
