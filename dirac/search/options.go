package search

import (
	"fmt"
	"strings"
)

// Kinds are the searched decision kinds (azmcts Kinds).
type Kinds struct {
	Priority, Attackers, Blockers, Target bool
	// the fuller search's kinds (:morekinds)
	Modes, Optional, Choose, MultiTarget bool
}

// AllKinds is azmcts AllKinds; FullKinds adds the fuller kinds.
func AllKinds() Kinds { return Kinds{Priority: true, Attackers: true, Blockers: true, Target: true} }

func FullKinds() Kinds {
	k := AllKinds()
	k.Modes, k.Optional, k.Choose, k.MultiTarget = true, true, true, true
	return k
}

// ParseKinds parses a comma list of priority, attackers, blockers, target,
// modes, optional, choose, multitarget or all (FullKinds) into Kinds, the
// same names azmcts.ParseKinds takes (the :kinds= spec key). An unknown or empty entry is an error.
func ParseKinds(s string) (Kinds, error) {
	var k Kinds
	for _, part := range strings.Split(s, ",") {
		switch p := strings.TrimSpace(part); p {
		case "priority":
			k.Priority = true
		case "attackers":
			k.Attackers = true
		case "blockers":
			k.Blockers = true
		case "target":
			k.Target = true
		case "modes":
			k.Modes = true
		case "optional":
			k.Optional = true
		case "choose":
			k.Choose = true
		case "multitarget":
			k.MultiTarget = true
		case "all":
			k = FullKinds()
		default:
			return Kinds{}, fmt.Errorf("unknown searched kind %q (want priority, attackers, blockers, target, modes, optional, choose, multitarget, all)", p)
		}
	}
	return k, nil
}

