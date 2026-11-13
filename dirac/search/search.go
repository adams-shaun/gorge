package search

import (
	"github.com/adams-shaun/gorge/dirac"
	"math/rand/v2"
	"sync/atomic"
)

// Search is gorge bots/azmcts's AlphaZero search on dirac state: a PUCT
// tree over the searching seat's searched decisions (and, under
// OpponentNodes, the opponent's), each simulation a fresh honest world
// (root copied, unseen cards re-dealt by dirac's PCG from the
// per-decision seed), walked by the tree and the bot, expanded once,
// evaluated once and backed up. Everything is preallocated by
// NewSearcher; Search allocates nothing.

const maxDepth = 256
const maxMarks = 4096

// Stats are azmcts Stats' counters dirac search keeps.
type Stats struct {
	Searched, Skipped               int
	Simulations, Completed          int
	Terminal, StepCapped, Expanded  int
	Unavailable, EnvSteps           int
	Plays, ReplayPlays, ReplaySteps int
	OppPoints, OppExpanded          int
	TreeFull                        int // simulations that could not expand (node/edge capacity)
	NodeSaves, NodeResumes          int
	NodeEvicts                      int
	ReactHeld, ReactSingle          int // InstWinReact: the root held a cast; ... and was left with only the bot's answer
	WorldRoots                      int // worlds whose root decision differed from the root's (worldRoot)
	NoOps                           int // edges pruned as no-ops (an answer that undid itself)
	MarkedDealt                     int // cards Belief.Marked flags dealt into worlds
	// PriorFallbacks counts points whose network ranking
	// Options.PriorRankMaxOptions skipped (azmcts counts its other
	// fallbacks here too; dirac has none).
	PriorFallbacks int
	// RootPrior is 1 when Searcher.RootEval supplied this search's root
	// prior.
	RootPrior int
	// Options.ForcedK (forced.go; azmcts's counters): ForcedSearched is 1
	// for a search run with it; ForcedPlayouts counts the root selections
	// the forced rule changed; PrunedVisits and PrunedChildren are what
	// policy target pruning took out of Result.Target.
	ForcedSearched, ForcedPlayouts int
	PrunedVisits, PrunedChildren   int
	EarlyCut                       int // searches stopped before Sims (earlycut.go)
	// LeafPlies and LeafEdges are azmcts's leaf depth sums, added at each
	// backup walk ends with: the world's submits since its root (bot steps
	// and searched answers alike) and the tree edges the walk took. A
	// node-cache replay ending at a stored leaf (cache.go) adds neither.
	LeafPlies, LeafEdges int
	// PriorSkips counts points whose network ranking LazyPriors skipped
	// (every key already an edge of the point's node, no cut possible).
	PriorSkips int
	// The timing options' counters (timing.go), all 0 with them off.
	// HoldUpkeep: the root dropped a cast / played pass for a held bot
	// cast; in-walk decisions it changed (the seat's, a mirrored
	// opponent's). CardSims: the budget a CardSims entry set; candidates
	// CardSimsForce added. RaidHold: the root held a raid cast / passed in
	// place of the bot's; in-walk decisions it changed. WaitTie: the rule
	// played pass; the root met every condition but the tie. PassMaxUsed:
	// the wait compared the best line after pass instead of its average.
	HeldUpkeep, HeldUpkeepBot, HeldUpkeepWalk, HeldUpkeepOpp int
	CardBoosted, CardForced                                  int
	RaidHeld, RaidHeldBot, RaidHeldWalk, RaidHeldOpp         int
	WaitTie, WaitTieEligible, PassMaxUsed                    int
	SkipPassed                                               int // in-walk windows SkipPass passed
	// the land keys' counters (landsearch.go), all 0 with them off
	LandSearched, LandDeferred, LandPicked, LandWalk, LandOpp, DedupSearched, LandAbNow int
	// OppViewSims counts the simulations OppView/OppFair re-dealt the
	// seat's hand in (hiddeninfo.go).
	OppViewSims int
	// The remaining instant-timing keys (instkeys.go), all 0 with them off: HoldEps
	// played pass / could; seat points CastPrior scaled; DeferQ played pass
	// / compared; Replan's unsearched pass; RootSH picked the move; Reuse
	// warmed the root (and with how many visits); ReactAim's probes that
	// kept a cast and target points it narrowed.
	HoldEps, HoldEpsEligible, CastPriorPoints, DeferQ, DeferQEligible     int
	Replanned, RootSH, Reused, ReusedVisits, ReactAimKept, ReactAimPoints int
	// PerCreature's counters (percreature.go): root creature searches
	// run, walk points built per creature, and declarations the bot had to
	// complete because AkDone was not offered.
	ChainSteps, ChainPoints, ChainFallbacks int
	// MaxN is 1 when this search ran its tree under max^n (Options.MaxN,
	// maxNApplies); VecLeaves counts the leaves whose value vector the
	// evaluator wrote (VecEvaluator), not splitVec.
	MaxN, VecLeaves int
}

