package azmcts

// The node state cache (Options.NodeCache): over a FIXED world -- one
// position and one future chance for every simulation (FixedWorldSource) --
// a walk's state is a function of the keys it played, so a simulation
// resumes from the state saved at the deepest stored node of its path
// instead of re-walking the tree's edges from the root (upstream
// BenchSearch caches a game state at every node the same way). Measured
// before the cache (TestNodeCacheMeasure, 12 roots, clairvoyant): 73% of a
// simulation's env steps re-walked already-expanded edges at 100
// simulations, 89% at 1000, 91% at 3000.
//
// It is exact: with the cache on, the tree sees the same points, leaves and
// selections as without it, so the Result is byte-identical except the
// walk's cost counters (Stats.EnvSteps, Plays, ReplayPlays, ReplaySteps and
// the Node* counters) -- TestNodeCacheIsExact pins it over many roots, and
// TestNodeCacheIsExactWithBenchKnobs under the search benchmark's knobs. A
// node keeps what the walk needs that its point does not say: the engine
// clock there (the discount and the leaf-depth sums read it), the actor's
// auto-pay bot stream, and whether producing the point counted a Truncated
// or a PriorFallbacks (counted again when a cached walk passes it). It is
// never on for a source whose worlds differ between
// simulations (the redeal source, a re-seeded chance): there a node's state
// is not a function of its path.
//
// What is stored, and when:
//
//   - the root's state, by the first simulation that reaches it (one copy);
//   - a newly expanded node's state, while there is room: that walk is over,
//     so its env is kept as it stands, with no copy;
//   - an unstored node a simulation walks through (it was expanded while the
//     cache was full, or its state was evicted): while there is room, or by
//     evicting the least-visited stored node (never the root) when that node
//     has strictly fewer visits than this one (one copy);
//   - an edge whose walk ended (game over, the step cap): its leaf, so the
//     next simulation down it touches no engine at all.
//
// Options.NodeCache caps the stored node states (the root's included). A
// stored state owns its whole event log and object arena, so it is larger
// than a plain Clone (which shares the log): measured 2026-09-30 on
// mid-game 60-card roots, 300-450 KB each (log ~45%, arena ~35%), so the
// default 256 holds at most ~100 MB live, and only while a search with more
// than 256 expanded nodes runs. Eviction drops the state for the garbage
// collector and never releases its arrays for reuse: a stored state's
// descendants may share its event-log prefix (rules.Engine.Release's
// contract). The tree's END is different: every state, walker and clone of
// the tree is spent together, so a source that pools storage
// (NodeStateReclaimer, sparePool) takes all of them back at once and the
// next tree's resumes clone into recycled arrays instead of fresh ones.

import (
	"errors"
	"fmt"
	"math/rand/v2"

	"github.com/adams-shaun/gorge/bots/heuristic"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/engine/rules"
)

// DefaultNodeCache is DefaultOptions' node-state cap.
const DefaultNodeCache = 256

// nodeCache is one RunTree's stored node states.
type nodeCache struct {
	src NodeStateSource
	max int
	// stored holds every node whose snap is set; stored[0] is the root once
	// its state is saved. No map: eviction scans it in insertion order.
	stored []*node
}

func validPoint(pt *Point) bool {
	return pt != nil && len(pt.Keys) > 0 && len(pt.Prior) == len(pt.Keys)
}

// samePoint reports whether a replayed walk reached the point nd was
// expanded from: the fixed-world promise, checked on every replayed edge.
func samePoint(pt *Point, nd *node) bool {
	if pt == nil || nd.pt == nil || len(pt.Keys) != len(nd.pt.Keys) {
		return false
	}
	for i, k := range pt.Keys {
		if nd.pt.Keys[i] != k {
			return false
		}
	}
	return true
}

// save stores env's state at nd. final means env's walk is over: the state
// is taken as it stands, with no copy.
func (c *nodeCache) save(nd *node, env Env, final bool, st *Stats) {
	snap, err := c.src.Save(env, final)
	if err != nil || snap == nil {
		return
	}
	nd.snap = snap
	c.stored = append(c.stored, nd)
	st.NodeSaves++
}

