package azmcts

import (
	"context"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// netVecModel is a pure policynet model with a random K-way value head: the
// scalar output blocks InitValue drew are left in place, inert on a K-way
// model (policynet.Model's doc), exactly as a checkpoint that grew the
// vector head out of a scalar one loads.
func netVecModel(k int) *policynet.Model {
	m := policynet.NewModel(policynet.TableRows, 4, 3, rand.New(rand.NewPCG(61, 62)))
	m.InitValue(3, rand.New(rand.NewPCG(63, 64)))
	m.ValueK = k
	m.VOutWK = make([]float32, k*m.ValueHidden)
	m.VOutBK = make([]float32, k)
	for i := range m.VOutWK {
		m.VOutWK[i] = 0.1 * float32((i%13)-6)
	}
	for i := range m.VOutBK {
		m.VOutBK[i] = 0.05 * float32((i%7)-3)
	}
	return m
}

// netVecLeaf is a network leaf's value vector at e's position under a K-way
// head, computed straight from the head (the expected answer netLeafVec must
// reproduce): the head's K probabilities on the actor's projected and
// encoded view, a living seat r's entry its win component plus the no-winner
// probability over the living seats.
func netVecLeaf(t testing.TB, m *policynet.Model, e *rules.Engine, actor state.PlayerID) (vec [maxVecSeats]float64, nv uint8, p []float32) {
	t.Helper()
	sc := new(viewScratch)
	v := &sc.leaf
	view.ProjectLeanInto(v, e.G, e, actor, e.Pending(), m.Features.ViewOmit())
	sb := policynet.AcquireStateBuf()
	p = m.ValueVec(policynet.EncodeStateWithInto(sb, m.Features, v, actor, nil))
	policynet.ReleaseStateBuf(sb)
	n := len(e.G.Players)
	living := 0
	for i := range e.G.Players {
		if !e.G.Players[i].Lost {
			living++
		}
	}
	nv = uint8(n)
	for i := range e.G.Players {
		if e.G.Players[i].Lost {
			continue
		}
		r := relSeatOf(n, actor, e.G.Players[i].ID)
		vec[r] = float64(p[r]) + float64(p[len(p)-1])/float64(living)
	}
	return vec, nv, p
}

// envLeafAt is the engine env's Leaf at the root position e stands on, with
// m as its network, MaxN on, at a three-seat pod position.
func envLeafAt(t testing.TB, e *rules.Engine, d *decision.Decision, bot decision.Intent, m *policynet.Model) Leaf {
	t.Helper()
	obs := searchprobe.NewCollector(d.Player)
	cands, _, ok := enumerate(obs, e, d, bot, AllKinds(), 6)
	if !ok {
		t.Fatal("precondition: the position is not searchable")
	}
	keys := make([]Key, len(cands))
	for i, c := range cands {
		keys[i] = c.key
	}
	wc := &walkConfig{
		net: m, kinds: AllKinds(), limit: 6, maxSteps: 1000, envSeed: 1, actor: d.Player, maxN: true,
		root: &Point{Keys: keys, Prior: uniform(len(keys))}, rootCands: cands, rootDec: d, stats: &Stats{},
	}
	env, err := newEngineEnv(World{Engine: e.Clone(), Observer: obs.Clone()}, wc)
	if err != nil {
		t.Fatal(err)
	}
	return env.Leaf()
}

// TestMaxNUsesNetVector: a network leaf under a K-way value head carries the
// head's own vector — every living seat's entry its win component plus the
// no-winner probability over the living seats, a lost seat's 0, Vec[0] V —
// and not splitLeafVec's even split of the scalar. A scalar-headed network
// at the same position still runs the split (the fallback).
func TestMaxNUsesNetVector(t *testing.T) {
	e, d, bot := podPosition(t, podConfig(t, 3, testSeed+2), 4)
	if n := len(e.G.Players); n != 3 {
		t.Fatalf("precondition: %d seats, want 3", n)
	}
	living := 0
	for i := range e.G.Players {
		if !e.G.Players[i].Lost {
			living++
		}
	}

	m := netVecModel(5)
	l := envLeafAt(t, e, d, bot, m)
	vec, nv, p := netVecLeaf(t, m, e, d.Player)
	if l.NVec != nv {
		t.Fatalf("NVec %d, want %d", l.NVec, nv)
	}
	if l.Vec != vec {
		t.Fatalf("leaf vector %v, want the head's own %v (components %v)", l.Vec, vec, p)
	}
	if math.Float64bits(l.V) != math.Float64bits(l.Vec[0]) {
		t.Fatalf("V %v is not Vec[0] %v", l.V, l.Vec[0])
	}
	// A lost seat's entry is 0 and the components differ from splitLeafVec's
	// spread of the scalar — the fallback the fix replaced.
	split, _ := splitLeafVec(e.G, d.Player, l.V)
	if l.Vec == split {
		t.Fatal("the leaf took splitLeafVec's spread, not the head's vector")
	}
	for i := range e.G.Players {
		if e.G.Players[i].Lost && l.Vec[relSeatOf(3, d.Player, e.G.Players[i].ID)] != 0 {
			t.Fatalf("lost seat %d's entry %v, want 0", i, l.Vec)
		}
	}
	if living != 3 {
		t.Fatalf("precondition: %d living seats, want 3", living)
	}

	// The scalar-headed fallback: the same position under a model without a
	// K-way head splits its scalar exactly as before.
	scalar := netVecModel(0)
	ls := envLeafAt(t, e, d, bot, scalar)
	want, _ := splitLeafVec(e.G, d.Player, ls.V)
	if ls.NVec == 0 || ls.Vec != want {
		t.Fatalf("scalar leaf vector %v (NVec %d), want the split %v", ls.Vec, ls.NVec, want)
	}
}

// TestMaxNNetVecSearchChangesTheResult: a K-way-headed max^n search over a
// three-seat pod position differs from the scalar-headed one at the same
// position — the vector, not the split, drove the backups — and the search
// itself stays deterministic.
func TestMaxNNetVecSearchChangesTheResult(t *testing.T) {
	e, d, bot := podPosition(t, podConfig(t, 3, testSeed+2), 4)
	opts := podOpts()
	opts.MaxN = true
	scalar := netVecModel(0)
	vector := netVecModel(5)
	obs := searchprobe.NewCollector(d.Player)
	src, err := newTestClairvoyant(e, obs)
	if err != nil {
		t.Fatal(err)
	}
	paranoid, err := Search(context.Background(), Root{Engine: e, Decision: d, Bot: bot, Observer: obs}, src, scalar, opts)
	if err != nil {
		t.Fatal(err)
	}
	obs2 := searchprobe.NewCollector(d.Player)
	src2, err := newTestClairvoyant(e, obs2)
	if err != nil {
		t.Fatal(err)
	}
	vec, err := Search(context.Background(), Root{Engine: e, Decision: d, Bot: bot, Observer: obs2}, src2, vector, opts)
	if err != nil {
		t.Fatal(err)
	}
	if paranoid.Stats.OppPoints == 0 || vec.Stats.OppPoints == 0 {
		t.Fatalf("precondition: opponent points paranoid %d vector %d", paranoid.Stats.OppPoints, vec.Stats.OppPoints)
	}
	if diff := sameResult(paranoid, vec); diff == "" {
		t.Fatal("the K-way head's vector drove the same search as the scalar split")
	}
	obs3 := searchprobe.NewCollector(d.Player)
	src3, err := newTestClairvoyant(e, obs3)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Search(context.Background(), Root{Engine: e, Decision: d, Bot: bot, Observer: obs3}, src3, vector, opts)
	if err != nil {
		t.Fatal(err)
	}
	if diff := sameResult(vec, again); diff != "" {
		t.Fatalf("the K-way search is not deterministic: %s", diff)
	}
}