// Result is one Search: the root candidates (index 0 the bot's answer),
// their statistics and the choice.
type Result struct {
	N         int
	Keys      [MaxCands]Key
	Visits    [MaxCands]int32
	Q         [MaxCands]float64
	Avail     [MaxCands]int32
	Prior     [MaxCands]float64
	RootValue float64
	Choice    int
	Searched  bool
	// Kind is the root's searched kind code (KindNames; 0 when unsearched).
	Kind uint8
	// Sims is the search's simulation budget (Options.Sims as SeatSearch
	// set it for this decision: a record's sims).
	Sims int
	// the chosen answer: Acts[:NActs] played in order (a declaration is
	// several picks closed by AkDone)
	NActs int
	Acts  [MaxPerm + 1]dirac.Action
	Stats Stats
	// Target, under Options.ForcedK (HasTarget), is the policy target:
	// Visits with the forced playouts pruned out (forced.go). The record
	// carries it and Sample draws from it; the argmax is unchanged.
	Target    [MaxCands]int32
	HasTarget bool
	// Set by SeatSearch (seat.go): the decision was a playout-cap cheap or
	// a decided-game search, its record's training weight (1, or
	// DecidedWeight), and whether it is recorded.
	Cheap, Decided bool
	Weight         float64
	Record         bool
	// HeldSingle (InstWinReact): the rule left only the bot's pass and held
	// at least one cast; N is 1 (visits [1], prior [1], q and RootValue 0.5,
	// Sims 0) and nothing was searched.
	HeldSingle bool
	// The timing options (timing.go): VisitBest is the argmax-visits
	// child (Choice unless RootQ/RootLCB, WaitTie or Sample picked
	// another); Waited, under WaitTie, is 1 + the candidate the rule
	// replaced by pass (Choice is pass's); RaidBot, under RaidHold, is 1 +
	// the root option index of the bot's held raid cast (pass stood in).
	VisitBest, Waited, RaidBot int
	// CandAct / CandOne: a one-action candidate's action (0 otherwise).
	CandAct [MaxCands]dirac.Action
	CandOne [MaxCands]bool
}

// PolicyTarget is the visits a record carries: Target under ForcedK, else
// Visits (azmcts Result.policyTarget).
func (r *Result) PolicyTarget() *[MaxCands]int32 {
	if r.HasTarget {
		return &r.Target
	}
	return &r.Visits
}

// Play applies the chosen answer to s (positioned at the searched
// decision).
func (r *Result) Play(s *dirac.State) {
	for i := 0; i < r.NActs; i++ {
		s.Apply(r.Acts[i])
	}
}

// world is one simulation's private game.
type world struct {
	s      dirac.State
	d      dirac.Decision
	steps  int // bot-answered submits (MaxSteps counts these)
	plies  int // every submit since the root
	capped bool
	react  reactTrack // InstWinReact: the world's turn tracker (the seat's)
}

func (w *world) copyFrom(src *world) {
	w.s.CopyFrom(&src.s)
	w.d = src.d
	w.steps, w.plies, w.capped = src.steps, src.plies, src.capped
	w.react = src.react
}

// Searcher holds one search's preallocated tree, worlds and scratch.
type Searcher struct {
	Opts   Options
	Eval   Evaluator
	Bot    Bot
	Belief *Belief // closed-list corpus (Options.ClosedList)
	// bel is the learned beliefs' state (beliefnet.go SetBeliefs; nil:
	// off), belBuf its storage kept across SetBeliefs(nil, nil)
	bel, belBuf *beliefState
	// RootEval, when set and Eval supplies a network prior (PriorSource),
	// is a second evaluator read only at the search root: the root point's
	// prior and the root node's initial value. Every other point and every
	// leaf reads Eval. Nil: the search exactly as without it.
	RootEval Evaluator

	netPrior bool
	seat     uint8
	seed     uint64
	envSeed  uint64
	epoch    uint32 // the root's epoch (dirac State.SetEpoch)
	// the evaluator's optional hooks, asserted once per evaluator (an
	// interface assertion can allocate its itab)
	hooked Evaluator
	wr     WorldReset
	bs     BaseSetter
	be     BatchEvaluator // Options.Batch's batched leaf value (nil: serial)
	ve     VecEvaluator   // Options.MaxN's leaf value vector (nil: splitVec)
	vr     VecReporter    // whether ve has a vector for the root's seats (nil: it always does)
	veOn   bool           // this search's leaves read ve (maxNApplies, and vr agrees at the root)
	// vec is the last leaf's value vector under max^n (maxn.go)
	vec [dirac.MaxSeats]float64
	// RootEval's hooks, asserted once per RootEval value; atRoot is true
	// only while the root point is built with RootEval; rootV/haveRootV
	// are the root's initial value from RootEval for this Search.
	rootHooked Evaluator
	rootWR     WorldReset
	atRoot     bool
	haveRootV  bool
	rootV      float64
	seen       [dirac.MaxObj]bool

	t       tree
	root    dirac.State
	rootD   dirac.Decision
	rootPt  point
	wpt     point // a world's own root point (worldRoot)
	w       world
	tmp     dirac.State
	td      dirac.Decision
	pts     [2]point
	logits  [dirac.MaxOptions]float32
	botPick [dirac.MaxOptions]bool

	pathN   [maxDepth]int32
	pathE   [maxDepth]int32
	marks   [maxMarks]int32
	pool    [2 * dirac.MaxDeck]dirac.CardID
	seenC   [2 * dirac.MaxDeck]dirac.CardID
	cache   nodeCache
	cacheOn bool
	// hint is the node the next successfully built point belongs to, or -1
	// for none (root, world root, expansion, node-cache walk): buildPoint's
	// LazyPriors skip reads it.
	hint int32
	// noLazyBot disables advance's lazyPoint shortcut, so the bot is asked
	// at every point exactly as before it: tests use it to compare the two
	// paths (TestLazyPointSearchIdentical).
	noLazyBot bool
	// InstWinReact (instwin.go): rootReact is the seat's tracker at the
	// root (SeatSearch copies it from the SeatState), curReact the one the
	// walk's buildPoint and instBot read; inRoot marks the root's
	// buildPoint, whose pruned casts priorityCands lists in held[:nHeld].
	rootReact reactTrack
	curReact  *reactTrack
	inRoot    bool
	held      [MaxCands]uint8
	nHeld     int
	merged    Result
	St        Stats

	// The timing options (timing.go): holdDrops counts HoldUpkeep's
	// drops and walk passes; rootBuild marks the root's buildPoint
	// (CardSimsForce); raidMark is 1 + the bot's held raid cast while its
	// point is built (the prior's bot mark); rootDec marks the root's bot
	// answer and point (ReactRoot); tmWalk is HoldUpkeep || RaidHold.
	holdDrops int
	raidMark  int
	rootBuild bool
	rootDec   bool
	tmWalk    bool
	landRoot  bool // the root is a LandSearch land root (Search; timing.go isLandRoot)

	// the batched path's per-descent state (batch.go): worlds and the
	// paths/marks each descent commits when its leaf value arrives. Only
	// touched with Options.Batch > 0; the serial path uses w, pathN/pathE
	// and marks above.
	worlds  [MaxBatch]world
	bpathN  [MaxBatch][maxDepth]int32
	bpathE  [MaxBatch][maxDepth]int32
	bmarks  [MaxBatch][maxMarks]int32
	bdepth  [MaxBatch]int32
	bnm     [MaxBatch]int32
	bnode   [MaxBatch]int32 // expansion node awaiting its value, -1 none
	beval   [MaxBatch]int32 // descent index per pending evaluation
	bstates [MaxBatch]*dirac.State
	bdecs   [MaxBatch]*dirac.Decision
	bout    [MaxBatch]float32
	batchN  int
	// Trace, when set, records the root after every simulation
	// (earlycut.go CutTrace).
	Trace *CutTrace
	// generation-mode exploration (explore.go)
	pcg   rand.PCG
	rng   *rand.Rand
	noisy [MaxCands]float64
	// a Declarer bot's whole declaration (declWalk)
	declBuf [MaxPerm]dirac.Action
	declL   []dirac.Action
	declS   *dirac.State // declL's decision (pickBot), nil none
	declD   *dirac.Decision
	// OppView/OppFair (hiddeninfo.go): hidOn for the search; the simulation's
	// re-deal draw, whether it happened, its update mode and index; markLo
	// is each path depth's first availability mark
	hidOn, hidSim, hidDealt bool
	hidMode                 uint8
	hidN                    uint64
	markLo                  [maxDepth]int32
	pathOpp                 [maxDepth]bool
	hidDealtCommits         int // re-dealt simulations backed up (tests)
	// The remaining instant-timing keys' state (instkeys.go)
	ik instKeyState
	// PerCreature's chain state (percreature.go): chainCur and
	// chainRoot are 1+obj of the creature a point decides (0 none)
	chainOn, inChain bool
	chainCur         dirac.Obj
	chainRoot        dirac.Obj
	chainX           dirac.Obj
	chainFrom        dirac.Obj // 1+obj of the blocker scan's start at the root (0 the chain's start)
	chainForced      [MaxPerm]uint32
	nForced          int
	chainPt          point
}