// offer stores env's state at nd, an unstored tree node the walk has just
// reached, if there is room or a less-visited node to evict.
func (c *nodeCache) offer(nd *node, env Env, final bool, st *Stats) {
	if len(c.stored) < c.max {
		c.save(nd, env, final, st)
		return
	}
	if final {
		// A new leaf (one visit) never displaces a stored node: every
		// stored node has at least one visit and ties keep the incumbent.
		return
	}
	victim := -1
	for i := 1; i < len(c.stored); i++ { // stored[0] is the root: never evicted
		if victim < 0 || c.stored[i].n < c.stored[victim].n {
			victim = i
		}
	}
	if victim < 0 || c.stored[victim].n >= nd.n {
		return
	}
	c.stored[victim].snap = nil
	c.stored = append(c.stored[:victim], c.stored[victim+1:]...)
	st.NodeEvicts++
	c.save(nd, env, final, st)
}

// simulate is one simulation over the cache: RunTree's simulate with the
// walk's prefix skipped. The tree walk -- availability, new children,
// selection, backup -- is simulate's, read from each node's stored point
// (in a fixed world the point a node was expanded from is the point every
// later walk reaches there). The env is positioned at the deepest stored
// node on the selected path and plays only the edges below it; a replayed
// edge that reaches a different point than the tree holds is an error (the
// source broke its fixed-world promise), never a silent mismatch.
func (c *nodeCache) simulate(top *node, sim int, opts Options, st *Stats) error {
	var env Env
	if top.snap == nil {
		e, err := c.src.Env(sim)
		if err != nil {
			return err
		}
		env = e
		if top.n == 0 {
			l := env.Leaf()
			if l.Err != nil {
				return l.Err
			}
			top.n, top.w = 1, l.V
		}
		top.pt = env.Root()
		if !validPoint(top.pt) {
			top.pt = nil
			return fmt.Errorf("%w: a world offered a malformed point", ErrSubmit)
		}
		top.clk = clockOf(env, 0)
		c.save(top, env, false, st)
	}
	var (
		nodes   []*node
		path    []*edge
		marks   []*edge
		unavail int
		from    int // index in nodes of the node env is (or will be) positioned at
	)
	nd := top
	for {
		pt := nd.pt
		if !validPoint(pt) {
			return fmt.Errorf("%w: a world offered a malformed point", ErrSubmit)
		}
		for _, kid := range nd.kids {
			if hasKey(pt.Keys, kid.key) {
				marks = append(marks, kid)
			} else {
				unavail++
			}
		}
		for j, k := range pt.Keys {
			if nd.child(k) == nil {
				kid := &edge{key: k, prior: pt.Prior[j]}
				nd.kids = append(nd.kids, kid)
				marks = append(marks, kid)
			}
		}
		sel := selectAt(nd, nd == top, pt, opts, st)
		if pt.Opp {
			st.OppPoints++
		}
		nodes, path = append(nodes, nd), append(path, sel)
		if sel.next == nil {
			break
		}
		nd = sel.next
		if env == nil && nd.snap != nil {
			from = len(nodes)
		}
	}
	clock := make([]pathClock, len(nodes), len(nodes)+1)
	for i, n := range nodes {
		clock[i] = n.clk
	}
	last := path[len(path)-1]
	if last.end != nil {
		// No point below the root is produced again: count what producing
		// each would have (a re-walk's Truncated and PriorFallbacks).
		skipped(nodes[1:], st)
		l := *last.end
		if l.Terminal {
			st.Terminal++
		}
		if l.Capped {
			st.StepCapped++
		}
		commit(nodes, path, marks, append(clock, last.endClk), l.V, opts, st)
		st.Unavailable += unavail
		return nil
	}
	if env == nil {
		e, err := c.src.Resume(nodes[from].snap)
		if err != nil {
			return err
		}
		env = e
		if from > 0 {
			st.NodeResumes++
		}
	}
	skipped(nodes[1:from+1], st)
	for j := from; j < len(path); j++ {
		sel := path[j]
		replay, steps0 := sel.next != nil || sel.n > 0, st.EnvSteps
		next, err := env.Play(sel.key)
		st.Plays++
		if replay {
			st.ReplayPlays++
			st.ReplaySteps += st.EnvSteps - steps0
		}
		if err != nil {
			return err
		}
		if j < len(path)-1 {
			if !samePoint(next, nodes[j+1]) || clockOf(env, j+1) != nodes[j+1].clk {
				return fmt.Errorf("%w: a fixed world reached a different point than the tree holds", ErrSubmit)
			}
			if nodes[j+1].snap == nil {
				c.offer(nodes[j+1], env, false, st)
			}
			continue
		}
		l := env.Leaf()
		if l.Err != nil {
			return l.Err
		}
		if next == nil {
			if l.Terminal {
				st.Terminal++
			}
			if l.Capped {
				st.StepCapped++
			}
			end := l
			sel.end, sel.endClk = &end, clockOf(env, len(path))
			commit(nodes, path, marks, append(clock, sel.endClk), l.V, opts, st)
			st.Unavailable += unavail
			return nil
		}
		sel.next = newNode(next)
		sel.next.pt, sel.next.clk = next, clockOf(env, len(path))
		sel.next.n, sel.next.w = 1, l.V
		st.Expanded++
		if next.Opp {
			st.OppExpanded++
		}
		commit(nodes, path, marks, append(clock, sel.next.clk), l.V, opts, st)
		st.Unavailable += unavail
		c.offer(sel.next, env, true, st)
		return nil
	}
	return errors.New("azmcts: unreachable: the cached walk ended without a leaf")
}