// Options are one Search's knobs (azmcts Options, the subset the c6
// config uses plus the node cache).
type Options struct {
	Sims     int
	CPUCT    float64
	FPU      float64
	Limit    int // candidates per point, the bot's answer first
	MaxSteps int // environment submits per simulation
	Kinds    Kinds
	// PriorTopK > 0: every point is enumerated in full and the bot's
	// candidate is kept with the PriorTopK-1 others of highest prior.
	PriorTopK int
	// OpponentNodes puts the opponent's searched decisions in the tree;
	// OpponentMirror gives them the seat's settings (topk).
	OpponentNodes, OpponentMirror bool
	// MaxN replaces paranoid search at the opponents' points (OpponentNodes)
	// with max^n (gorge bots/azmcts Options.MaxN). Off (the default), every
	// opponent maximises 1 - Q, so with more than two seats the opponents act
	// as one coalition against the searching seat. On, in a game with more
	// than two living seats, every leaf also carries a value vector over the
	// seats (relative to the searching seat in turn order; a VecEvaluator's,
	// else splitVec's), backed up beside the scalar, and an opponent's point
	// maximises its own entry; the scalar is the vector's searching-seat
	// entry. With two living seats the two coincide and the scalar path runs
	// unchanged (Stats.MaxN stays 0). It acts only with OpponentNodes, and
	// Validate refuses it with NodeCache or Batch, whose backups carry the
	// scalar alone.
	MaxN               bool
	AbsoluteUnvisitedQ bool
	UnvisitedQ         float64
	// NodeCache > 0 enables the per-world node-state cache (only with
	// CachedWorlds: a fixed world per tree). Default off.
	NodeCache int
	// CachedWorlds K > 0: PIMC-K over K fixed deals (Sims/K simulations
	// each, roots merged by summed visits) instead of a fresh deal per
	// simulation.
	CachedWorlds int
	// ClosedList: the opponent's deck list is unknown; worlds deal its
	// hidden cards from a list drawn from Searcher.Belief (an open list --
	// the opponent's real hidden cards re-dealt -- when Belief is nil).
	ClosedList bool
	// InstWin != InstWinOff (instwin.go) restricts the seat's
	// own-turn instant-speed casts to a combat / response window.
	InstWin uint8
	// EndFire (:endfire, instwin.go): at the opponent's end step the
	// seat never passes while it can cast an instant-speed removal spell.
	EndFire bool
	Seed    uint64
	// Batch > 0 runs the tree with up to Batch simulations in flight
	// (batch.go): each descends its own world to a leaf, the leaves are
	// valued together through a BatchEvaluator, and an edge a descent is
	// still traversing is marked in flight so the next descent spreads
	// away from it. 0 (the default) is the serial path, unchanged.
	// Batch <= MaxBatch; it needs an Evaluator implementing
	// BatchEvaluator (otherwise Search falls back to the serial path) and
	// is exclusive with NodeCache and the early cut.
	Batch int
	// VirtualLoss is the Q penalty selection applies to an in-flight edge
	// (Batch > 0): Q - VirtualLoss*inFlight/(n+1). 0 disables it (batched
	// descents still see the marks, but a repeat choice is not
	// discouraged); 0 with Batch 0 is off.
	VirtualLoss float64
	// Noise and Sample are generation-mode exploration (:explore;
	// explore.go): Dirichlet(DirichletAlpha) root-prior noise with weight
	// DirichletEps (0 = azmcts's 0.3 / 0.25), and a visit-proportional
	// root pick. Off (the eval behaviour) by default. ChoiceSalt salts the
	// stream (azmcts Options.ChoiceSalt).
	Noise, Sample                bool
	DirichletAlpha, DirichletEps float64
	ChoiceSalt                   uint64

	// The self-play options (gorge bots/azmcts), each 0 = off, the
	// search exactly as without it. PriorRankMaxOptions and ForcedK are
	// per-search knobs read by Search; SimsByKind, the playout-cap fields
	// and the decided-game fields are the seat's (azmcts SeatConfig), read
	// once per decision by SeatSearch (seat.go) and ignored by Search.

	// PriorRankMaxOptions > 0 (azmcts Options.PriorRankMaxOptions): under
	// a PriorTopK cut, a point whose decision has more options than this
	// skips the network ranking (no Priors call) and keeps the bot's answer
	// and the enumeration's next PriorTopK-1 candidates under a uniform
	// prior, counted in Stats.PriorFallbacks.
	PriorRankMaxOptions int
	// ForcedK > 0 (azmcts Options.ForcedK; forced.go): forced playouts at
	// the root and policy target pruning into Result.Target.
	ForcedK float64
	// SimsByKind[k] > 0 (azmcts SeatConfig.SimsByKind) is the simulation
	// budget at a decision of searched kind k (KindNames; index 0 unused);
	// 0 uses Sims.
	SimsByKind [NumKinds]int32
	// FullProb > 0 (azmcts SeatConfig.FullProb; seat.go) is playout-cap
	// randomisation: a searched decision is a FULL search (the configured
	// one) with probability FullProb -- FullProbByKind[k] for a kind whose
	// bit is set in FullProbKinds -- else a CHEAP one: CheapSims, no root
	// noise, no sampling, no forced playouts, never recorded.
	FullProb       float64
	CheapSims      int
	FullProbByKind [NumKinds]float64
	FullProbKinds  uint16 // bit k: FullProbByKind[k] is set
	// DecidedHigh > 0 (azmcts SeatConfig.DecidedHigh; seat.go): once a
	// seat's last DecidedN completed searches all valued its position above
	// DecidedHigh, or all below 1-DecidedHigh, each later full search is a
	// DECIDED one -- DecidedSims, no root noise, no sampling, no forced
	// playouts -- recorded at training weight DecidedWeight (0: not at all).
	DecidedHigh   float64
	DecidedN      int
	DecidedSims   int
	DecidedWeight float64
	// EarlyCutGap / EarlyCutShare (with EarlyCutMin) stop a decision's
	// search before Sims (earlycut.go). Off by default.
	EarlyCutGap   bool
	EarlyCutShare float64
	EarlyCutMin   int
	// Clairvoyant is a diagnostic belief (:clairvoyant, never
	// hosted): every world keeps the opponent's real hand and deals its
	// library from its real remaining cards (order re-shuffled), the
	// ceiling any better belief could reach. Off by default.
	Clairvoyant bool
	// LazyPriors (default on) skips a replayed point's network ranking when
	// the point belongs to an existing node, no candidate cut is possible
	// (the count is within PriorTopK, or Limit without a top-k cut) and
	// every candidate key already has an edge: nothing then reads the
	// point's prior. Exact: TestLazyPriorsIsExact.
	LazyPriors bool

	// The instant-timing options: each off by default, the search exactly
	// as without it when off. timing.go has the rules, Validate's
	// validateTiming the cross-checks.
	//
	// RootQ / RootLCB (:rootq, :rootlcb, :rootqf=F, :rootlcbz=Z): the final
	// root move is the argmax of Q (or of Q - Z*sqrt(max(Q(1-Q),0.01)/n))
	// among children with visits >= max(2, RootQFrac*maxvisits) (RootQFrac
	// not > 0 = 0.1, RootLCBZ not > 0 = 1.96); never a sampled pick.
	RootQ, RootLCB      bool
	RootQFrac, RootLCBZ float64
	// Defer w > 0 (:defer=w, :noupkeep = defer=1): no own
	// instant-speed spell cast in the seat's own upkeep or draw unless it
	// responds to an opponent's stack object; own main-1 such casts have
	// prior x w.
	Defer float64
	// HoldUpkeep (:holdupkeep[=NAMES]): no cast in the
	// active player's own upkeep or draw with an empty stack; a bot answer
	// that is one is played as pass, unsearched. HoldUpkeepExempt names
	// cards never held (packed from 0, "" ends the list).
	HoldUpkeep       bool
	HoldUpkeepExempt [MaxCardNames]string
	// CardSims (:csims=NAME.N|...): a searched decision whose root
	// candidates cast or activate a listed card runs that entry's sims
	// (the largest); CardSimsForce (:csimsforce) keeps the listed cards'
	// moves among the root candidates past the topk=/cands= cut.
	CardSims      [MaxCardNames]CardBudget
	CardSimsForce bool
	// RaidHold (:raidhold[=NAMES]): no cast of a raid-trigger card
	// in the active player's precombat main with an empty stack while it
	// has a possible attacker; a bot answer that is one becomes pass, the
	// prior asked with the bot's own cast marked. RaidHoldOnly names the
	// raid cards held instead (packed from 0).
	RaidHold     bool
	RaidHoldOnly [MaxCardNames]string
	// PriorTemp T > 0 (:ptemp=T): the network prior becomes softmax(logit/T)
	// at the seat's points (a mirrored opponent's too), before topk=.
	PriorTemp float64
	// WaitTie EPS > 0 (:waittie=EPS, "when tied, wait"): at a
	// searched priority root whose chosen move is a cast or activation the
	// timing rules offer again (the stack empty; sorcery-speed in main 1,
	// instant-speed before the end step), with pass a candidate and both
	// above the RootQ visit floor, pass is played when Q(move) - Q(pass) <
	// EPS. WaitSorcOn (:waitsorc=EPS) gives sorcery-speed
	// moves their own margin WaitSorc (0: they never wait); off, WaitTie
	// is both margins. WaitTieUntap (:waittiehorizon=untap) extends an
	// instant-speed move's horizon to the seat's next untap step. PassMax
	// (:passmax) values pass at the best line after it (the
	// seat's next decision through the opponent's most-visited replies)
	// instead of its average.
	WaitTie, WaitSorc float64
	WaitSorcOn        bool
	WaitTieUntap      bool
	PassMax           bool
	// ReactRoot / ReactHorizon (:react, :reactin, :reacttarget), read with InstWin == InstWinReact: ReactHorizon takes the rule's
	// last window from the untap-step horizon (laterStep, OutlastsTurn)
	// instead of "the opponent's end step on", and lists the held root
	// casts for the record only with ReactTarget; ReactRoot holds at the
	// root only (:react without :reactin).
	ReactRoot, ReactHorizon bool
	ReactTarget             bool
	// SkipPass L > 0 (:skippass[=L], gorge azmcts Options.SkipPass): a simulation shortcut; the walk passes
	// every priority decision (either seat's) with an empty stack outside a
	// main phase -- at level 2 also the non-active seat's main-phase one --
	// without asking the bot or making it a point (skipWindow). Never the
	// root. 0 is off.
	SkipPass int

	// The land keys (landsearch.go), each off by
	// default and the search exactly as without it: LandSearch searches a
	// priority decision whose bot answer is a land play (that land
	// candidate 0); LandPick (needs LandSearch) adds every other distinct
	// land in hand as a candidate; DedupSearch offers a library search one
	// candidate per distinct card; LandAbNow plays a land's non-mana
	// ability (Evolving Wilds' crack) at once, unsearched.
	LandSearch, LandPick, DedupSearch, LandAbNow bool

	// AltCands, CastDedup: the MageZero like-for-like priority keys
	// (mzkeys.go), each off by default and the search exactly as without
	// it: AltCands makes plot, bestow, sneak, unlock and turn-face-up
	// options candidates; CastDedup merges cast candidates of one card.
	AltCands, CastDedup bool

	// HoldBonus, OppView, OppFair: the value and hidden-information keys
	// (:holdbonus, :oppview, :oppfair; hiddeninfo.go). 0 = off.
	HoldBonus, OppView, OppFair float64

	// The remaining instant-timing keys (instkeys.go has the rules),
	// each 0 / false = off, the search exactly as without it:
	// HoldEps (:holdeps=E) plays pass for a re-offered root cast with
	// probability E; CastPrior (:castprior=B) scales the seat's re-offered
	// casts' priors by B; DeferQ / DeferQMin (:deferq=EPS, :deferqmin=N)
	// pass where the tree values the same cast at the next window within
	// EPS; Replan (:replan) passes unchanged windows unsearched (SeatSearch);
	// RootSH (:rootsh) runs sequential halving at a castable priority root;
	// Reuse (:reuse=F) warm-starts the root from the previous search's pass
	// line (SeatSearch); InfoSeed (:infoseed) seeds the worlds by the seat's
	// information; ReactAim (:reactaim, with the horizon react) keeps an aimed
	// pre-combat cast and narrows its targets to the opponent's objects.
	HoldEps, CastPrior, DeferQ float64
	DeferQMin                  int
	Replan, RootSH             bool
	Reuse                      float64
	InfoSeed, ReactAim         bool
	// PerCreature (0 is off; :percreature=attack) is a bitmask of the
	// declarations searched one creature at a time (percreature.go):
	// PerCreatureAttack makes a searched attackers decision a chain of
	// points, one per eligible creature, each {not attacking} or an attack
	// at one of its defenders.
	PerCreature uint8
}

// PerCreature's bits.
const (
	PerCreatureAttack = 1
	PerCreatureBlock  = 2
)

// MaxCardNames bounds Options' card-name lists (HoldUpkeepExempt,
// RaidHoldOnly, CardSims): fixed arrays, so Options stays a comparable
// value SeatSearch copies without allocating.
const MaxCardNames = 8

// CardBudget is one Options.CardSims entry: a card by its table name
// (kop.CardDef.Name, as decks spell it) and its decisions' simulations.
type CardBudget struct {
	Name string
	Sims int32
}

// DefaultOptions are azmcts DefaultOptions with the c6 production config
// (az:sims=100:autopay:oppnodes:oppfull:morekinds:closedlist:topk=6).
func DefaultOptions() Options {
	return Options{Sims: 100, CPUCT: 1.5, FPU: 0.1, Limit: 6, MaxSteps: 1000, Kinds: FullKinds(),
		PriorTopK: 6, OpponentNodes: true, OpponentMirror: true, ClosedList: true, LazyPriors: true}
}
