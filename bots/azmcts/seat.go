package azmcts

import (
	"context"
	"errors"
	"fmt"

	"github.com/adams-shaun/gorge/bots/heuristic"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/explain"
	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/bots/searchseat"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// SeatConfig is the az seat's configuration.
type SeatConfig struct {
	// Search are the per-decision knobs; Seed, Noise and Sample are set per
	// decision by the seat and ignored here.
	Search Options
	// SimsByKind, when set, overrides Search.Sims at a decision of a listed
	// kind: a per-kind simulation budget, e.g. more for casting (priority)
	// and fewer for blocks. An entry of 0, and every kind not listed, uses
	// Search.Sims; Search.Sims <= 0 still turns the search off. Keys are
	// decision.Kinds members and entries >= 0 (NewSeat refuses others). Only
	// looked up, never ranged.
	SimsByKind map[decision.Kind]int
	// Explore is generation mode (ticket 3's azgen): Dirichlet noise at every
	// searched root, and moves sampled proportional to visits while the game
	// turn is <= ExploreTurns (spec §2: turns 1-4). Eval leaves it false:
	// argmax, no noise.
	Explore      bool
	ExploreTurns int32
	// NoNoise drops the root Dirichlet noise from Explore: moves are still
	// sampled proportional to visits on turns <= ExploreTurns, but the
	// recorded visit distribution is the search's own (M1b distillation: at a
	// small budget the root noise would be a large share of every target).
	NoNoise bool
	// PriorOnly is the STUDENT seat (M1b): no simulation at all; at every
	// decision Search would search, play the argmax of the network prior
	// over the same candidates (ties on the lowest index, the bot's answer).
	// It reads only the seat's own decision and redacted view, never a
	// world, so it is honest. With no network the prior is uniform and the
	// seat is exactly the bot it wraps (the same-seed control).
	PriorOnly bool
	// RecordFeatures is the feature set SetRecorder's records encode under.
	RecordFeatures policynet.FeatureSet
	// World is the world source: WorldClairvoyant ("" is clairvoyant, the
	// stage-1 default), whose source is injected as Source, or WorldRedeal,
	// the honest source.
	World string
	// Source is the clairvoyant world source, required when World is "" or
	// WorldClairvoyant: at each searched decision it builds the source
	// (bots/azmcts/clairvoyant's NewClairvoyant under that package's
	// AllowClairvoyant). azmcts never links the real-engine clone itself, so
	// only a bench or training command injects it; NewSeat refuses a
	// clairvoyant world with a nil Source.
	Source func(searchseat.Env, *searchprobe.Collector) (WorldSource, error)
	// Worlds is the redeal source's K (RedealSource): 0 deals a fresh world
	// per simulation. Ignored by the clairvoyant source.
	Worlds int
	// Belief is WorldRedealClosed's model of the opponents' lists, required
	// by that world and ignored by the others. Shared and read-only.
	Belief *searchprobe.Belief
	// Mulligan is the London-mulligan rule of the seat's own bot (heuristic.Bot.
	// WithMulligan), which answers every KMulligan ask (the round is never
	// searched). The zero value is the default bot's 1/3 coin.
	Mulligan botpolicy.MulliganRule
	// FullProb, CheapSims and FullProbByKind are KataGo's playout-cap
	// randomization (pcr.go): with FullProb > 0 each searched root
	// decision is a FULL search with probability FullProb (its kind's
	// FullProbByKind entry when there is one), else a CHEAP one. A full
	// search is the seat's usual one: Search.Sims (SimsByKind), root noise
	// and visit sampling as Explore says, and it is recorded (SetRecorder).
	// A cheap search runs CheapSims simulations with no root noise, plays
	// the most-visited candidate (Search.Temp and Search.Second still apply
	// when set) and is never recorded. The draw is a pure function of the
	// seat's seed and the decision's sequence number (PCRFull). Decisions
	// inside a simulation (the opponent's points) are unaffected. FullProb 0
	// is off: every searched decision is full, the seat exactly as without
	// these fields. NewSeat refuses FullProb outside [0,1].
	FullProb float64
	// CheapSims is a cheap search's simulation budget, >= 1 whenever a
	// decision can be cheap.
	CheapSims int
	// FullProbByKind overrides FullProb at a decision of a listed searched
	// kind, by its ID (SearchKind; SearchKindOf resolves the :kinds= names
	// priority, attackers, blockers, target, modes, optional, choose,
	// multitarget); each in [0,1], 0 making every decision of that kind
	// cheap. Needs FullProb > 0. Only looked up, never ranged.
	FullProbByKind map[SearchKind]float64
	// DecidedHigh, DecidedN, DecidedSims and DecidedWeight are KataGo's
	// fewer visits once the game is decided (decided.go): once DecidedN
	// consecutive searched root values of the seat are all above
	// DecidedHigh, or all below 1 - DecidedHigh, each later full search is
	// a DECIDED one -- DecidedSims simulations, no root noise, the
	// most-visited candidate (Search.Temp and Search.Second still apply
	// when set), no forced playouts -- recorded with training
	// weight DecidedWeight (VisitRecord.Weight; 1 records it as usual, 0
	// not at all), until a searched root value leaves that side's band. A
	// playout-cap cheap decision stays cheap. Only searches that completed
	// (no refusal, not every simulation failed) count. DecidedHigh 0 is off
	// (the default): the seat exactly as without these fields, whatever
	// the other three say; else DecidedHigh in (0.5, 1), DecidedN >= 1,
	// DecidedSims >= 1 and DecidedWeight in [0, 1].
	DecidedHigh   float64
	DecidedN      int
	DecidedSims   int
	DecidedWeight float64
}

// DefaultSeatConfig is the eval seat with the spec's knobs.
func DefaultSeatConfig() SeatConfig {
	return SeatConfig{Search: DefaultOptions(), ExploreTurns: 4, RecordFeatures: policynet.FeaturesMZ}
}

// Diag is one decision of a searched kind as the cost report sees it.
type Diag struct {
	Turn       int32
	Kind       string // priority, attackers, blockers, target (or a fuller kind, ExtraKindNames); "" for a FeedStopped record
	Searched   bool   // a tree was built
	Candidates int
	Choice     int
	MS         float64 // wall ms of the whole decision (Millis); 0 when untimed
	Stats      Stats
	// Refused is the redeal source's refusal reason at this decision, "" when
	// it prepared (or the source is clairvoyant); DealFailed its first
	// per-world deal failure (those simulations counted in Stats.NoWorld).
	Refused, DealFailed string
	// Seed and Seq locate the decision: the seat's seed (NewSeat's) and the
	// decision's sequence number, so a driver can name the game a failure
	// came from. Diagnostic only.
	Seed uint64
	Seq  uint64
	// Cheap marks a searched decision playout-cap randomization made cheap
	// (SeatConfig.FullProb): CheapSims, no root noise, argmax, unrecorded.
	Cheap bool
	// Decided marks a decided-game search (SeatConfig.DecidedHigh):
	// DecidedSims, no root noise, argmax, recorded at DecidedWeight.
	Decided bool
}

// Millis is the monotonic elapsed-milliseconds clock the driving command
// installs before any game starts (this package may not import time --
// internal/archtest). Nil means untimed. It never reaches an answer.
var Millis func() float64

// Watch receives one Diag per decision of a searched kind the seat answers,
// plus one FeedStopped record per decision the driver routed around the
// search. Installed once before any game starts; games run on several
// goroutines, so the consumer synchronises itself. Nil is silent.
var Watch func(Diag)

// Seat is the az policy: the default bot (heuristic.NewBot, the same PCG
// derivation, so its delegation consumes exactly the bot's stream) with
// Search answering the searched kinds. It is a searchseat.SearchSeat, so
// testkit/bench.PlayGame hands it the live engine at its own decisions --
// the route the L10 search seat takes. Stage 1 reads only the engine; the
// driver's observation feed is for ticket 5's sampled worlds.
type Seat struct {
	def  *heuristic.Bot
	seed uint64
	net  *policynet.Model
	cfg  SeatConfig
	// record, when set (SetRecorder), receives one visit-corpus record per
	// decision this seat searched to completion. nil records nothing.
	record func(policynet.VisitRecord)
	// spare is the recycled world storage carried from one searched
	// decision's redeal source to the next (RedealSource.reclaim): the
	// seat's decisions are answered one at a time, and every world of a
	// decision is spent once its Search returns.
	spare rules.Spare
	// pool is the recycled node-state storage carried the same way, for the
	// cached-worlds trees (sparePool): every state a tree stores is spent
	// once its Search returns, and the next decision's trees resume into
	// those arrays. Made on the first searched decision that can use it.
	pool *sparePool
	// pcrFull and pcrCheap count the searched decisions playout-cap
	// randomization made full and cheap (PCRCounts; 0 with it off).
	pcrFull, pcrCheap int
	// dec is the decided-game state (decided.go; SeatConfig.DecidedHigh).
	dec decidedState
	// note, when set (SetExplain), receives one decision note per decision
	// the seat answers (explain.go). nil builds nothing.
	note func(explain.Note)
}

// SetRecorder installs the visit-corpus recorder (M1b; a bench runner
// -az-corpus). The records carry the searched decision (redacted state,
// the candidates' options, visits, prior, Q, root value, the played
// candidate) and the diagnostic opponent-hand rows; the driver fills the
// game fields and the outcome. world is the world source's name. Recording
// only reads: the seat's answers are unchanged.
func (s *Seat) SetRecorder(fn func(policynet.VisitRecord)) {
	s.record = fn
}

var (
	_ seat.Seat             = (*Seat)(nil)
	_ heuristic.BoardSeat   = (*Seat)(nil)
	_ searchseat.SearchSeat = (*Seat)(nil)
)

// NewSeat builds one seat of one game. seed is the per-seat seed the bench
// derives; net nil is generation 0.
func NewSeat(seed uint64, net *policynet.Model, cfg SeatConfig) (*Seat, error) {
	s, err := newSeat(seed, net, cfg)
	if err == nil && cfg.Mulligan != botpolicy.MulliganCoin {
		// Applied to the seat's own bot after newSeat builds it, so the
		// default seat's construction (and the autopay bot) is untouched.
		s.def = s.def.WithMulligan(cfg.Mulligan)
	}
	return s, err
}

// newSeat is NewSeat before the SeatConfig.Mulligan rule.
func newSeat(seed uint64, net *policynet.Model, cfg SeatConfig) (*Seat, error) {
	if err := cfg.Search.Validate(net); err != nil {
		return nil, err
	}
	if err := cfg.validateSimsByKind(); err != nil {
		return nil, err
	}
	if err := cfg.validatePCR(); err != nil {
		return nil, err
	}
	if err := cfg.validateDecided(); err != nil {
		return nil, err
	}
	switch cfg.World {
	case WorldRedeal:
	case WorldRedealClosed:
		if cfg.Belief == nil {
			return nil, errors.New("azmcts: the closed redeal world needs a belief (SeatConfig.Belief)")
		}
	case "", WorldClairvoyant:
		if cfg.Source == nil {
			return nil, errors.New("azmcts: the clairvoyant world is not linked (bots/azmcts/clairvoyant)")
		}
	default:
		return nil, fmt.Errorf("azmcts: world source %q: want %s, %s or %s", cfg.World, WorldClairvoyant, WorldRedeal, WorldRedealClosed)
	}
	def := heuristic.NewBot(seed)
	if cfg.Search.AutoPayment {
		// The seat's own answers (candidate 0, every unsearched decision) come
		// from the auto-pay bot, as the walk's do (env.go): a manual bot's
		// answer before a cast is a mana tap, which the auto-payment
		// vocabulary never offers, so every cast decision would be skipped.
		def = def.EnableAutoPayMana()
	}
	return &Seat{def: def, seed: seed, net: net, cfg: cfg}, nil
}

// validateSimsByKind refuses a SimsByKind entry that is negative or keyed by
// a kind outside decision.Kinds. It walks decision.Kinds in order, never the
// map, so the error it names is deterministic.
func (c *SeatConfig) validateSimsByKind() error {
	known := 0
	for _, k := range decision.Kinds {
		n, ok := c.SimsByKind[k]
		if !ok {
			continue
		}
		known++
		if n < 0 {
			return fmt.Errorf("azmcts: SimsByKind[%s] %d must be >= 0 (0 uses Search.Sims)", k, n)
		}
	}
	if known != len(c.SimsByKind) {
		return errors.New("azmcts: SimsByKind has a key outside decision.Kinds")
	}
	return nil
}

// Decide is the plain Seat half: the wrapped bot.
func (s *Seat) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	return s.def.Decide(ctx, v, d)
}