// reclaim ends the tree: every stored state goes back to the source
// (NodeStateReclaimer) and the nodes forget them. RunTree calls it once,
// after its last simulation, when no walker is alive and nothing will
// resume a state again.
func (c *nodeCache) reclaim() {
	r, ok := c.src.(NodeStateReclaimer)
	if ok {
		snaps := make([]any, 0, len(c.stored))
		for _, nd := range c.stored {
			snaps = append(snaps, nd.snap)
		}
		r.Reclaim(snaps)
	}
	for _, nd := range c.stored {
		nd.snap = nil
	}
	c.stored = nil
}

// clockOf is env's engine clock at a point depth searched edges below the
// root: simulate's now().
func clockOf(env Env, depth int) pathClock {
	if pe, ok := env.(PathEnv); ok {
		return pathClock{plies: pe.Plies(), turn: pe.Turn()}
	}
	return pathClock{plies: depth}
}

// skipped counts, for stored points a cached walk reached without
// producing them, what producing each counted (Point.cut, Point.fell).
func skipped(nodes []*node, st *Stats) {
	for _, n := range nodes {
		if n.pt == nil {
			continue
		}
		if n.pt.cut {
			st.Truncated++
		}
		if n.pt.fell {
			st.PriorFallbacks++
		}
		countTopK(st, n.pt.ranked, n.pt.before)
		countPoint(st, n.pt)
	}
}

// NodeStateReclaimer is an optional NodeStateSource extension for a source
// that recycles its states' storage. RunTree calls Reclaim once, after its
// last simulation, with every state the tree still stores: the tree is
// over, no walker is alive, and no state will be resumed again, so the
// source may release every engine it handed out (rules.Engine.Release)
// and keep the storage for its next tree. A source that cannot promise the
// states die together (one whose worlds outlive the tree) does not
// implement it.
type NodeStateReclaimer interface {
	NodeStateSource
	Reclaim(snaps []any)
}

// sparePool is the recycled engine storage (rules.Spare) one seat's
// searches share, decision after decision. A fixed-world tree stores a
// state at nearly every node it expands (the walker's own engine, kept as
// it stands), so within a tree almost no walker is ever released and every
// Resume clones into fresh arrays: a mid-game log (~170 KB) and object
// arena (~125 KB) per simulation, allocated, zeroed and collected. At the
// tree's end every stored state is spent, and its storage comes here
// (fixedEnvs.Reclaim); the next tree's Resumes draw from it (get), so the
// clone copies its parent's history into an array that already exists.
// Capacity only: a recycled array is overwritten before it is read and
// zeroed past what the clone writes (rules.Spare), and no event, object,
// chain byte or search result depends on where an array came from.
//
// Every pooled Spare is detached from its event-array provenance
// (rules.Spare.Detach): a pooled array's parent may itself be pooled, and
// consumed and overwritten before the child is, so the only prefix claim
// that survives the pool is none. The pool holds at most max Spares (the
// rest are dropped for the collector) and pins nothing else.
type sparePool struct {
	spares []rules.Spare
	max    int
}

// newSparePool is a pool for trees of at most states stored states: that
// many Resumes draw from it per tree, and that many states refill it.
func newSparePool(states int) *sparePool {
	if states <= 0 {
		return nil
	}
	return &sparePool{max: states + 1}
}

