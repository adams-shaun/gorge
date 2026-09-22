package searchseat

import (
	"context"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/searchprobe"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/seat"
	"github.com/adams-shaun/gorge/view"
)

// SearchBot is the playable seat: the default bot wrapped with the teacher
// (Choose) answering the covered kinds, everything else delegated unchanged.
// It is the playing half of the same pair cmd/searchteacher is the measuring
// half of: candidate building, world sampling, teacher scoring and root
// matching are the ONE extracted function (Choose), so a seat that plays the
// teacher and the corpus a student learns from cannot drift.
//
// The contract is seat.PolicyNetBot's, stated the same way:
//
//   - it wraps the default bot (seat.NewBot, the same PCG seed derivation, so
//     the delegation path consumes exactly the rng the default bot would);
//   - it answers only the kinds Options.Kinds gates (the implemented set:
//     "attackers", and "cast" for a priority decision offering two or more
//     distinct castable objects) and only when the sampler and teacher both
//     succeed -- Choose returns the bot's own intent on EVERY failure path
//     (ineligible kind, <2 candidates, sampler failure, no accepted worlds,
//     teacher error, unmatchable root), so a search teacher that cannot
//     search plays legally as the bot;
//   - deterministic: no wall clock, no map iteration reaching an answer
//     (Options.Kinds is probed by key only), ties on option index; Choose is
//     a pure function of its inputs, so a decision answered with the timing
//     hooks installed is the decision answered without them;
//   - the seat never reads hidden state beyond what the actor-scoped
//     Collector already exposes: it reads the Env the driver hands it
//     (board, engine handle for Choose's seed/turn derivation, and the
//     actor's own observation history) and nothing else.
//
// It is NOT a plain Seat in spirit even though it satisfies the interface:
// its search path is only reachable through the driver's feed (DecideSearch),
// exactly the idiom seat.BoardSeat uses -- a seat variant the driver detects
// and feeds richer inputs than the plain interface carries.
type SearchBot struct {
	def  *seat.Bot
	opts Options
	// sampleMS/searchMS are the per-decision timing scratch the Millis hook
	// fills via the AfterSample/AfterSearch callbacks. A seat is driven by
	// one goroutine (the game's driver; Choose's internal parallelism folds
	// before returning), so plain fields need no lock. They are diagnostics
	// only and never reach an answer.
	sampleMS, searchMS float64
}

// SearchSeat is the driver-side contract internal/bench.PlayGame type-asserts
// (the seat.BoardSeat branch, mirrored): a seat that answers from the driver's
// feed at its own decisions. DecideBoard is the plain-bot path the driver
// falls back to when the seat's feed has stopped observing (a capture error
// must never wedge a game that can still play legally).
type SearchSeat interface {
	seat.Seat
	DecideSearch(ctx context.Context, env Env, d decision.Decision) (decision.Intent, error)
	DecideBoard(ctx context.Context, b botpolicy.Board, d decision.Decision) (decision.Intent, error)
}

// Env is what the driving caller hands the seat at one of its own decisions.
// The engine handle is required by Choose itself (seed/turn derivation); the
// seat reads no hidden state through it beyond what the actor-scoped
// Collector's captures already expose to the actor.
type Env struct {
	// Setup is the declared public game (deck lists, token definitions,
	// starting life) the sampler rebuilds worlds from.
	Setup searchprobe.PublicGame
	// Engine is the live engine the decision was asked of.
	Engine *rules.Engine
	// Board is the deciding seat's botpolicy.Board, built by the driver the
	// same way it builds it for a BoardSeat.
	Board botpolicy.Board
	// Feed is the seat's own observation stream, captured through this very
	// decision: History includes Frame, and RecordAnswer (the driver calls
	// it after the seat answers) writes the played intent back.
	Feed *Feed
}

// Diag is one Choose outcome as a cost-report consumer sees it, fired through
// Watch. It mirrors the DecisionRecord cmd/searchteacher writes -- a record
// exists only when the decision was ASKED (Eligible and at least two
// candidates built), which is exactly the population a per-decision cost and
// coverage report aggregates.
type Diag struct {
	Turn               int32
	SampleMS, SearchMS float64
	Trace              Trace
}

