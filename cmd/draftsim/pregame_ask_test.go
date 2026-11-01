package main

import (
	"testing"

	"github.com/adams-shaun/gorge/draft/limited/setdata"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/replay"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestDraftsimPregameAskIsPosed pins the mtgsim-shaped pregame: every game
// draftsim drives is built with rules.NewStartingPlayerChoice and poses CR
// 103.1's winner-chooses ask before its first Advance, so the engine log carries the ask and the game still replays from
// its config. Without the pose the log carries no starting_player ask at
// all, so a zero ask count fails loudly.
func TestDraftsimPregameAskIsPosed(t *testing.T) {
	dir := testutil.CorpusDirOrSkip(t)
	r, err := newTestRunner(dir, "bot,bot,bot,bot")
	if err != nil {
		t.Fatalf("runner: %v", err)
	}
	spec, err := setdata.Load("FDN")
	if err != nil {
		t.Fatalf("setdata FDN: %v", err)
	}
	r.spec = spec

	// Precondition: the corpus really resolves the deck's card, or the
	// decks below are empty and the game asserts nothing about real play.
	basic, ok := r.reg.Lookup("Plains")
	if !ok {
		t.Fatal("corpus does not resolve Plains; cannot build a probe deck")
	}
	deck := func() []*cards.Card {
		cs := make([]*cards.Card, 0, 40)
		for i := 0; i < 40; i++ {
			cs = append(cs, basic)
		}
		return cs
	}

	deckA, deckB := deck(), deck()
	oc, eng, err := driveGame(r, 99, [2]string{"bot", "bot"}, [2]string{"p0", "p1"}, deckA, deckB)
	if err != nil {
		t.Fatalf("game: %v", err)
	}
	if oc.Turns == 0 {
		t.Fatal("game recorded 0 turns; the drive never ran")
	}
	asks := 0
	for _, ev := range eng.L.Events {
		if ev.Kind == events.DecisionAsk && ev.Text == string(decision.KStartingPlayer) {
			asks++
		}
	}
	if asks != 1 {
		t.Fatalf("engine log carries %d starting_player asks, want 1 (the pose ran before the first Advance)", asks)
	}
	if _, err := replay.Replay(eng.L, gameConfig(r, 99, [2]string{"p0", "p1"}, deckA, deckB)); err != nil {
		t.Fatalf("replay of the asked game: %v", err)
	}
}