// WorldReset is an optional Evaluator extension told when a simulation
// starts a new world (an evaluator caching its last position drops it).
type WorldReset interface{ NewWorld() }

// lines hands out line-of-play ids (dirac State.SetLine), unique in the
// process so an evaluator shared between searchers never mistakes one
// searcher's line for another's.
var lines atomic.Uint64

// epochs numbers search roots across every Searcher of the process.
var epochs atomic.Uint32

// nextLine is a fresh line-of-play id (never 0).
func (sr *Searcher) nextLine() uint64 { return lines.Add(1) }

// hooks caches the evaluator's optional interfaces.
func (sr *Searcher) hooks() {
	if sr.rootHooked != sr.RootEval {
		sr.rootHooked = sr.RootEval
		sr.rootWR, _ = sr.RootEval.(WorldReset)
	}
	if sr.hooked == sr.Eval {
		return
	}
	sr.hooked = sr.Eval
	sr.wr, _ = sr.Eval.(WorldReset)
	sr.bs, _ = sr.Eval.(BaseSetter)
	sr.be, _ = sr.Eval.(BatchEvaluator)
	sr.ve, _ = sr.Eval.(VecEvaluator)
	sr.vr, _ = sr.Eval.(VecReporter)
}

// BaseSetter is an evaluator that keys on dirac's change marks: it is
// handed the search root once its epoch is set, before any world is
// cloned from it.
type BaseSetter interface{ SetBase(s *dirac.State) }

// PriorSource is an Evaluator that supplies a real prior (a network):
// PriorTopK only applies with one (azmcts priorTopK needs a net).
type PriorSource interface{ HasPrior() bool }

// NewSearcher preallocates a searcher for opts (the only allocation).
func NewSearcher(opts Options, eval Evaluator, bot Bot) *Searcher {
	if eval == nil {
		eval = Uniform{}
	}
	if bot == nil {
		bot = Heuristic{}
	}
	sr := &Searcher{Opts: opts, Eval: eval, Bot: bot, hint: -1}
	if ps, ok := eval.(PriorSource); ok {
		sr.netPrior = ps.HasPrior()
	}
	nodes := MaxSims(&opts) + 2
	sr.t.nodes = make([]node, nodes)
	sr.t.edges = make([]edge, nodes*MaxCands)
	if opts.MaxN {
		sr.t.nwv = make([][dirac.MaxSeats]float64, nodes)
		sr.t.ewv = make([][dirac.MaxSeats]float64, nodes*MaxCands)
	}
	if opts.NodeCache > 0 {
		sr.cache.init(opts.NodeCache)
	}
	return sr
}