// DecideBoard is the driver's fallback when the seat's observation feed has
// stopped: the wrapped bot, counted (FeedStopped) so it is never silent.
func (s *Seat) DecideBoard(ctx context.Context, b botpolicy.Board, d decision.Decision) (decision.Intent, error) {
	if Watch != nil {
		Watch(Diag{Stats: Stats{FeedStopped: 1}, Seed: s.seed, Seq: d.Seq})
	}
	in, err := s.def.DecideBoard(ctx, b, d)
	if s.note != nil && err == nil {
		s.noteBot(d.Player, explain.Heuristic, "", in)
	}
	return in, err
}

// DecideRootRefused is DecideBoard for a decision whose feed is live but
// whose honest root refused to build (reason is the builder's): the wrapped
// bot, counted as FeedStopped (its historical meaning: the driver routed
// around the search) and as RootRefused, with the reason in Diag.Refused.
func (s *Seat) DecideRootRefused(ctx context.Context, b botpolicy.Board, d decision.Decision, reason string) (decision.Intent, error) {
	if Watch != nil {
		Watch(Diag{Stats: Stats{FeedStopped: 1, RootRefused: 1}, Refused: reason, Seed: s.seed, Seq: d.Seq})
	}
	in, err := s.def.DecideBoard(ctx, b, d)
	if s.note != nil && err == nil {
		s.noteBot(d.Player, explain.FallbackRoot, reason, in)
	}
	return in, err
}

