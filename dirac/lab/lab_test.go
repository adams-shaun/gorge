package lab_test

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/lab"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// vanillaCfg is a game on the hand-written vanilla table, which needs no corpus.
func vanillaCfg(seats int) lab.Config {
	ids := make([][]kop.CardID, seats)
	for i := range ids {
		ids[i] = v.Decks[i]
	}
	return lab.Config{Table: v.Table, IDs: ids}
}

func team(seats int, mk func(i int) lab.Policy) []lab.Policy {
	out := make([]lab.Policy, seats)
	for i := range out {
		out[i] = mk(i)
	}
	return out
}

func seeds(n int) []uint64 {
	out := make([]uint64, n)
	for i := range out {
		out[i] = uint64(i + 1)
	}
	return out
}

func TestDeterministicAcrossWorkers(t *testing.T) {
	pols := func() []lab.Policy { return []lab.Policy{lab.HBot(), lab.Random(7)} }
	a, sa, err := lab.Batch(vanillaCfg(2), pols(), seeds(12), 1, 20000)
	if err != nil {
		t.Fatal(err)
	}
	b, sb, err := lab.Batch(vanillaCfg(2), pols(), seeds(12), 3, 20000)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) || !reflect.DeepEqual(sa, sb) {
		t.Fatalf("same seeds and policies differ with the worker count:\n%+v\n%+v", sa, sb)
	}
	if sa.Games != 12 || sa.Wins[0]+sa.Wins[1]+sa.Draws+sa.Stalls+sa.Truncated+sa.Failed != 12 {
		t.Fatalf("summary does not add up: %+v", sa)
	}
	if sa.Failed != 0 {
		t.Fatalf("failed games: %+v", a)
	}
}

func TestForkIsIndependent(t *testing.T) {
	g, err := lab.NewGame(vanillaCfg(2))
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	rnd := dirac.NewRandomPolicy(5)
	for i := 0; i < 40 && !g.Done(); i++ {
		if err := g.Apply(rnd.Pick(g.Decision())); err != nil {
			t.Fatal(err)
		}
	}
	h, steps := g.Hash(), g.Steps()
	f := g.Fork()
	if f.Hash() != h {
		t.Fatal("a fork has another hash than its source")
	}
	r := lab.Play(f, []lab.Policy{lab.Random(1), lab.Random(2)}, 20000)
	if r.Outcome == lab.Running || g.Hash() != h || g.Steps() != steps || g.Done() {
		t.Fatalf("playing the fork moved the original (outcome %v)", r.Outcome)
	}
	// two forks played with the same policies end alike
	f1, f2 := g.Fork(), g.Fork()
	pols := func() []lab.Policy { return []lab.Policy{lab.Random(3), lab.HBot()} }
	r1, r2 := lab.Play(f1, pols(), 20000), lab.Play(f2, pols(), 20000)
	if !reflect.DeepEqual(r1, r2) || f1.Hash() != f2.Hash() {
		t.Fatalf("equal forks diverged: %+v vs %+v", r1, r2)
	}
	if g.Hash() != h {
		t.Fatal("original moved")
	}
}

func TestActionsAreReadable(t *testing.T) {
	g, err := lab.NewGame(vanillaCfg(2))
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	acts := g.Actions()
	if len(acts) == 0 || len(acts) != int(g.Decision().N) {
		t.Fatalf("%d actions for %+v", len(acts), g.Decision())
	}
	for _, o := range acts {
		if o.Text == "" || strings.HasPrefix(o.Text, "kind ") {
			t.Fatalf("option %d has no wording: %q", o.Index, o.Text)
		}
	}
	if err := g.Apply(dirac.MakeAction(dirac.AkCount-1, 0, 0, 255)); err == nil {
		t.Fatal("an action that is not an option was applied")
	}
}

func TestFourSeatBatch(t *testing.T) {
	pols := team(4, func(i int) lab.Policy { return lab.Random(uint64(i)) })
	rs, sum, err := lab.Batch(vanillaCfg(4), pols, seeds(6), 2, 50000)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Games != 6 || len(sum.Wins) != 4 || sum.Failed != 0 {
		t.Fatalf("summary %+v", sum)
	}
	for _, r := range rs {
		if r.Outcome == lab.Won && (r.Winner < 0 || r.Winner > 3) {
			t.Fatalf("winner %d", r.Winner)
		}
		if r.Decisions == 0 || len(r.Decks) != 4 {
			t.Fatalf("result %+v", r)
		}
	}
}

func TestTruncatedIsNotADraw(t *testing.T) {
	rs, sum, err := lab.Batch(vanillaCfg(2), team(2, func(int) lab.Policy { return lab.Random(1) }), seeds(3), 1, 5)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rs {
		if r.Outcome != lab.Truncated || r.Winner != -1 || r.Decisions != 5 {
			t.Fatalf("a game cut at 5 steps: %+v", r)
		}
	}
	if sum.Truncated != 3 || sum.Draws != 0 || sum.Wins[0]+sum.Wins[1] != 0 {
		t.Fatalf("truncated games are counted as %+v", sum)
	}
	if r := sum.WinRate(0); r.P != 0 || r.Hi <= 0 {
		t.Fatalf("rate %+v", r)
	}
}