// UseForkPool makes the States the Searcher steps fork on p. They then share p
// with whatever else uses it, so all of those must run on one goroutine.
func (sr *Searcher) UseForkPool(p *dirac.ForkPool) {
	sr.root.UseForkPool(p)
	sr.tmp.UseForkPool(p)
	sr.w.s.UseForkPool(p)
	sr.ik.aimS.UseForkPool(p)
	for i := range sr.worlds {
		sr.worlds[i].s.UseForkPool(p)
	}
}

// DecisionSeed is azmcts DecisionSeed: the per-decision seed from the
// seat's seed and the decision's sequence number.
func DecisionSeed(seatSeed, seq uint64) uint64 { return splitmix(seatSeed ^ splitmix(seq)) }

func splitmix(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}

// redealSeed is azmcts RedealSeed.
func redealSeed(seed uint64, sim int) (uint64, uint64) {
	a := splitmix(seed ^ 0x72656465616c2d77 ^ splitmix(uint64(sim)+1))
	return a, splitmix(a ^ 0x9e3779b97f4a7c15)
}

// Search runs the tree at root's pending decision (root must be at a
// decision of the seat to search: call StepInto first) and returns the
// answer. A decision that is not a searched point plays the bot's answer.
func (sr *Searcher) Search(root *dirac.State, seatSeed uint64) *Result {
	sr.root.CopyFrom(root)
	sr.root.StepInto(&sr.rootD)
	sr.hint = -1
	// a fresh epoch: every world is cloned from the root, so an object a
	// world has not changed is the root's (dirac marks.go)
	// unique across Searchers: an evaluator shared by several (its
	// base-slot cache is keyed by the epoch) never mistakes another
	// search's root for this one's
	sr.epoch = epochs.Add(1)
	if sr.epoch == 0 {
		sr.epoch = epochs.Add(1)
	}
	sr.root.SetEpoch(sr.epoch)
	sr.root.SetLine(sr.nextLine())
	sr.hooks()
	if sr.bs != nil {
		sr.bs.SetBase(&sr.root)
	}
	res := &sr.merged
	*res = Result{}
	sr.St = Stats{}
	sr.haveRootV = false
	sr.chainOn = sr.Opts.PerCreature != 0
	d := &sr.rootD
	if d.Kind == dirac.DkNone {
		return res
	}
	if sr.chainOn && !sr.inChain && (d.Kind == dirac.DkAttackers || d.Kind == dirac.DkBlockers) {
		if r := sr.chainSearch(seatSeed); r != nil {
			return r
		}
	}
	sr.seat = d.Player
	sr.t.vec = sr.maxNApplies() // maxn.go
	sr.veOn = sr.t.vec && sr.ve != nil && (sr.vr == nil || sr.vr.HasValueVec(sr.root.NumSeats()))
	sr.nHeld = 0
	sr.hidOn, sr.hidMode, sr.hidDealtCommits = sr.Opts.infoOn(), hidAll, 0
	react := sr.Opts.InstWin == InstWinReact
	if react {
		sr.rootReact.observe(&sr.root, sr.seat)
		sr.curReact = &sr.rootReact
	}
	sr.seed = DecisionSeed(seatSeed, uint64(sr.root.SeqNo()))
	sr.envSeed = splitmix(sr.seed ^ 0x656e762d73656564)
	sr.ik.wseed = sr.seed
	n := sr.root.NumObjs()
	for o := 0; o < n; o++ {
		sr.seen[o] = !sr.root.Hidden(dirac.Obj(o), sr.seat)
	}
	// objects a simulation creates (tokens) are not the root's: clear what
	// an earlier, larger root left past n, or their keys name stale objects
	clear(sr.seen[n:])
	if sr.Opts.InfoSeed {
		sr.ik.wseed = sr.infoSeed(seatSeed) // instkeys.go
	}
	sr.rootDec = true
	sr.tmWalk = sr.Opts.HoldUpkeep || sr.Opts.RaidHold
	bot := sr.endFireBot(&sr.root, d, sr.instBot(&sr.root, d, sr.pickBot(&sr.root, d)))
	if sr.Opts.RaidHold {
		bot = sr.timingRootBot(d, bot, res) // timing.go
	}
	drops := sr.holdDrops
	useRoot := sr.RootEval != nil && sr.netPrior
	if useRoot && sr.rootWR != nil {
		sr.rootWR.NewWorld()
	}
	sr.atRoot = useRoot
	sr.inRoot = react && (!sr.Opts.ReactHorizon || sr.Opts.ReactTarget)
	sr.rootBuild = true
	abNow := sr.Opts.LandAbNow && sr.landAbNow(&sr.root, d, &bot) // landsearch.go
	if abNow {
		sr.St.LandAbNow = 1
	}
	built := !abNow && sr.Opts.Sims > 0 && sr.buildPoint(&sr.root, d, bot, &sr.rootPt)
	sr.atRoot, sr.inRoot = false, false
	sr.rootBuild, sr.raidMark, sr.rootDec = false, 0, false
	if sr.holdDrops != drops {
		sr.St.HeldUpkeep = 1
	}
	if !built {
		if sr.Opts.HoldUpkeep {
			bot = sr.timingRootHeld(d, bot) // timing.go
		}
		sr.St.Skipped = 1
		if sr.nHeld > 0 {
			sr.St.ReactHeld = 1
			sr.nHeld = min(sr.nHeld, MaxCands-1)
			if d.Kind == dirac.DkPriority && d.Options[bot].Kind() == dirac.AkPass {
				// the rule left only the bot's pass: a record of the held casts
				sr.St.ReactSingle = 1
				res.N, res.Visits[0], res.Prior[0], res.Q[0], res.RootValue = 1, 1, 1, 0.5, 0.5
				res.HeldSingle = true
				res.Kind = uint8(kindPriority)
			}
		}
		res.NActs = 1
		res.Acts[0] = d.Options[bot]
		if (d.Kind == dirac.DkAttackers || d.Kind == dirac.DkBlockers) && d.Options[bot].Kind() != dirac.AkDone {
			// the bot's whole declaration
			var picks [MaxPerm]uint32
			n := sr.botDeclaration(&sr.root, d, bot, &picks)
			for i := 0; i < n; i++ {
				res.Acts[i] = dirac.Action(picks[i])
			}
			res.Acts[n] = dirac.MakeAction(dirac.AkDone, 0, 0, 0)
			res.NActs = n + 1
		}
		res.Stats = sr.St
		return res
	}
	if useRoot {
		sr.rootV, sr.haveRootV = float64(sr.RootEval.Value(&sr.root, d, sr.seat)), true
	}
	sr.St.Searched = 1
	if sr.t.vec {
		sr.St.MaxN = 1
	}
	if sr.nHeld > 0 {
		sr.St.ReactHeld = 1
		sr.nHeld = min(sr.nHeld, MaxCands-sr.rootPt.n)
	}
	if sr.bel != nil {
		sr.beliefRoot(&sr.root, sr.seat) // beliefnet.go
	}
	sims := sr.Opts.Sims
	if sr.Opts.CardSims[0].Name != "" {
		sims = sr.cardRootSims(sims) // cardsims.go
	}
	res.Kind, res.Sims = sr.kindAt(&sr.root, d), sims
	if sr.Opts.DedupSearch && libSearch(d) {
		sr.St.DedupSearched = 1
	}
	if sr.Opts.ForcedK > 0 {
		sr.St.ForcedSearched = 1
	}
	pt := &sr.rootPt
	res.N = pt.n
	for i := 0; i < pt.n; i++ {
		res.Keys[i], res.Prior[i] = pt.keys[i], pt.prior[i]
		if res.CandOne[i] = pt.off[i+1]-pt.off[i] == 1; res.CandOne[i] {
			res.CandAct[i] = dirac.Action(pt.acts[pt.off[i]])
		}
	}
	// LandSearch's land root (landsearch.go): no root noise, and Sample
	// defers the drop only for a line above it
	landRoot := sr.Opts.LandSearch && d.Options[bot].Kind() == dirac.AkPlayLand
	sr.landRoot = landRoot
	sr.explore(pt.n, landRoot)
	sr.ik.shOn, sr.ik.warmOn = false, false
	if sr.Opts.RootSH || sr.Opts.Reuse > 0 {
		sr.ik.shOn = sr.Opts.RootSH && d.Kind == dirac.DkPriority && shRoot(res) // instkeys.go
		if sr.Opts.Reuse > 0 {
			sr.reuseMatch()
		}
	}
	// Batch > 0 runs the batched path when the evaluator has one; without
	// a BatchEvaluator it keeps the serial path. Trace (the early-cut
	// scorer) needs one record per simulation, so it keeps the serial
	// path too (Options.Validate refuses Batch with the early cut itself).
	useBatch := sr.Opts.Batch > 0 && sr.be != nil && sr.Trace == nil
	if k := sr.Opts.CachedWorlds; k > 0 {
		per := sims / k
		for wi := 0; wi < k; wi++ {
			if useBatch {
				sr.runTreeBatch(per, wi, true)
			} else {
				sr.runTree(per, wi, true)
			}
			sr.mergeRoot(res, wi == 0)
		}
		for i := 0; i < res.N; i++ {
			if res.Visits[i] > 0 {
				res.Q[i] /= float64(res.Visits[i])
			}
		}
	} else {
		if useBatch {
			sr.runTreeBatch(sims, 0, false)
		} else {
			sr.runTree(sims, 0, false)
		}
		sr.mergeRoot(res, true)
		for i := 0; i < res.N; i++ {
			if res.Visits[i] > 0 {
				res.Q[i] /= float64(res.Visits[i])
			}
		}
	}
	// argmax visits, ties to the lower index (the bot's answer wins ties)
	best := 0
	for i := 1; i < res.N; i++ {
		if res.Visits[i] > res.Visits[best] {
			best = i
		}
	}
	res.VisitBest = best
	if sr.ik.shOn {
		best = sr.shBest(res, best) // instkeys.go
	}
	if sr.Opts.RootQ || sr.Opts.RootLCB || sr.Opts.WaitTie > 0 || sr.Opts.WaitSorcOn || sr.Opts.DeferQ > 0 {
		best = sr.finalMove(res, d, best) // timing.go
	}
	played := &res.Visits
	if sr.Opts.ForcedK > 0 {
		// the argmax is unchanged by the pruning; a sampled move is drawn
		// from the pruned target (azmcts)
		sr.pruneTarget(res)
		played = &res.Target
	}
	if sr.Opts.Sample {
		best = sr.sampleVisits(res.N, played, best)
		if landRoot {
			best = sr.landDraw(best, played)
		}
	}
	if sr.Opts.HoldEps > 0 {
		best = sr.holdEps(res, best) // instkeys.go
	}
	res.Choice = best
	res.NActs = 0
	for j := pt.off[best]; j < pt.off[best+1]; j++ {
		res.Acts[res.NActs] = dirac.Action(pt.acts[j])
		res.NActs++
	}
	if landRoot {
		sr.landStats(best)
	}
	res.Stats = sr.St
	return res
}