// DecideSearch answers one of the seat's own decisions: the bot's answer
// first (candidate 0 and every fallback), then Search over the configured
// worlds: the injected source's (SeatConfig.Source, the clairvoyant clone),
// or honest redeals built from the driver's feed (SeatConfig.World). A
// source that refuses is an error: the game fails loudly rather than playing
// an unsearched seat under the az name. A refused redeal is counted
// (Stats.RedealRefused) and plays the bot's answer -- it never falls back to
// the clairvoyant source.
func (s *Seat) DecideSearch(ctx context.Context, env searchseat.Env, d decision.Decision) (decision.Intent, error) {
	botIn, err := s.def.DecideBoard(ctx, env.Board, d)
	if err != nil {
		return decision.Intent{}, err
	}
	if s.cfg.PriorOnly && env.Engine != nil {
		return s.decidePrior(ctx, env, d, botIn)
	}
	if s.cfg.Search.Sims <= 0 || env.Engine == nil {
		if s.note != nil {
			s.noteBot(d.Player, explain.Heuristic, "", botIn)
		}
		return botIn, nil
	}
	var t0 float64
	if Millis != nil {
		t0 = Millis()
	}
	obs := searchprobe.NewCollector(d.Player)
	// The budget, seed, root noise and sampling; cheap is a playout-cap
	// cheap search (never with SeatConfig.FullProb 0).
	opts, cheap := s.rootOptions(&d, env.Engine.G.Turn)
	// A full search once the game is decided is a decided one (decided.go).
	decided := !cheap && s.decided()
	if decided {
		opts = s.decidedOptions(opts)
	}
	var src WorldSource
	var redeal *RedealSource
	if s.redeals() {
		rs, err := s.redealSource(env, obs, opts.Seed, d.Player)
		if err != nil {
			return decision.Intent{}, err
		}
		rs.spare, s.spare = s.spare, rules.Spare{}
		if s.pool == nil && !noSparePool && opts.CachedWorlds > 0 && opts.NodeCache > 0 && !opts.RootPerWorld {
			s.pool = newSparePool(min(opts.NodeCache, opts.Sims))
		}
		rs.pool = s.pool
		src, redeal = rs, rs
	} else {
		cs, err := s.cfg.Source(env, obs)
		if err != nil {
			return decision.Intent{}, err
		}
		src = cs
	}
	root := Root{Engine: env.Engine, Decision: &d, Bot: botIn, Observer: obs}
	// The tap backstop: under AutoPayment a bot answer that taps mana by hand
	// is outside the payment vocabulary, and the ordinary root would skip the
	// decision (SkipFewCandidates) and play the tap unsearched -- the
	// counterspell windows az never learned. Search the full vocabulary with
	// no bot candidate instead (Root.NoBot); every failure path still plays
	// the bot's tap. Stats.BotTapRerooted counts it.
	tapReroot := opts.AutoPayment && d.Kind == decision.KPriority && priorityBase(&d, botIn) == BaseActivate
	if tapReroot {
		root.NoBot = true
	}
	res, err := Search(ctx, root, src, s.net, opts)
	if tapReroot && res.Stats.Searched == 1 {
		res.Stats.BotTapRerooted = 1
	}
	if redeal != nil {
		s.spare = redeal.reclaim()
	}
	if err != nil {
		return decision.Intent{}, err
	}
	refused, dealFailed := "", ""
	if redeal != nil && res.Stats.Searched == 1 {
		if refused = redeal.Refused(); refused != "" {
			res.Stats.RedealRefused = 1
		}
		dealFailed = redeal.DealFailed()
	}
	searched := res.Stats.Searched == 1
	if searched && s.cfg.FullProb > 0 {
		if cheap {
			s.pcrCheap++
		} else {
			s.pcrFull++
		}
	}
	if searched && decided {
		s.dec.searched++
		if s.dec.side > 0 {
			s.dec.winning++
		}
	}
	complete := searched && res.Stats.AllFailed == 0 && refused == ""
	if s.cfg.DecidedHigh > 0 && complete && res.Stats.DeadlineHits == 0 {
		s.dec.observe(res.RootValue, s.cfg.DecidedHigh)
	}
	// A cheap search only moves the game on: it is never recorded. A decided
	// one is recorded at DecidedWeight (not at all at 0).
	if s.record != nil && !cheap && complete && (!decided || s.cfg.DecidedWeight > 0) {
		r := visitRecord(env.Engine, &d, res, s.cfg.RecordFeatures, s.worldName(), opts.Sims)
		if decided && s.cfg.DecidedWeight < 1 {
			r.Weight = s.cfg.DecidedWeight
		}
		s.record(r)
	}
	if res.Kind != "" && Watch != nil {
		dg := Diag{
			Turn: env.Engine.G.Turn, Kind: res.Kind, Searched: searched,
			Candidates: len(res.Candidates), Choice: res.Choice, Stats: res.Stats, Refused: refused, DealFailed: dealFailed,
			Seed: s.seed, Seq: d.Seq, Cheap: cheap && searched, Decided: decided && searched,
		}
		if Millis != nil {
			dg.MS = Millis() - t0
		}
		Watch(dg)
	}
	if s.note != nil {
		sn := searchNote{d: &d, bot: botIn, res: res, root: env.Engine, refused: refused, cheap: cheap && searched,
			noBot: tapReroot, rankSkip: rankSkips(opts.priorTopK(s.net), opts.PriorRankMaxOptions, &d),
			searchedOK: searched && res.Stats.AllFailed == 0 && res.Stats.DeadlineHits == 0}
		if Millis != nil {
			sn.ms = Millis() - t0
		}
		s.noteSearch(sn)
	}
	return res.Intent, nil
}