// stallTable is vanilla with a Raise the Alarm that makes 60 tokens: a few casts fill the board.
func stallTable() *kop.Table {
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	cd := tbl.Cards[v.RaiseTheAlarm]
	ab := cd.Abilities[0]
	ab.Prog = []kop.Instr{{Op: kop.OpCreateToken, Obj: kop.RefController, A: kop.Const(60), C: int16(v.SoldierToken)}}
	cd.Abilities = []kop.Ability{ab}
	tbl.Cards[v.RaiseTheAlarm] = cd
	return &tbl
}

func TestStalledIsNotADraw(t *testing.T) {
	var d []kop.CardID
	for i := 0; i < 20; i++ {
		d = append(d, v.Plains, v.RaiseTheAlarm)
	}
	tbl := stallTable()
	defer dirac.ReleaseTable(tbl)
	cfg := lab.Config{Table: tbl, IDs: [][]kop.CardID{d, d}}
	rs, sum, err := lab.Batch(cfg, []lab.Policy{lab.Random(1), lab.Random(2)}, seeds(8), 1, 100000)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Stalls == 0 {
		t.Fatalf("no game stalled: %+v", sum)
	}
	for _, r := range rs {
		if r.Outcome == lab.Stalled && (r.Winner != -1 || r.Stall == "" || r.Class == "") {
			t.Fatalf("stalled game %+v", r)
		}
	}
	if sum.Draws != 0 || sum.Stalls+sum.Wins[0]+sum.Wins[1] != sum.Games {
		t.Fatalf("stalls are counted as %+v", sum)
	}
	if r := sum.WinRate(0); r.P > float64(sum.Wins[0])/float64(sum.Games) {
		t.Fatalf("rate %+v", r)
	}
}

func TestSummarizeKeepsStallsApart(t *testing.T) {
	rs := []lab.Result{
		{Outcome: lab.Stalled, Winner: -1, Stall: "board", Class: "capacity"},
		{Outcome: lab.Drawn, Winner: -1},
		{Outcome: lab.Won, Winner: 1},
	}
	s := lab.Summarize(2, rs)
	if s.Stalls != 1 || s.Draws != 1 || s.Wins[1] != 1 || s.StallsBy["board"] != 1 {
		t.Fatalf("%+v", s)
	}
	if got := s.WinRate(1).P; got < 0.33 || got > 0.34 {
		t.Fatalf("win rate %v counts the stall as a non-game", got)
	}
}

func TestWilson(t *testing.T) {
	r := lab.Wilson(50, 100)
	if r.P != 0.5 || r.Lo < 0.40 || r.Lo > 0.41 || r.Hi < 0.59 || r.Hi > 0.60 {
		t.Fatalf("%+v", r)
	}
	if z := lab.Wilson(0, 0); z != (lab.Rate{}) {
		t.Fatalf("%+v", z)
	}
}

func TestJSONL(t *testing.T) {
	rs, _, err := lab.Batch(vanillaCfg(2), []lab.Policy{lab.Random(1), lab.Random(2)}, seeds(2), 1, 30000)
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := lab.WriteJSONL(&b, rs); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(b.String()), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], `{"pair":0,"decks":["seat0","seat1"],"seed":1,"winner":`) ||
		!strings.Contains(lines[0], `"intents":`) {
		t.Fatalf("%q", lines)
	}
}

func TestSearchSeat(t *testing.T) {
	o := lab.SearchOptions(8)
	pols := []lab.Policy{lab.Search(o), lab.Random(1)}
	a, _, err := lab.Batch(vanillaCfg(2), pols, seeds(2), 1, 30000)
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := lab.Batch(vanillaCfg(2), []lab.Policy{lab.Search(o), lab.Random(1)}, seeds(2), 2, 30000)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) || a[0].Outcome == lab.Failed || a[0].Outcome == lab.Running {
		t.Fatalf("%+v vs %+v", a, b)
	}
}

type onlyPicker struct{}

func (onlyPicker) Pick(d *dirac.Decision) dirac.Action { return d.Options[0] }

func TestNonClonerNeedsOneWorker(t *testing.T) {
	pols := []lab.Policy{lab.FromPicker(onlyPicker{}), lab.Random(1)}
	if _, _, err := lab.Batch(vanillaCfg(2), pols, seeds(2), 2, 100); err == nil {
		t.Fatal("two workers shared a policy that cannot be cloned")
	}
	if _, _, err := lab.Batch(vanillaCfg(2), pols, seeds(2), 1, 100); err != nil {
		t.Fatal(err)
	}
}

func TestParseList(t *testing.T) {
	d, err := lab.ParseList("x", "# c\n20 Mountain\n4x Shock\nGrizzly Bears\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Cards) != 3 || d.Cards[1].Count != 4 || d.Cards[2].Name != "Grizzly Bears" || d.Cards[2].Count != 1 {
		t.Fatalf("%+v", d.Cards)
	}
}
