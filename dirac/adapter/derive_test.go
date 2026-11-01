package adapter

import (
	"errors"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// benchGames lists game configs over bench deck pairs of one set.
func benchGames(t testing.TB, set string, n int) (*Engine, []game.Config) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e, err := New([]string{set}, reg, dirac.SurfaceAutopay)
	if err != nil {
		t.Skip(err)
	}
	files, err := filepath.Glob(filepath.Join("..", "testdata", "bench", set, "*.json"))
	if err != nil || len(files) < 2 {
		t.Fatalf("bench decks: %v, %d files", err, len(files))
	}
	var out []game.Config
	for i := 0; len(out) < n && i < 4*n; i++ {
		var decks [][]*cards.Card
		for _, f := range []string{files[i%len(files)], files[(i*7+1)%len(files)]} {
			_, d, err := deck.Load(reg, f)
			if err != nil {
				t.Fatal(err)
			}
			decks = append(decks, d)
		}
		c := game.Config{Seed: uint64(1000 + i), Names: []string{"p0", "p1"}, Decks: decks, Mulligans: 1}
		if e.Accepts(c) == nil {
			out = append(out, c)
		}
	}
	return e, out
}

// randomIntent is a legal shape of answer, never a concession.
func randomIntent(d *decision.Decision, r *rand.Rand) decision.Intent {
	in := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{}}
	if d.Kind == decision.KPriority {
		if n := len(d.PaymentActions); n > 0 && r.IntN(2) == 0 {
			a := &d.PaymentActions[r.IntN(n)]
			in.Choices = nil
			in.Payment = &decision.PaymentSelection{ActionID: a.ID, Plan: a.Plans[0]}
			return in
		}
		in.Choices = []int{r.IntN(len(d.Options) - 1)}
		return in
	}
	want := d.Min
	if d.Max > d.Min {
		want += r.IntN(d.Max - d.Min + 1)
	}
	if d.Repeatable {
		for range want {
			in.Choices = append(in.Choices, r.IntN(len(d.Options)))
		}
		return in
	}
	in.Choices = append(in.Choices, r.Perm(len(d.Options))[:min(want, len(d.Options))]...)
	return in
}

// fewestPicks is the shortest answer the kernel takes pick by pick: a random
// combination of a many-part ask may not be one.
func fewestPicks(g *Game, d *decision.Decision) decision.Intent {
	in := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{}}
	for len(in.Choices) < d.Min {
		next := -1
		for _, p := range g.Picks(in.Choices, false, nil) {
			if p.Choice >= 0 {
				next = p.Choice
				break
			}
		}
		if next < 0 {
			break
		}
		in.Choices = append(in.Choices, next)
	}
	return in
}

// playOut answers randomly to the end of the game. visit, if set, sees each position.
func playOut(t testing.TB, g *Game, seed uint64, visit func()) {
	t.Helper()
	r := rand.New(rand.NewPCG(seed, 11))
	for i := 0; g.Pending() != nil; i++ {
		if i > 20000 {
			t.Fatal("no end in 20000 answers")
		}
		if visit != nil {
			visit()
		}
		d := g.Pending()
		if err := g.Submit(randomIntent(d, r)); err != nil {
			if err := g.Submit(fewestPicks(g, d)); err != nil {
				t.Fatalf("%s: %v", d.Kind, err)
			}
		}
	}
}

func played(t testing.TB, e *Engine, c game.Config) *Game {
	t.Helper()
	gm, err := e.New(c)
	if err != nil {
		t.Fatal(err)
	}
	g := gm.(*Game)
	playOut(t, g, c.Seed, nil)
	return g
}

func TestDerivedLogIsDeterministic(t *testing.T) {
	e, cs := benchGames(t, "fdn", 3)
	for _, c := range cs {
		a, b := played(t, e, c), played(t, e, c)
		if a.Log().Head() != b.Log().Head() {
			t.Fatalf("seed %d: heads %s and %s", c.Seed, a.Log().Head(), b.Log().Head())
		}
	}
}