// noSparePool keeps a seat from pooling its trees' storage (sparePool): the
// test that pins pooled searches to unpooled ones (TestSparePoolIsExact)
// runs both.
var noSparePool bool

// WantsPaymentActions reports whether the driver must build the decision's
// payment plans for this seat (seat.PaymentPlanConsumer): exactly when the
// search runs over the auto-payment vocabulary (SeatConfig.Search.AutoPayment).
// Without plans the wrapped auto-pay bot answers with a manual mana tap, which
// is outside that vocabulary, so the decision would go unsearched. A seat
// without AutoPayment searches the manual options and opts out.
func (s *Seat) WantsPaymentActions() bool { return s.cfg.Search.AutoPayment }

// DigestFree reports that the seat never samples from its feed's history
// (searchseat.DigestFree): both world sources -- the injected clairvoyant
// clone and the honest redeal -- read the frames and the known-card
// projection, never searchprobe.Sample, so a driver may skip the history
// digest chain on every real decision.
func (s *Seat) DigestFree() bool { return true }

// decidePrior is the PriorOnly seat's decision: the candidates Search would
// build, answered by the argmax of the network prior (ties on the lowest
// index). A decision Search would not search is the bot's.
func (s *Seat) decidePrior(ctx context.Context, env searchseat.Env, d decision.Decision, botIn decision.Intent) (decision.Intent, error) {
	obs := searchprobe.NewCollector(d.Player)
	opts := s.cfg.Search
	opts.Sims, opts.Noise, opts.Sample = 0, false, false
	opts.Seed = DecisionSeed(s.seed, d.Seq)
	res, err := Search(ctx, Root{Engine: env.Engine, Decision: &d, Bot: botIn, Observer: obs}, nil, s.net, opts)
	if err != nil {
		return decision.Intent{}, err
	}
	if len(res.Candidates) < 2 {
		if s.note != nil {
			s.noteBot(d.Player, explain.Forced, "", botIn)
		}
		return botIn, nil
	}
	best := 0
	for i, p := range res.Prior {
		if p > res.Prior[best] {
			best = i
		}
	}
	if Watch != nil {
		Watch(Diag{Turn: env.Engine.G.Turn, Kind: res.Kind, Candidates: len(res.Candidates), Choice: best, Stats: res.Stats})
	}
	if s.note != nil {
		s.notePrior(d.Player, botIn, res, best)
	}
	if best == 0 {
		return botIn, nil
	}
	return res.Candidates[best], nil
}

