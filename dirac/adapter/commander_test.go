package adapter

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// precons is a Commander table of two Foundations precons on the cmd5 set.
func precons(t testing.TB, names ...string) (*Engine, game.Config) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e, err := New([]string{"cmd5"}, reg, dirac.SurfaceAutopay)
	if err != nil {
		t.Skip(err)
	}
	c := game.Config{Seed: 7, Format: game.FormatCommander, StartingLife: 40, Mulligans: 1}
	for i, n := range names {
		f, d, err := deck.Load(reg, filepath.Join("..", "..", "testkit", "testutil", "decks", n+".json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := e.CheckDeck(d); err != nil {
			t.Skipf("the cmd5 set does not play %s yet: %v", n, err)
		}
		c.Names = append(c.Names, "p"+string(rune('0'+i)))
		c.Decks = append(c.Decks, d)
		c.Commanders = append(c.Commanders, f.CommanderIndices())
	}
	return e, c
}

// benchCommander is a Commander table of two FDN bench decks, each naming its
// first card the commander: the wiring is the same whichever set holds the decks.
func benchCommander(t testing.TB) (*Engine, game.Config) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e, err := New([]string{"fdn"}, reg, dirac.SurfaceAutopay)
	if err != nil {
		t.Skip(err)
	}
	c := game.Config{Seed: 7, Format: game.FormatCommander, StartingLife: 40, Mulligans: 1, Names: []string{"p0", "p1"}}
	for _, f := range []string{"000_FDN_top_00001_WB.json", "001_FDN_top_00032_WU.json"} {
		_, d, err := deck.Load(reg, filepath.Join("..", "testdata", "bench", "fdn", f))
		if err != nil {
			t.Fatal(err)
		}
		c.Decks = append(c.Decks, d)
		c.Commanders = append(c.Commanders, []int{0})
	}
	return e, c
}

func TestDiracServesCommander(t *testing.T) {
	t.Run("bench decks", func(t *testing.T) {
		e, c := benchCommander(t)
		checkServedCommander(t, e, c)
	})
	t.Run("foundations precons", func(t *testing.T) {
		e, c := precons(t, "foundations-calling-all-angels", "foundations-keen-engineering")
		checkServedCommander(t, e, c)
	})
}

func checkServedCommander(t *testing.T, e *Engine, c game.Config) {
	t.Helper()
	gm, err := e.New(c)
	if err != nil {
		t.Fatal(err)
	}
	g := gm.(*Game)
	for p := state.PlayerID(0); p < 2; p++ {
		if life := g.Board().Players[p].Life; life != 40 {
			t.Errorf("seat %d life %d, want 40", p, life)
		}
		cmd := g.Board().Zone(state.ZCommand, p)
		if len(cmd) != 1 {
			t.Fatalf("seat %d command zone holds %d cards, want its commander", p, len(cmd))
		}
		if r := g.Board().Players[p].Commanders; len(r) != 1 || r[0] != cmd[0] {
			t.Errorf("seat %d commander roster %v, want the command zone's %v", p, r, cmd)
		}
		want := cards.NormalizeName(c.Decks[p][c.Commanders[p][0]].Faces[0].Name)
		if got := cards.NormalizeName(g.Board().Obj(cmd[0]).Card.Faces[0].Name); got != want {
			t.Errorf("seat %d command zone holds %q, want %q", p, got, want)
		}
	}
	for i := 0; i < 400 && g.Pending() != nil; i++ {
		answerFirst(t, g)
	}

	// a third seat, three commanders, and a bad index are refused with the reason
	c3 := c
	c3.Decks = append(append([][]*cards.Card{}, c.Decks...), c.Decks[0])
	c3.Names = append(append([]string{}, c.Names...), "p2")
	if _, err := e.New(c3); err == nil {
		t.Error("a 3-seat commander table was accepted")
	}
	cp := c
	cp.Commanders = [][]int{{c.Commanders[0][0], 1, 2}, c.Commanders[1]}
	if _, err := e.New(cp); err == nil {
		t.Error("three commanders were accepted")
	}
	cb := c
	cb.Commanders = [][]int{{len(c.Decks[0])}, c.Commanders[1]}
	if _, err := e.New(cb); err == nil {
		t.Error("a commander index outside the deck was accepted")
	}
}
