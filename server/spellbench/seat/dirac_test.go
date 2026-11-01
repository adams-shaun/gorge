package seat

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/adapter"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/game"
	engineseat "github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestV2AgentSeatPlaysDiracGame seats the stdio agent on dirac games, to the end.
func TestV2AgentSeatPlaysDiracGame(t *testing.T) {
	const decks = "../../../dirac/testdata/bench/fdn"
	goCmd := goRun(t)
	reg := testutil.CorpusRegistry(t)
	e, err := adapter.New([]string{"fdn"}, reg, dirac.SurfaceAutopay)
	if err != nil {
		t.Skip(err)
	}
	names := testutil.DeckDirNames(t, decks)
	for i, seed := range []uint64{7, 8} {
		t.Run(fmt.Sprintf("seed%d", seed), func(t *testing.T) {
			c := game.Config{Seed: seed, Names: names[i : i+2], Tokens: reg.Tokens, NameUniverse: reg.Universe()}
			for _, name := range c.Names {
				_, d, err := deck.Load(reg, filepath.Join(decks, name+".json"))
				if err != nil {
					t.Fatal(err)
				}
				c.Decks = append(c.Decks, d)
			}
			g, err := e.New(c)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := New("test-agent", append(append([]string(nil), goCmd...), "./testdata/gorgeagent"), bots.Options{Seed: seed})
			if err != nil {
				t.Fatal(err)
			}
			agent := raw.(*Agent)
			own := deck.NewManifest(c.Names[0], "", []*cards.Card(c.Decks[0]), nil, nil)
			if err := agent.AttachMatch(&bots.MatchContext{Context: context.Background(), Game: g, GameID: "g-0000000000000001",
				Seats: 2, Format: "constructed", Seed: []byte("test"), OwnDeck: &own, Mulligan: "none",
				StartingSeat: int(g.Board().Active), Logf: t.Logf}); err != nil {
				t.Fatal(err)
			}
			seats := []engineseat.Seat{agent, heuristic.NewBot(seed)}
			defer endTestMatch(seats)
			for d := g.Pending(); d != nil; d = g.Pending() {
				v := view.Project(g.Board(), g.Chars(), state.PlayerID(d.Player), d)
				v.Round = view.RoundOf(g.Board(), g.Log().Events)
				in, err := seats[d.Player].Decide(context.Background(), v, *d)
				if err != nil {
					t.Fatal(err)
				}
				if err := g.Submit(in); err != nil {
					t.Fatalf("seq %d: %v", d.Seq, err)
				}
			}
			if !g.Board().Over || agent.forfeit != "" {
				t.Fatalf("over=%v forfeit=%q", g.Board().Over, agent.forfeit)
			}
		})
	}
}