func TestReplayVerifies(t *testing.T) {
	e, cs := benchGames(t, "fdn", 1)
	c := cs[0]
	g := played(t, e, c)
	l := g.Log()
	r, err := e.Replay(l, c)
	if err != nil {
		t.Fatal(err)
	}
	if r.Log().Head() != l.Head() {
		t.Fatalf("heads %s and %s", r.Log().Head(), l.Head())
	}
	short := *l
	short.Events = l.Events[:len(l.Events)-1]
	var dv *game.Divergence
	if _, err := e.Replay(&short, c); !errors.As(err, &dv) {
		t.Fatalf("a truncated recording replayed: %v", err)
	}
	var at int
	for i, in := range l.Intents {
		if len(in.Choices) == 1 && in.Payment == nil {
			at = i
			break
		}
	}
	altered := *l
	altered.Intents = slices.Clone(l.Intents)
	altered.Intents[at].Seq++
	if _, err := e.Replay(&altered, c); err == nil || !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("an altered intent replayed: %v", err)
	}
	moved := slices.Clone(l.Events)
	moved[len(moved)/2].Amount += 7
	tampered := *l
	tampered.Events = moved
	if _, err := e.Replay(&tampered, c); !errors.As(err, &dv) || dv.Seq != uint64(len(moved)/2) {
		t.Fatalf("a changed event at %d: %v", len(moved)/2, err)
	}
}

func TestReplayRefusesAnotherKey(t *testing.T) {
	e, cs := benchGames(t, "fdn", 1)
	c := cs[0]
	g := played(t, e, c)
	c.Key = "another"
	_, err := e.Replay(g.Log(), c)
	if !errors.Is(err, ErrTablesMoved) {
		t.Fatalf("a recording of other tables: %v", err)
	}
	c.Key = g.EngineKey()
	if _, err := e.Replay(g.Log(), c); err != nil {
		t.Fatal(err)
	}
}

func TestReplayToMatchesFork(t *testing.T) {
	e, cs := benchGames(t, "fdn", 1)
	c := cs[0]
	gm, err := e.New(c)
	if err != nil {
		t.Fatal(err)
	}
	g := gm.(*Game)
	type mark struct {
		fork game.Game
		n    int
	}
	var marks []mark
	seen := 0
	playOut(t, g, c.Seed, func() {
		if seen%50 == 0 {
			marks = append(marks, mark{g.Fork(), len(g.Log().Intents)})
		}
		seen++
	})
	for _, m := range marks {
		r, err := e.ReplayTo(g.Log(), c, m.n)
		if err != nil {
			t.Fatal(err)
		}
		if r.Log().Head() != m.fork.Log().Head() || r.Pending().Seq != m.fork.Pending().Seq {
			t.Fatalf("at intent %d: head %s, fork's %s", m.n, r.Log().Head(), m.fork.Log().Head())
		}
	}
}

