package azmcts

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// podConfig is testConfig's two decks seated n times around the table,
// alternating.
func podConfig(t testing.TB, n int, seed uint64) rules.Config {
	t.Helper()
	cfg := testConfig(t, "mono-red-prowess", "mono-blue-tempo", seed)
	names, decks := make([]string, n), make([][]*cards.Card, n)
	for i := range names {
		names[i], decks[i] = cfg.Names[i%2], cfg.Decks[i%2]
	}
	cfg.Names, cfg.Decks = names, decks
	return cfg
}

// podPosition plays the default bot for every seat of cfg until seat 0
// faces a decision Search would search at turn >= minTurn, and returns the
// engine there with the bot's answer, unsubmitted.
func podPosition(t testing.TB, cfg rules.Config, minTurn int32) (*rules.Engine, *decision.Decision, decision.Intent) {
	t.Helper()
	n := len(cfg.Names)
	e := rules.New(cfg)
	e.Advance()
	rngs := searchprobe.BotRandoms(cfg.Seed, n)
	board := botpolicy.NewBoard(n)
	for steps := 0; steps < 20000 && !e.G.Over; steps++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no pending decision at step %d", steps)
		}
		in := botpolicy.Decide(botpolicy.BoardFromGameInto(e.G, e, d.Player, &board), d, rngs[d.Player])
		if d.Player == 0 && e.G.Turn >= minTurn {
			if _, _, ok := enumerate(searchprobe.NewCollector(0), e, d, in, AllKinds(), DefaultOptions().Limit); ok {
				return e, d, in
			}
		}
		if err := e.Submit(in); err != nil {
			t.Fatalf("step %d submit: %v", steps, err)
		}
	}
	t.Fatalf("%d seats, seed %d: no searchable seat-0 decision at turn >= %d", n, cfg.Seed, minTurn)
	return nil, nil, decision.Intent{}
}