// RootCand is candidate i of the last Search's root (Result order, the
// bot's answer first): dirac actions it plays, a declaration's picks
// closed by AkDone. Valid until the next Search.
func (sr *Searcher) RootCand(i int) []uint32 {
	pt := &sr.rootPt
	return pt.acts[pt.off[i]:pt.off[i+1]]
}

// mergeRoot adds the tree's root children into res (visits summed, Q
// accumulated as visit-weighted value sums; divided at the end).
func (sr *Searcher) mergeRoot(res *Result, first bool) {
	top := &sr.t.nodes[0]
	i := 0
	for e := top.first; e >= 0 && i < res.N; e = sr.t.edges[e].sib {
		ed := &sr.t.edges[e]
		res.Visits[i] += ed.n
		res.Q[i] += ed.w
		res.Avail[i] += ed.avail
		i++
	}
	if first {
		res.RootValue = top.q()
	}
}

// runTree is azmcts RunTree over sims simulations. fixed: one world for
// every simulation (CachedWorlds tree wi), which the node cache may use.
func (sr *Searcher) runTree(sims, wi int, fixed bool) {
	sr.t.reset()
	top := sr.t.newNode(&sr.rootPt)
	sr.noiseRoot(top)
	if sr.ik.shOn {
		sr.shInit(sims) // instkeys.go
	}
	if sr.ik.warmOn && top >= 0 {
		sr.reuseWarm(top)
	}
	useCache := fixed && sr.Opts.NodeCache > 0
	cut := !fixed && (sr.Opts.EarlyCutGap || sr.Opts.EarlyCutShare > 0 || sr.Trace != nil)
	if sr.Trace != nil {
		sr.Trace.N = 0
	}
	sr.cacheOn = useCache
	if useCache {
		sr.cache.reset()
	}
	for i := 0; i < sims; i++ {
		sr.St.Simulations++
		if useCache {
			if sr.cache.simulate(sr, wi) {
				sr.St.Completed++
			}
			continue
		}
		t0 := profStart()
		sr.newWorld(&sr.w, i, wi, fixed)
		if sr.hidOn {
			sr.hidBegin(i)
		}
		profAdd(phRedeal, t0)
		if pt := sr.worldRoot(&sr.w); pt != nil && sr.simulate(0, pt) {
			sr.St.Completed++
		}
		if cut && sr.earlyCut(i+1, sims) {
			sr.St.EarlyCut++
			return
		}
	}
}