// Millis is the wall-clock source the DRIVING COMMAND installs before any
// game starts (cmd/* may import time; this package may not -- internal/
// archtest). Nil (the zero, and the state every engine-replay context runs
// in) means untimed. It must be a monotonic elapsed-milliseconds read; it
// never reaches an answer, only the Diag.
var Millis func() float64

// Watch is the per-decision diagnostic consumer the driving command installs
// before any game starts (write-once, read-only during play; games run on
// several goroutines, so the consumer must synchronise itself). Nil = silent.
// It is called for every ASKED decision -- at most once per game decision per
// search seat -- never from a replay.
var Watch func(Diag)

// compile-time assertions: SearchBot is a Seat and a BoardSeat (the wrapped
// bot is a BoardSeat and the driver's plain-bot fallback needs it) and
// satisfies the driver's search contract.
var (
	_ seat.Seat      = (*SearchBot)(nil)
	_ seat.BoardSeat = (*SearchBot)(nil)
	_ SearchSeat     = (*SearchBot)(nil)
)

// NewSearchBot wraps the default bot (same PCG seed derivation as
// seat.NewBot, so delegation consumes exactly the default bot's rng stream)
// with the teacher answering Options.Kinds. A zero Options.Kinds is completed
// from Defaults(), so a caller that passes nothing gets the measured teacher,
// not a silent never-search bot.
func NewSearchBot(seed uint64, opts Options) *SearchBot {
	if opts.Kinds == nil {
		opts = Defaults()
	}
	return &SearchBot{def: seat.NewBot(seed), opts: opts}
}

// Decide is the plain Seat half: the wrapped bot. It exists for callers that
// hold the seat only as a Seat; the driver's search path goes through
// DecideSearch/DecideBoard.
func (s *SearchBot) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	return s.def.Decide(ctx, v, d)
}

// DecideBoard is the game-shaped half: the wrapped bot, and the driver's
// plain-bot fallback when the seat's feed has stopped observing.
func (s *SearchBot) DecideBoard(ctx context.Context, brd botpolicy.Board, d decision.Decision) (decision.Intent, error) {
	return s.def.DecideBoard(ctx, brd, d)
}

// DecideSearch answers one of the seat's own decisions: the bot's answer
// first (it is candidate index 0 and the fallback on every failure path),
// then the teacher when the decision is eligible. Diagnostics fire exactly
// where the teacher's own DecisionRecord would: only for ASKED decisions
// (eligible, at least two candidates).
func (s *SearchBot) DecideSearch(ctx context.Context, env Env, d decision.Decision) (decision.Intent, error) {
	botIn, err := s.def.DecideBoard(ctx, env.Board, d)
	if err != nil {
		return decision.Intent{}, err
	}
	// A feed that stopped observing has no valid history to sample from; the
	// driver normally routes around this (its own DecideBoard fallback), so
	// this guard is defence in depth, never the expected path.
	if !env.Feed.Live() || env.Feed.Frames() == 0 {
		return botIn, nil
	}
	if !Eligible(&d, s.opts) {
		return botIn, nil
	}
	opts := s.opts
	if Millis != nil {
		t0 := Millis()
		opts.AfterSample = func() { s.sampleMS = Millis() - t0 }
		opts.AfterSearch = func() { s.searchMS = Millis() - t0 - s.sampleMS }
	}
	in, _, tr := Choose(env.Setup, env.Feed.History(), env.Feed.Collector(), env.Engine, &d, botIn, env.Feed.LastFrame(), opts)
	if len(tr.Candidates) >= 2 && Watch != nil {
		Watch(Diag{Turn: env.Engine.G.Turn, SampleMS: s.sampleMS, SearchMS: s.searchMS, Trace: tr})
	}
	// Reset the per-decision scratch so a stale time never reaches the next
	// diagnostic when the timing hooks are absent.
	s.sampleMS, s.searchMS = 0, 0
	return in, nil
}