// podSearch is one clairvoyant Search at the pod position with the
// opponents in the tree.
func podSearch(t testing.TB, e *rules.Engine, d *decision.Decision, bot decision.Intent, opts Options) Result {
	t.Helper()
	obs := searchprobe.NewCollector(d.Player)
	src, err := newTestClairvoyant(e, obs)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Search(context.Background(), Root{Engine: e, Decision: d, Bot: bot, Observer: obs}, src, nil, opts)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func podOpts() Options {
	o := DefaultOptions()
	o.Sims, o.Seed, o.OpponentNodes = 64, 7, true
	return o
}

// TestMaxNOffIsBitIdentical pins a four-seat search with the opponents in
// the tree to the visits, answer and values it returned before MaxN
// existed, and shows MaxN on is a different search at the same position.
func TestMaxNOffIsBitIdentical(t *testing.T) {
	e, d, bot := podPosition(t, podConfig(t, 4, testSeed), 4)
	off := podSearch(t, e, d, bot, podOpts())
	wantVisits := []int{27, 7, 8, 7, 8, 7}
	wantQ := []float64{0.5339867870998996, 0.34476586721717667, 0.3604542882497122, 0.3504919316638934, 0.37385917087387543, 0.3504919316638934}
	if fmt.Sprint(off.Visits) != fmt.Sprint(wantVisits) || off.Choice != 0 {
		t.Fatalf("MaxN off: visits %v choice %d, want %v choice 0", off.Visits, off.Choice, wantVisits)
	}
	for i, q := range wantQ {
		if math.Float64bits(off.Q[i]) != math.Float64bits(q) {
			t.Fatalf("MaxN off: Q[%d] %v, want %v", i, off.Q[i], q)
		}
	}
	if off.Stats.OppPoints == 0 {
		t.Fatal("precondition: no opponent point was selected at")
	}
	o := podOpts()
	o.MaxN = true
	on := podSearch(t, e, d, bot, o)
	if on.Stats.MaxNFallbacks != 0 || on.Stats.OppPoints == 0 {
		t.Fatalf("MaxN on: fallbacks %d, opponent points %d", on.Stats.MaxNFallbacks, on.Stats.OppPoints)
	}
	if sameResult(off, on) == "" {
		t.Fatal("MaxN on returned the paranoid search's result at a four-seat position")
	}
	if again := podSearch(t, e, d, bot, o); sameResult(on, again) != "" {
		t.Fatalf("MaxN on is not deterministic: %s", sameResult(on, again))
	}
	t.Logf("paranoid visits %v Q %v; max^n visits %v Q %v", off.Visits, off.Q, on.Visits, on.Q)
}

// TestMaxNNodeCacheIsExact: the node cache's stored nodes and ended edges
// carry the value vector, so a cached max^n search over a fixed world
// returns the uncached one.
func TestMaxNNodeCacheIsExact(t *testing.T) {
	e, d, bot := podPosition(t, podConfig(t, 4, testSeed), 4)
	o := podOpts()
	o.MaxN = true
	plain := podSearch(t, e, d, bot, o)
	o.NodeCache = 64
	cached := podSearch(t, e, d, bot, o)
	if cached.Stats.NodeResumes == 0 {
		t.Fatal("precondition: the cached search resumed no stored node")
	}
	if diff := sameResult(plain, cached); diff != "" {
		t.Fatalf("cached max^n search differs: %s", diff)
	}
}

// TestMaxNTwoSeatsTakesOldPath: with two living seats max^n and paranoid
// coincide, and the scalar path runs: the same Result and counters.
func TestMaxNTwoSeatsTakesOldPath(t *testing.T) {
	e, d, bot := podPosition(t, podConfig(t, 2, testSeed+1), 6)
	off := podSearch(t, e, d, bot, podOpts())
	o := podOpts()
	o.MaxN = true
	on := podSearch(t, e, d, bot, o)
	if off.Stats.OppPoints == 0 {
		t.Fatal("precondition: no opponent point was selected at")
	}
	if diff := sameResult(off, on); diff != "" || off.Stats != on.Stats {
		t.Fatalf("two seats, MaxN on differs: %s (stats equal %v)", diff, off.Stats == on.Stats)
	}
	g := state.NewGameLife([]string{"a", "b", "c", "d"}, 40)
	g.Players[1].Lost, g.Players[3].Lost = true, true
	var st Stats
	if maxNApplies(g, o, &st) || st.MaxNFallbacks != 0 {
		t.Fatal("a pod down to two living seats applied max^n")
	}
	g.Players[3].Lost = false
	if !maxNApplies(g, o, &st) {
		t.Fatal("a pod of three living seats did not apply max^n")
	}
}

// TestMaxNFallsBackAboveFourSeats: five seats are more than a value vector
// holds, so the search runs paranoid -- the MaxN-off Result -- and counts
// the fallback.
func TestMaxNFallsBackAboveFourSeats(t *testing.T) {
	e, d, bot := podPosition(t, podConfig(t, 5, testSeed), 3)
	if len(e.G.Players) <= maxVecSeats {
		t.Fatalf("precondition: %d seats", len(e.G.Players))
	}
	off := podSearch(t, e, d, bot, podOpts())
	o := podOpts()
	o.MaxN = true
	on := podSearch(t, e, d, bot, o)
	if on.Stats.MaxNFallbacks != 1 || off.Stats.MaxNFallbacks != 0 {
		t.Fatalf("fallbacks on %d off %d, want 1 and 0", on.Stats.MaxNFallbacks, off.Stats.MaxNFallbacks)
	}
	on.Stats.MaxNFallbacks = 0
	if diff := sameResult(off, on); diff != "" || off.Stats != on.Stats {
		t.Fatalf("five seats, MaxN on differs from paranoid: %s", diff)
	}
}

// TestHeuristicLeafVecSumsToOne: the living seats' entries sum to 1, a lost
// seat's is 0, the entries follow turn order from the actor, two seats
// reproduce heuristicLeafValue, and a finished game is one-hot (uniform
// over the living seats on a draw).
func TestHeuristicLeafVecSumsToOne(t *testing.T) {
	g := state.NewGameLife([]string{"a", "b", "c", "d"}, 40)
	g.Players[0].Life, g.Players[2].Life = 30, 50
	g.Players[3].Lost = true
	e := &rules.Engine{G: g}
	vec, nv := heuristicLeafVec(e, 1)
	if nv != 4 {
		t.Fatalf("NVec %d, want 4", nv)
	}
	// actor 1: relative 0 is seat 1, 1 seat 2, 2 seat 3 (lost), 3 seat 0.
	if vec[2] != 0 {
		t.Fatalf("lost seat's entry %v, want 0 (%v)", vec[2], vec)
	}
	if s := vec[0] + vec[1] + vec[3]; math.Abs(s-1) > 1e-12 {
		t.Fatalf("living entries sum to %v (%v)", s, vec)
	}
	if !(vec[1] > vec[0] && vec[0] > vec[3]) {
		t.Fatalf("entries %v not ordered by life (seat 2 50 > seat 1 40 > seat 0 30)", vec)
	}

	two := state.NewGameLife([]string{"a", "b"}, 20)
	two.Players[1].Life = 13
	e2 := &rules.Engine{G: two}
	pe, _, _ := podPosition(t, podConfig(t, 2, testSeed), 4)
	for _, en := range []*rules.Engine{e2, pe} {
		for actor := state.PlayerID(0); actor < 2; actor++ {
			v, _ := heuristicLeafVec(en, actor)
			want := heuristicLeafValue(en, actor)
			if math.Abs(v[0]-want) > 1e-12 || math.Abs(v[0]+v[1]-1) > 1e-12 {
				t.Fatalf("two seats, actor %d: vector %v, heuristicLeafValue %v", actor, v, want)
			}
		}
	}
	if heuristicLeafValue(pe, 0) == 0.5 {
		t.Fatal("precondition: the recorded two-seat position is level")
	}

	g.Over, g.Winner = true, 2
	vec, _ = heuristicLeafVec(e, 1)
	if vec != [maxVecSeats]float64{0, 1, 0, 0} {
		t.Fatalf("seat 2 won: vector %v, want one-hot at relative seat 1", vec)
	}
	g.Draw = true
	vec, _ = heuristicLeafVec(e, 1)
	if third := 1.0 / 3; vec != [maxVecSeats]float64{third, third, 0, third} {
		t.Fatalf("draw: vector %v, want uniform over the living seats", vec)
	}
}

// vecNode is one position of a hand-built game whose leaves carry a value
// vector, addressed like fakeNode by the path of keys played.
type vecNode struct {
	keys     []Key
	opp      bool
	seat     uint8
	vec      [maxVecSeats]float64
	terminal bool
}

type vecEnv struct {
	nodes map[string]vecNode
	path  string
}

func (f *vecEnv) Root() *Point {
	n := f.nodes[""]
	return &Point{Keys: n.keys, Prior: uniform(len(n.keys))}
}

func (f *vecEnv) Play(k Key) (*Point, error) {
	f.path += "/" + string(k)
	n, ok := f.nodes[f.path]
	if !ok {
		return nil, fmt.Errorf("fake: no node at %q", f.path)
	}
	if n.terminal {
		return nil, nil
	}
	return &Point{Keys: n.keys, Prior: uniform(len(n.keys)), Opp: n.opp, Seat: n.seat}, nil
}

func (f *vecEnv) Leaf() Leaf {
	n := f.nodes[f.path]
	return Leaf{V: n.vec[0], Vec: n.vec, NVec: 3, Terminal: n.terminal}
}

type vecSource struct{ nodes map[string]vecNode }

func (s vecSource) Env(int) (Env, error) { return &vecEnv{nodes: s.nodes}, nil }

// TestMaxNOpponentMaximisesOwnComponent: three seats; after the searching
// seat's a, opponent seat 1 replies x (its own best, 0.4, the searching
// seat's 0.5) or y (its own 0.1, the searching seat's worst, 0.2). Paranoid
// search has seat 1 play y, so a is worth 0.2 and b's sure 0.35 is taken;
// max^n has it play x, so a is worth 0.5 and is taken.
func TestMaxNOpponentMaximisesOwnComponent(t *testing.T) {
	nodes := map[string]vecNode{
		"":     {keys: []Key{"a", "b"}, vec: [maxVecSeats]float64{0.34, 0.33, 0.33}},
		"/a":   {keys: []Key{"x", "y"}, opp: true, seat: 1, vec: [maxVecSeats]float64{0.34, 0.33, 0.33}},
		"/a/x": {terminal: true, vec: [maxVecSeats]float64{0.5, 0.4, 0.1}},
		"/a/y": {terminal: true, vec: [maxVecSeats]float64{0.2, 0.1, 0.7}},
		"/b":   {terminal: true, vec: [maxVecSeats]float64{0.35, 0.35, 0.3}},
	}
	x, y := nodes["/a/x"].vec, nodes["/a/y"].vec
	if !(x[1] > y[1] && y[0] < x[0]) {
		t.Fatal("precondition: the opponent's own best reply must differ from the searching seat's worst")
	}
	for _, maxN := range []bool{false, true} {
		o := treeOpts(300)
		o.MaxN = maxN
		var st Stats
		tr, err := RunTree(context.Background(), &Point{Keys: []Key{"a", "b"}, Prior: []float64{0.5, 0.5}}, vecSource{nodes}, o, &st)
		if err != nil {
			t.Fatal(err)
		}
		want := 1
		if maxN {
			want = 0
		}
		if c := choose(tr.Visits, false, nil); c != want {
			t.Fatalf("maxN=%v: chose %d (visits %v, Q %v), want %d", maxN, c, tr.Visits, tr.Q, want)
		}
		if st.OppPoints == 0 {
			t.Fatalf("maxN=%v: precondition: no opponent point selected at", maxN)
		}
		t.Logf("maxN=%v: visits %v, Q %v", maxN, tr.Visits, tr.Q)
	}
	// The reply itself, by hand at the opponent's node: with both replies
	// visited equally, max^n selects x, paranoid y.
	nd := newNode(&Point{Keys: []Key{"x", "y"}, Prior: []float64{0.5, 0.5}})
	pt := &Point{Keys: []Key{"x", "y"}, Opp: true, Seat: 1}
	for _, kid := range nd.kids {
		kid.n, kid.avail = 10, 10
		v := nodes["/a/"+string(kid.key)].vec
		kid.w, kid.wv = 10*v[0], [maxVecSeats]float64{10 * v[0], 10 * v[1], 10 * v[2]}
		nd.n += 10
		nd.w += kid.w
		for r := range nd.wv {
			nd.wv[r] += kid.wv[r]
		}
	}
	o := treeOpts(1)
	if got := selectEdge(nd, pt, o).key; got != "y" {
		t.Fatalf("paranoid reply %q, want y", got)
	}
	o.MaxN = true
	if got := selectEdge(nd, pt, o).key; got != "x" {
		t.Fatalf("max^n reply %q, want x", got)
	}
}