// newWorld positions w at the root of simulation sim's honest world.
func (sr *Searcher) newWorld(w *world, sim, wi int, fixed bool) {
	w.s.CopyFrom(&sr.root)
	w.steps, w.plies, w.capped = 0, 0, false
	var a, b uint64
	if fixed {
		a, b = redealSeed(sr.ik.wseed, wi)
		b = splitmix(sr.ik.wseed ^ splitmix(uint64(wi)+0x51))
	} else {
		a, b = redealSeed(sr.ik.wseed, sim)
	}
	sr.redeal(&w.s, a, b)
	w.s.SetLine(sr.nextLine())
	w.s.StepInto(&w.d)
	w.react = sr.rootReact
	if sr.wr != nil {
		sr.wr.NewWorld()
	}
}

// worldRoot is the root point in the current world: the root's own,
// unless the world poses another root decision -- the decision shows
// cards the redeal changed (a look at the searching seat's re-dealt
// library) -- then the world's own point, rebuilt; nil when the world's
// game ends before a searched point.
func (sr *Searcher) worldRoot(w *world) *point {
	if sameDecision(&w.d, &sr.rootD) {
		return &sr.rootPt
	}
	sr.St.WorldRoots++
	if !sr.advance(w, &sr.wpt) {
		return nil
	}
	return &sr.wpt
}

func sameDecision(a, b *dirac.Decision) bool {
	if a.Kind != b.Kind || a.Sub != b.Sub || a.Player != b.Player || a.N != b.N {
		return false
	}
	for i := 0; i < int(a.N); i++ {
		if a.Options[i] != b.Options[i] {
			return false
		}
	}
	return true
}

// redeal deals the unseen cards of world s (azmcts RedealSource.World ->
// searchprobe Redealer.Deal): per seat in order, the searching seat's
// library order, the opponent's hand and library (from a closed-list draw
// under ClosedList); the world's chance is re-seeded with b.
func (sr *Searcher) redeal(s *dirac.State, a, b uint64) {
	r := dirac.NewRand(a, b)
	for p := uint8(0); p < s.NumSeats(); p++ {
		if p == sr.seat || sr.Opts.Clairvoyant {
			// the seat's own library; under Clairvoyant the opponent's
			// real hand stays and its real library is re-shuffled
			s.RedealOwnLibrary(p, &r)
			continue
		}
		// a hand revealed to the searching seat (its choice from it is
		// pending) is kept; only the library is dealt
		revealed := s.HandRevealed(p, sr.seat)
		n := s.HandSize(p) + s.LibrarySize(p)
		if revealed {
			n = s.LibrarySize(p)
		}
		if sr.bel != nil {
			sr.beliefRedeal(s, p, sr.seat, &r, n, revealed) // beliefnet.go
			continue
		}
		pool := sr.pool[:n]
		if sr.Opts.ClosedList && sr.Belief != nil {
			ns := s.SeenCards(p, sr.seat, sr.seenC[:])
			seen := sr.seenC[:ns]
			for i := 1; i < ns; i++ {
				for j := i; j > 0 && seen[j] < seen[j-1]; j-- {
					seen[j], seen[j-1] = seen[j-1], seen[j]
				}
			}
			var mask uint8
			for _, c := range seen {
				mask |= CardColours(s.Table(), c)
			}
			sr.Belief.deal(&r, seen, mask, n, pool)
			sr.St.MarkedDealt += sr.Belief.marked(pool)
		} else if revealed {
			h := s.HandSize(p)
			m := s.HiddenCards(p, sr.pool[:h+n])
			copy(pool, sr.pool[h:m])
		} else {
			s.HiddenCards(p, pool)
		}
		if revealed {
			r.ShuffleCards(pool)
			s.DealLibrary(p, pool)
			continue
		}
		s.DealHidden(p, &r, pool)
	}
	s.ReseedChance(b, b^0x9e3779b97f4a7c15)
}