func TestWalkVisitsEachBoundary(t *testing.T) {
	e, cs := benchGames(t, "fdn", 1)
	c := cs[0]
	g := played(t, e, c)
	n := 0
	if _, err := e.Walk(g.Log(), c, len(g.Log().Intents), func(_ game.Game, i int) error {
		if i != n {
			t.Fatalf("visit %d at boundary %d", n, i)
		}
		n++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if n != len(g.Log().Intents)+1 {
		t.Fatalf("%d visits, %d intents", n, len(g.Log().Intents))
	}
}

// TestDerivedLogCoversWebKinds holds the log to every kind the web client's board
// motion reads, but player_counter: no card in these sets gives a player a counter.
func TestDerivedLogCoversWebKinds(t *testing.T) {
	e, cs := benchGames(t, "fdn", 40)
	seen := map[events.Kind]int{}
	for _, c := range cs {
		for _, ev := range played(t, e, c).Log().Events {
			seen[ev.Kind]++
		}
	}
	for _, k := range []events.Kind{events.MoveZone, events.Draw, events.PutOnStack, events.Tap, events.Untap, events.Damage,
		events.CounterChange, events.TokenCreate, events.Resolve, events.StepChange, events.TurnChange, events.DecisionAsk, events.GameOver} {
		if seen[k] == 0 {
			t.Errorf("%d games logged no %s", len(cs), k)
		}
	}
}

// TestDerivedLogNeverLeaks plays a game asking, of every event each seat and a
// spectator would be served, that it names nothing of a hand or library it can't see.
func TestDerivedLogNeverLeaks(t *testing.T) {
	e, cs := benchGames(t, "fdn", 5)
	for _, c := range cs {
		gm, err := e.New(c)
		if err != nil {
			t.Fatal(err)
		}
		g := gm.(*Game)
		checked := 0
		check := func() {
			b := g.Board()
			evs := g.Log().Events
			for _, ev := range evs[checked:] {
				if ev.Kind != events.TokenCreate && ev.From == state.ZCeased {
					t.Fatalf("a card comes from outside the game: %+v", ev)
				}
				if ev.Kind != events.Draw && (ev.Kind != events.MoveZone || !ev.From.Hidden() || !ev.To.Hidden()) {
					continue
				}
				for _, v := range []struct {
					seat int
					vis  view.Visibility
				}{{0, view.Seat}, {1, view.Seat}, {0, view.Public}} {
					if v.vis == view.Seat && int(ev.Player) == v.seat {
						continue
					}
					if r := view.RedactEventFor(b, ev, 0, v.vis); v.vis == view.Public && r.Obj != 0 {
						t.Fatalf("a spectator is told %v's hidden move: %+v", ev.Player, r)
					}
					if r := view.RedactEventFor(b, ev, ev.Player^1, view.Seat); r.Obj != 0 {
						t.Fatalf("the other seat is told %v's hidden move: %+v", ev.Player, r)
					}
				}
			}
			checked = len(evs)
		}
		playOut(t, g, c.Seed, check)
	}
}

// TestBoundsOfDerivedLog holds the burst structure a host reads: one ask ending each
// answer's events, and a last burst ending in the game's end.
func TestBoundsOfDerivedLog(t *testing.T) {
	e, cs := benchGames(t, "fdn", 3)
	for _, c := range cs {
		gm, err := e.New(c)
		if err != nil {
			t.Fatal(err)
		}
		g := gm.(*Game)
		var want []uint64
		playOut(t, g, c.Seed, func() { want = append(want, uint64(len(g.Log().Events))) })
		var got []uint64
		for _, ev := range g.Log().Events {
			if ev.Kind == events.DecisionAsk {
				got = append(got, ev.Seq+1)
			}
		}
		if !slices.Equal(got, want) {
			t.Fatalf("seed %d: bounds %d, intent boundaries %d", c.Seed, len(got), len(want))
		}
		evs := g.Log().Events
		if evs[len(evs)-1].Kind != events.GameOver && g.StallReason() == "" {
			t.Fatalf("seed %d ends on %s", c.Seed, evs[len(evs)-1].Kind)
		}
	}
}

// silentKinds are what a transcript leaves out by design.
var silentKinds = map[events.Kind]bool{events.DecisionAsk: true}

func TestDescribeHasNoEmptyLine(t *testing.T) {
	e, cs := benchGames(t, "fdn", 5)
	for _, c := range cs {
		g := played(t, e, c)
		for _, ev := range g.Log().Events {
			if !silentKinds[ev.Kind] && view.Describe(g.Board(), ev) == "" {
				t.Errorf("%s reads as nothing: %+v", ev.Kind, ev)
			}
		}
	}
}

// TestTranscriptSample writes a game's transcript where DERIVE_SAMPLE names, for a
// person to read.
func TestTranscriptSample(t *testing.T) {
	dir := os.Getenv("DERIVE_SAMPLE")
	if dir == "" {
		t.Skip("DERIVE_SAMPLE names a directory")
	}
	e, cs := benchGames(t, "fdn", 40)
	var b strings.Builder
	for i, c := range cs {
		g := played(t, e, c)
		if i < 40 {
			b.WriteString("=== game\n")
			for _, ev := range g.Log().Events {
				if s := view.Describe(g.Board(), ev); s != "" {
					b.WriteString(s + "\n")
				}
			}
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "transcripts.txt"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestAnnouncedCastIsPaidByItsPlan answers a cast the way the web client's hand does,
// by announcing it, and gets what its plan pays.
func TestAnnouncedCastIsPaidByItsPlan(t *testing.T) {
	e, cs := benchGames(t, "fdn", 1)
	gm, err := e.New(cs[0])
	if err != nil {
		t.Fatal(err)
	}
	g := gm.(*Game)
	r := rand.New(rand.NewPCG(1, 2))
	for i := 0; g.Pending() != nil && i < 2000; i++ {
		d := g.Pending()
		if d.Kind == decision.KPriority && len(d.PaymentActions) > 0 {
			f := g.Fork().(*Game)
			a := &d.PaymentActions[0]
			if err := g.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Announce: &decision.AnnounceSelection{ActionID: a.ID}}); err != nil {
				t.Fatal(err)
			}
			if err := f.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Payment: &decision.PaymentSelection{ActionID: a.ID, Plan: a.Plans[0]}}); err != nil {
				t.Fatal(err)
			}
			if g.Log().Head() != f.Log().Head() {
				t.Fatalf("announced %s, paid %s", g.Log().Head(), f.Log().Head())
			}
			return
		}
		if err := g.Submit(randomIntent(d, r)); err != nil {
			if err := g.Submit(fewestPicks(g, d)); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Fatal("no payment action in the game")
}
