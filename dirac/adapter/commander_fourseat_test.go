package adapter

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// fourSeatPod is a 4-seat Commander pod of foundations-wretched-ranks in every
// seat, on the edh set.
func fourSeatPod(t testing.TB) (*Engine, game.Config) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e, err := New([]string{"edh"}, reg, dirac.SurfaceAutopay)
	if err != nil {
		t.Skip(err)
	}
	c := game.Config{Seed: 11, Format: game.FormatCommander, StartingLife: 40, Mulligans: 1}
	for i := range 4 {
		f, d, err := deck.Load(reg, filepath.Join("..", "..", "testkit", "testutil", "decks", "foundations-wretched-ranks.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := e.CheckDeck(d); err != nil {
			t.Fatalf("the edh set does not play foundations-wretched-ranks: %v", err)
		}
		c.Names = append(c.Names, "p"+string(rune('0'+i)))
		c.Decks = append(c.Decks, d)
		c.Commanders = append(c.Commanders, f.CommanderIndices())
	}
	return e, c
}

func TestAdapterFourSeatCommander(t *testing.T) {
	e, c := fourSeatPod(t)
	gm, err := e.New(c)
	if err != nil {
		t.Fatal(err)
	}
	g := gm.(*Game)
	b := g.Board()
	if len(b.Players) != 4 {
		t.Fatalf("board seats %d players, want 4", len(b.Players))
	}
	for p := range state.PlayerID(4) {
		if life := b.Players[p].Life; life != 40 {
			t.Errorf("seat %d life %d, want 40", p, life)
		}
		if cmd := b.Zone(state.ZCommand, p); len(cmd) != 1 {
			t.Errorf("seat %d command zone holds %d cards, want its commander", p, len(cmd))
		}
		if n := len(b.Zone(state.ZHand, p)); n != 7 {
			t.Errorf("seat %d hand holds %d cards, want 7", p, n)
		}
	}
	playOut(t, g, c.Seed, nil)
	if r := g.StallReason(); r != "" {
		t.Fatalf("the pod stalled: %s", r)
	}
	if !g.board.g.Over {
		t.Fatal("the pod has no pending decision but is not over")
	}
	lost := map[state.PlayerID]int{}
	for _, ev := range g.Log().Events {
		if ev.Kind == events.PlayerLost {
			lost[ev.Player]++
		}
	}
	gm2 := g.Board()
	t.Logf("turn %d, winner %d, draw %v, losses %v, %d events", gm2.Turn, gm2.Winner, gm2.Draw, lost, len(g.Log().Events))
	if !gm2.Draw && (len(lost) != 3 || lost[gm2.Winner] != 0) {
		t.Errorf("seat %d won, but the log's losses are %v: want each other seat once", gm2.Winner, lost)
	}
	for p, n := range lost {
		if n != 1 || !gm2.Players[p].Lost {
			t.Errorf("seat %d: %d losses logged, board lost %v", p, n, gm2.Players[p].Lost)
		}
	}
	again := played(t, e, c)
	if again.Log().Head() != g.Log().Head() {
		t.Fatalf("seed %d played twice: heads %s and %s", c.Seed, g.Log().Head(), again.Log().Head())
	}
	r, err := e.Replay(g.Log(), c)
	if err != nil {
		t.Fatal(err)
	}
	if r.Log().Head() != g.Log().Head() {
		t.Fatalf("replay head %s, played %s", r.Log().Head(), g.Log().Head())
	}
}