// leaf evaluates w for the searching seat (azmcts engineEnv.Leaf).
func (sr *Searcher) leaf(w *world) (v float64, terminal bool) {
	s := &w.s
	if s.Over() {
		if sr.t.vec {
			sr.overVec(s) // maxn.go
		}
		switch s.Winner() {
		case -1:
			return 0.5, true
		case int8(sr.seat):
			return 1, true
		}
		return 0, true
	}
	if s.NumSeats() > 2 && s.FeatLost(sr.seat) {
		if sr.t.vec {
			sr.liveVec(s, &w.d, true)
		}
		return 0, true // the game plays on without the searching seat (CR 800.4a)
	}
	if sr.t.vec {
		v = sr.liveVec(s, &w.d, false)
	} else {
		v = float64(sr.Eval.Value(s, &w.d, sr.seat))
	}
	if sr.Opts.HoldBonus != 0 {
		v += sr.holdBonus(s)
		sr.vec[0] = v // only read under max^n: the bonus shapes the seat's entry as it shapes v
	}
	return v, false
}

// play submits candidate idx of pt in w and advances to the next
// searched point (written to next); false means the walk ended (game over
// or the step cap).
//
// undone reports that the candidate's own answer was taken back (dirac
// State.Undos: an unpayable either-or part undoing the cast), the position
// being the one before it: the edge is a no-op.
func (sr *Searcher) play(w *world, pt *point, idx int, next *point) (ok, undone bool) {
	if sr.chainOn {
		if ok, undone, done := sr.chainPlay(w, pt, idx, next); done {
			return ok, undone
		}
	}
	u0 := w.s.Undos()
	k0, p0 := w.d.Kind, w.d.Player // the decision pt answers
	for j := pt.off[idx]; j < pt.off[idx+1]; j++ {
		a := dirac.Action(pt.acts[j])
		if j > pt.off[idx] && !Offered(&w.s, a, k0, p0, &sr.td) {
			continue
		}
		w.s.Apply(a)
	}
	if w.s.Undos() != u0 {
		return false, true
	}
	w.plies++
	return sr.advance(w, next), false
}

// advance answers every non-searched decision with the bot until the next
// searched point (azmcts engineEnv.advance): the bot's answer is computed
// first at every decision; the step cap is checked before a bot submit.
func (sr *Searcher) advance(w *world, next *point) bool {
	react := sr.Opts.InstWin == InstWinReact
	if react {
		sr.curReact = &w.react
	}
	for {
		w.s.StepInto(&w.d)
		if w.d.Kind == dirac.DkNone {
			return false
		}
		if react {
			w.react.observe(&w.s, sr.seat)
		}
		if sr.Opts.SkipPass > 0 && w.plies > 0 && sr.skipWindow(w) { // timing.go
			continue
		}
		abNow := sr.Opts.LandAbNow && landAbility(&w.s, &w.d) >= 0 // landsearch.go
		if !sr.noLazyBot && sr.hint >= 0 && !abNow && sr.lazyPoint(&w.s, &w.d, next) {
			return true
		}
		bot := sr.endFireBot(&w.s, &w.d, sr.instBot(&w.s, &w.d, sr.pickBot(&w.s, &w.d)))
		if abNow && sr.landAbNow(&w.s, &w.d, &bot) {
			// the land ability is played at once, unsearched (landsearch.go)
		} else if sr.tmWalk {
			var ok bool
			if bot, ok = sr.timingWalk(&w.s, &w.d, bot, next); ok { // timing.go
				if sr.Opts.LandSearch && w.d.Options[bot].Kind() == dirac.AkPlayLand {
					sr.landWalk(&w.d)
				}
				return true
			}
		} else if sr.buildPoint(&w.s, &w.d, bot, next) {
			if sr.Opts.LandSearch && w.d.Options[bot].Kind() == dirac.AkPlayLand {
				sr.landWalk(&w.d)
			}
			return true
		}
		if w.steps >= sr.Opts.MaxSteps {
			w.capped = true
			return false
		}
		u0 := w.s.Undos()
		w.s.Apply(w.d.Options[bot])
		w.steps++
		w.plies++
		sr.St.EnvSteps++
		if w.s.Undos() != u0 {
			// the bot's answer was taken back: it would answer the same
			// again and loop; the walk ends here, valued by the leaf
			w.capped = true
			return false
		}
	}
}

// simulate is azmcts simulate from node ni at point pt in the current
// world. It reports whether a value was backed up.
func (sr *Searcher) simulate(ni int32, pt *point) bool {
	t := &sr.t
	if t.nodes[0].n == 0 {
		v := sr.rootV
		if !sr.haveRootV {
			v, _ = sr.leaf(&sr.w)
		} else if t.vec {
			splitVec(&sr.w.s, sr.seat, v, &sr.vec) // RootEval values the seat alone
		}
		t.nodes[0].n, t.nodes[0].w = 1, v
		if t.vec {
			t.nwv[0] = sr.vec
		}
	}
	return sr.walk(ni, pt, 0, 0, 0, 0)
}