// put pools sp, detached; an empty Spare, or one past the cap, is dropped.
func (p *sparePool) put(sp rules.Spare) {
	if p == nil || sp.Empty() || len(p.spares) >= p.max {
		return
	}
	sp.Detach()
	p.spares = append(p.spares, sp)
}

// get takes the Spare that fits e's clone best -- the smallest event array
// that holds e's history plus the fork slack (rules.Spare.Fits), so the
// larger arrays stay for the deeper states -- or, when none fits, the
// largest (its other storage still recycles and the clone's log allocates
// on its first append as a plain clone's does). The zero Spare when empty.
func (p *sparePool) get(e *rules.Engine) rules.Spare {
	if p == nil || len(p.spares) == 0 {
		return rules.Spare{}
	}
	best, fits := -1, false
	for i := range p.spares {
		sp := &p.spares[i]
		f := sp.Fits(e)
		switch {
		case best < 0:
			best, fits = i, f
		case f && !fits:
			best, fits = i, true
		case f == fits && ((f && sp.EventCap() < p.spares[best].EventCap()) || (!f && sp.EventCap() > p.spares[best].EventCap())):
			best = i
		}
	}
	sp := p.spares[best]
	last := len(p.spares) - 1
	p.spares[best] = p.spares[last]
	p.spares[last] = rules.Spare{}
	p.spares = p.spares[:last]
	return sp
}

// resumeSlack is the spare event capacity a Resume reserves past a history
// of n events when no pooled array fit it: enough for the walk's appends
// (under 128 in 99% of simulations, measured on a cached-worlds
// bench) and for the next trees' resumes, whose histories run a few
// hundred events past this one as the game goes on.
func resumeSlack(n int) int { return max(512, n/8) }

// fixedEnvs is worldEnvs over a FixedWorldSource: the NodeStateSource the
// node cache walks. It asks the world source for worlds until the root's
// state is saved; every later simulation is a copy of a saved state.
type fixedEnvs struct {
	*worldEnvs
	// prev is the last walker this source made (Resume, or a Save that
	// moved a walker onto its own copy): simulations run one at a time, so
	// it is spent when the next is resumed, and its arrays build that one.
	// A walker whose state was kept (a final Save) is never released.
	prev  *engineEnv
	spare rules.Spare
	// pool, when set, is the seat's recycled storage across trees
	// (sparePool): a Resume with no spare of its own draws from it, and
	// Reclaim refills it with every stored state's storage at the tree's
	// end.
	pool *sparePool
}

// Reclaim is NodeStateReclaimer: the tree is over, so the last walker, the
// unused spare and every stored state's engine are released into the pool.
// Without a pool the states are simply forgotten, as before. A stored
// state's log may still share its parent's event array (a clone that never
// appended): Release recycles only an array that is the engine's own, so
// no array enters the pool twice.
func (f *fixedEnvs) Reclaim(snaps []any) {
	if f.pool == nil {
		return
	}
	if f.prev != nil {
		if f.prev.e != nil {
			f.pool.put(f.prev.e.Release())
		}
		f.prev.e = nil
		f.prev = nil
	}
	f.pool.put(f.spare)
	f.spare = rules.Spare{}
	for _, snap := range snaps {
		s, ok := snap.(*envState)
		if !ok || s.e == nil {
			continue
		}
		f.pool.put(s.e.Release())
		s.e, s.obs, s.cands, s.oppObs = nil, nil, nil, nil
	}
}

// envState is one saved engineEnv position. Its engine and observer are
// never written again: Resume walks copies.
type envState struct {
	e     *rules.Engine
	obs   *searchprobe.Collector
	hyp   bool
	pcgs  []rand.PCG
	cands []cand
	steps int
	plies int
	root  *Point
	// actor is the auto-pay actor bot's source (engineEnv.actorPCG); nil
	// without autoPayment.
	actor *rand.PCG
	// oppObs are copies of the walk's opponent collectors
	// (engineEnv.oppObs, cloneObservers); nil without OpponentNodes or
	// before the walk met an opponent's decision.
	oppObs []*searchprobe.Collector
	// opp are the opponents' auto-pay bot sources (engineEnv.oppPCGs); nil
	// without oppAutoPay (OpponentMirror with AutoPayment).
	opp []*rand.PCG
}

