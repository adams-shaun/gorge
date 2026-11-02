package adapter

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// startGame is a served game between the first two FDN bench decks.
func startGame(t testing.TB) *Game {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e, err := New([]string{"fdn"}, reg, dirac.SurfaceAutopay)
	if err != nil {
		t.Skip(err)
	}
	var decks [][]*cards.Card
	for _, f := range []string{"000_FDN_top_00001_WB.json", "001_FDN_top_00032_WU.json"} {
		_, d, err := deck.Load(reg, filepath.Join("..", "testdata", "bench", "fdn", f))
		if err != nil {
			t.Fatal(err)
		}
		decks = append(decks, d)
	}
	gm, err := e.New(game.Config{Seed: 7, Names: []string{"p0", "p1"}, Decks: decks, Mulligans: 1})
	if err != nil {
		t.Fatal(err)
	}
	return gm.(*Game)
}

// answerFirst answers the pending decision with its first legal shape, playing a land
// or a free option at priority before passing.
func answerFirst(t testing.TB, g *Game) {
	t.Helper()
	d := g.Pending()
	in := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{}}
	if d.Kind == decision.KPriority {
		in.Choices = []int{0}
		if d.Options[0].Kind == "concede" {
			t.Fatal("a priority decision with only concede")
		}
	} else {
		for j := 0; j < d.Min; j++ {
			in.Choices = append(in.Choices, j)
		}
	}
	if err := g.Submit(in); err != nil {
		t.Fatal(err)
	}
}

// toBattlefield plays until seat p has a permanent, and returns it.
func toBattlefield(t testing.TB, g *Game, p state.PlayerID) *state.Object {
	t.Helper()
	for i := 0; i < 400 && g.Pending() != nil; i++ {
		if bf := g.Board().Zone(state.ZBattlefield, p); len(bf) > 0 {
			return g.Board().Obj(bf[0])
		}
		answerFirst(t, g)
	}
	t.Fatal("no permanent came into play")
	return nil
}

func TestStallIsReported(t *testing.T) {
	g := startGame(t)
	if r := g.StallReason(); r != "" {
		t.Fatalf("a live game reads as stalled: %q", r)
	}
	g.stalled, g.pend = dirac.StallExiledWith, nil
	g.kp.Over = true
	g.materialise()
	if g.Pending() != nil || g.Board().Over {
		t.Fatalf("stalled: pending %v, over %v", g.Pending(), g.Board().Over)
	}
	want := "capacity " + dirac.StallName(dirac.StallExiledWith)
	if r := g.StallReason(); r != want {
		t.Fatalf("StallReason = %q, want %q", r, want)
	}
}

// TestAdapterConfigIsLockstepConfig holds what a served game plays under to the config
// lockstep proves, and config as the one way into Init.
func TestAdapterConfigIsLockstepConfig(t *testing.T) {
	for _, surface := range []dirac.Surface{dirac.SurfaceFull, dirac.SurfaceAutopay} {
		want := dirac.LockstepConfig(surface)
		want.NoElide, want.ArrangeRestOrder, want.Mulligans = true, true, 2
		if got := (&Engine{surface: surface}).config(game.Config{Mulligans: 2}); got != want {
			t.Errorf("surface %v: config %+v, want %+v", surface, got, want)
		}
	}
	fset := token.NewFileSet()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	inits := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok {
				if s, ok := c.Fun.(*ast.SelectorExpr); ok && s.Sel.Name == "Init" {
					inits++
					last, ok := c.Args[len(c.Args)-1].(*ast.CallExpr)
					if sel, sok := last.Fun.(*ast.SelectorExpr); !ok || !sok || sel.Sel.Name != "config" {
						t.Errorf("%s: a game starts under something other than Engine.config", fset.Position(c.Pos()))
					}
				}
			}
			return true
		})
	}
	if inits != 1 {
		t.Errorf("%d calls of Init, want 1", inits)
	}
}

func TestFaceDownRedacted(t *testing.T) {
	g := startGame(t)
	o := toBattlefield(t, g, 0)
	name := o.Face().Name
	o.FaceDown = true
	for _, tc := range []struct {
		seat state.PlayerID
		vis  view.Visibility
	}{{1, view.Seat}, {0, view.Public}} {
		raw, err := json.Marshal(view.ProjectFor(g.Board(), g.Chars(), tc.seat, tc.vis, nil))
		if err != nil {
			t.Fatal(err)
		}
		var v view.View
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatal(err)
		}
		for _, c := range v.Players[0].Battlefield {
			if c.ID == o.ID && (c.Name == name || c.CardName == name || !c.FaceDown) {
				t.Errorf("seat %d, %v: a face-down permanent reads as %q: %+v", tc.seat, tc.vis, name, c)
			}
		}
	}
}

// BenchmarkMaterialiseFDN is the cost of one board rebuild on a real FDN position, the
// work a served decision adds to a kernel step.
func BenchmarkMaterialiseFDN(b *testing.B) {
	g := startGame(b)
	for i := 0; i < 80 && g.Pending() != nil; i++ {
		answerFirst(b, g)
	}
	if g.Pending() == nil {
		b.Fatal("the game ended before the benchmark's position")
	}
	g.materialise()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		g.materialise()
	}
}