// walk continues a simulation from node ni at point pt in sr.w with depth
// path entries and nm availability marks already recorded.
func (sr *Searcher) walk(ni int32, pt *point, depth, nm, unavail, cur int) bool {
	t := &sr.t
	for {
		nd := &t.nodes[ni]
		if sr.hidOn && depth < maxDepth {
			sr.markLo[depth] = int32(nm)
		}
		for e := nd.first; e >= 0; e = t.edges[e].sib {
			if pt.has(t.edges[e].key) {
				if nm < maxMarks {
					sr.marks[nm] = e
					nm++
				}
			} else {
				unavail++
			}
		}
		for j := 0; j < pt.n; j++ {
			if t.child(ni, pt.keys[j]) < 0 {
				if e := t.addEdge(ni, pt.keys[j], pt.prior[j]); e >= 0 && nm < maxMarks {
					sr.marks[nm] = e
					nm++
				}
			}
		}
		t0 := profStart()
		sel := sr.selectAt(ni, pt) // forced.go
		profAdd(phBackup, t0)
		if sel < 0 {
			// every available edge is a no-op: so is the edge here
			if depth > 0 {
				t.edges[sr.pathE[depth-1]].dead = true
				sr.St.NoOps++
			}
			return false
		}
		if pt.opp {
			sr.St.OppPoints++
			if sr.hidOn && sr.hidSim && !sr.hidDealt {
				sr.hidRedeal(&sr.w.s)
			}
		}
		if depth >= maxDepth {
			sr.St.TreeFull++
			return false
		}
		sr.pathN[depth], sr.pathE[depth] = ni, sel
		if sr.hidOn {
			sr.pathOpp[depth] = pt.opp
		}
		depth++
		ed := &t.edges[sel]
		replay, steps0 := ed.next >= 0 || ed.n > 0, sr.St.EnvSteps
		nextPt := &sr.pts[cur]
		cur ^= 1
		t0 = profStart()
		sr.hint = -1
		if sr.Opts.LazyPriors && !sr.cacheOn {
			sr.hint = ed.next
		}
		ok, undone := sr.play(&sr.w, pt, pt.index(ed.key), nextPt)
		sr.hint = -1
		if undone {
			// the answer was taken back: the position is the one before
			// it, so the edge is a no-op (a loop); prune it
			ed.dead = true
			sr.St.NoOps++
			return false
		}
		if ed.next >= 0 {
			profAdd(phReplay, t0)
		} else {
			profAdd(phExpand, t0)
		}
		sr.St.Plays++
		if replay {
			sr.St.ReplayPlays++
			sr.St.ReplaySteps += sr.St.EnvSteps - steps0
		}
		if !ok {
			v, term := sr.leaf(&sr.w)
			if term {
				sr.St.Terminal++
			}
			if sr.w.capped {
				sr.St.StepCapped++
			}
			if sr.cacheOn {
				ed.ended, ed.endV, ed.endTerm, ed.endCap = true, v, term, sr.w.capped
			}
			sr.St.LeafPlies += sr.w.plies
			sr.St.LeafEdges += depth
			sr.commit(depth, nm, v)
			sr.St.Unavailable += unavail
			return true
		}
		if ed.next < 0 {
			v, _ := sr.leaf(&sr.w)
			nn := t.newNode(nextPt)
			if nn >= 0 && sr.cacheOn {
				sr.cache.offer(sr, nn, nextPt, true)
			}
			if nn < 0 {
				sr.St.TreeFull++
				return false
			}
			ed = &t.edges[sel]
			ed.next = nn
			if !sr.hidOn || sr.hidWants(nextPt.opp) {
				t.nodes[nn].n, t.nodes[nn].w = 1, v
				if t.vec {
					t.nwv[nn] = sr.vec
				}
			}
			sr.St.Expanded++
			if nextPt.opp {
				sr.St.OppExpanded++
			}
			sr.St.LeafPlies += sr.w.plies
			sr.St.LeafEdges += depth
			sr.commit(depth, nm, v)
			sr.St.Unavailable += unavail
			return true
		}
		ni, pt = ed.next, nextPt
		if sr.cacheOn && t.nodes[ni].snap < 0 {
			sr.cache.offer(sr, ni, pt, false)
		}
	}
}

// commit backs v up the walk (no discount: azmcts's default), and under
// max^n the leaf's value vector sr.vec beside it.
func (sr *Searcher) commit(depth, nm int, v float64) {
	t0 := profStart()
	if sr.hidOn && sr.hidMode != hidAll {
		sr.commitInfo(depth, nm, v)
		profAdd(phBackup, t0)
		return
	}
	t := &sr.t
	for i := 0; i < depth; i++ {
		nd := &t.nodes[sr.pathN[i]]
		nd.n++
		nd.w += v
		e := &t.edges[sr.pathE[i]]
		e.n++
		e.w += v
	}
	if t.vec {
		for i := 0; i < depth; i++ {
			t.addVec(sr.pathN[i], sr.pathE[i], &sr.vec)
		}
	}
	for i := 0; i < nm; i++ {
		t.edges[sr.marks[i]].avail++
	}
	profAdd(phBackup, t0)
}

// Offered reports whether a, a later action of a multi-action candidate
// (a declaration's picks and closing Done, a multi-pick choice) answering a
// decision of kind for player, is offered at s's decision, which must
// still be that decision: dirac answers a decision with a single
// option in place (pass elision), so a candidate's last pick or closing
// Done can find the game already past it -- at the opponent's blockers
// decision, whose own Done would end it with no blocks -- and is dropped.
// scratch receives the decision.
func Offered(s *dirac.State, a dirac.Action, kind dirac.DecisionKind, player uint8, scratch *dirac.Decision) bool {
	s.StepInto(scratch)
	if scratch.Kind != kind || scratch.Player != player {
		return false
	}
	for i := 0; i < int(scratch.N); i++ {
		if scratch.Options[i] == a {
			return true
		}
	}
	return false
}