// redealSource builds the honest source at one of the seat's decisions from
// the driver's feed: the seat's History, its known-card projection (folded
// incrementally across the game) and the feed's collector. A feed that has
// no frames, or a known-card fold that failed, is a refusal -- counted, and
// the bot's answer is played -- never a clairvoyant fallback.
func (s *Seat) redealSource(env searchseat.Env, obs *searchprobe.Collector, seed uint64, actor state.PlayerID) (*RedealSource, error) {
	in := RedealInput{Setup: searchseat.SeatSetup(env.Setup, s, actor)}
	if env.Feed != nil && env.Feed.Live() && env.Feed.Frames() > 0 {
		in.History = env.Feed.History()
		in.Base = searchprobe.RedealBase{Engine: env.Engine, Observer: env.Feed.Collector()}
		known, err := env.Feed.Known()
		if err != nil {
			// A dead projection: the redealer refuses without a base.
			in.Base = searchprobe.RedealBase{}
		}
		in.Known = known
	}
	return NewRedeal(in, obs, seed, s.cfg.Worlds)
}

// redeals reports whether the seat searches honest redeal worlds (open or
// closed lists).
func (s *Seat) redeals() bool {
	return s.cfg.World == WorldRedeal || s.cfg.World == WorldRedealClosed
}

// SearchBelief is the belief a closed-list seat draws its opponents' lists
// from (searchseat.ClosedListSeat), nil for every other world: a driver
// closes the seat's public game (and so its honest root) with it.
func (s *Seat) SearchBelief() *searchprobe.Belief {
	if s.cfg.World == WorldRedealClosed {
		return s.cfg.Belief
	}
	return nil
}

// worldName is the record's world source name.
func (s *Seat) worldName() string {
	if s.cfg.World == "" {
		return WorldClairvoyant
	}
	return s.cfg.World
}