func (f *fixedEnvs) Save(env Env, final bool) (snap any, err error) {
	ee, ok := env.(*engineEnv)
	if !ok || ee.e == nil || ee.cur == nil || ee.capped {
		return nil, errors.New("azmcts: the node cache saves an engine env at a point")
	}
	s := &envState{e: ee.e, obs: ee.obs, hyp: ee.hyp, cands: ee.cands, steps: ee.steps, plies: ee.plies, root: ee.root, pcgs: make([]rand.PCG, len(ee.pcgs)),
		oppObs: cloneObservers(ee.oppObs)}
	for i, p := range ee.pcgs {
		s.pcgs[i] = *p
	}
	if ee.actorPCG != nil {
		a := *ee.actorPCG
		s.actor = &a
	}
	s.opp = clonePCGs(ee.oppPCGs)
	if final {
		// The walk is over: the state is ee's own engine, kept as it stands.
		ee.e, ee.obs, ee.cur, ee.cands = nil, nil, nil, nil
		if f.prev == ee {
			f.prev = nil
		}
		return s, nil
	}
	defer func() {
		if p := recover(); p != nil {
			tracePanic("cache-save", p)
			snap, err = nil, fmt.Errorf("%w: node cache save: %v", ErrPanic, p)
		}
	}()
	// The walk goes on: it moves onto its own copy, and the state keeps
	// the engine it stood on.
	c := s.e.CloneInto(&f.spare)
	ee.e, ee.obs = c, s.obs.Clone()
	ee.cur = c.Pending()
	f.prev = ee
	return s, nil
}

func (f *fixedEnvs) Resume(snap any) (env Env, err error) {
	s, ok := snap.(*envState)
	if !ok {
		return nil, errors.New("azmcts: not a node cache state")
	}
	if f.prev != nil {
		if f.prev.e != nil {
			f.spare = f.prev.e.Release()
		}
		f.prev.e = nil
		f.prev = nil
	}
	if f.spare.Empty() {
		// No walker was released since the last Resume (the usual case:
		// the last walk's engine is now a stored state), so the clone is
		// built from the pool's recycled storage, when there is any.
		f.spare = f.pool.get(s.e)
	}
	fits := f.pool != nil && f.spare.Fits(s.e)
	defer func() {
		if p := recover(); p != nil {
			tracePanic("cache-resume", p)
			env, err = nil, fmt.Errorf("%w: node cache resume: %v", ErrPanic, p)
		}
	}()
	e := s.e.CloneInto(&f.spare)
	if f.pool != nil && !fits {
		// No pooled array held this history: the clone shares its parent's
		// and its first append would copy the history into a fresh array
		// grown by the forked log's slack. Reserve that array now instead,
		// with resumeSlack: the same one copy, into an array that will fit
		// the resumes of the next trees (sparePool) rather than only this
		// walk's appends. Capacity only (events.Log.Reserve).
		e.L.Reserve(len(e.L.Events) + resumeSlack(len(e.L.Events)))
	}
	rngs, pcgs := make([]*rand.Rand, len(s.pcgs)), make([]*rand.PCG, len(s.pcgs))
	for i := range s.pcgs {
		p := s.pcgs[i]
		pcgs[i] = &p
		rngs[i] = rand.New(pcgs[i])
	}
	var board *botpolicy.Board
	if f.cfg.envBoard != nil {
		board = f.cfg.envBoard.board(len(e.G.Players))
	} else {
		b := botpolicy.NewBoard(len(e.G.Players))
		board = &b
	}
	ee := &engineEnv{
		e: e, obs: s.obs.Clone(), hyp: s.hyp, cfg: f.cfg,
		rngs: rngs, pcgs: pcgs, board: board,
		cur: e.Pending(), cands: s.cands, steps: s.steps, plies: s.plies, root: s.root,
		oppObs: cloneObservers(s.oppObs),
	}
	if s.actor != nil {
		a := *s.actor
		ee.actorPCG = &a
		ee.actorBot = heuristic.NewBotOn(ee.actorPCG).EnableAutoPayMana()
	}
	if s.opp != nil {
		ee.oppPCGs = clonePCGs(s.opp)
		ee.oppBots = oppBotsOn(ee.oppPCGs)
	}
	f.prev = ee
	return ee, nil
}

// clonePCGs is a deep copy of a list of bot sources (nil entries kept).
func clonePCGs(pcgs []*rand.PCG) []*rand.PCG {
	if pcgs == nil {
		return nil
	}
	out := make([]*rand.PCG, len(pcgs))
	for i, p := range pcgs {
		if p != nil {
			c := *p
			out[i] = &c
		}
	}
	return out
}
