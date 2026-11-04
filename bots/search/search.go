// Package search hosts the search-teacher policy
// (docs/design/2026-09-28-hosted-bot-packages.md): the adapter over
// searchseat.SearchBot that answers the covered kinds from the
// host's honest root and the seat's own observation feed, and plays the
// wrapped default bot everywhere else.
//
// The routing is exactly design §5.1's adapter rules for `search`: a decision
// the seat wants an Env for (searchseat.Eligible) is answered from
// env.Search when the honest root built AND the feed is still live, and from
// env.Board — the wrapped bot, the bench's own fallback — otherwise. The
// wrapped bot (DecideBoard) is also what a non-Env decision costs, so a
// search table plays no differently from a bot table except at the decisions
// the teacher actually searches.
package search

import (
	"context"

	"github.com/adams-shaun/gorge/bots/heuristic"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/explain"
	"github.com/adams-shaun/gorge/bots/searchseat"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
)

// Policy is the registry key.
const Policy = "search"

func init() {
	bots.Register(bots.Entry{Info: bots.Info{
		Name:        Policy,
		Label:       "Search (teacher)",
		Description: "The search teacher: it replans the covered decisions over sampled worlds rebuilt from its own observations, and plays the production bot everywhere else.",
		Tier:        bots.Experimental,
		Cost: bots.Cost{
			MeanMS: 664, P95MS: 2200, Scope: "per searched decision",
			Note:   "~85 searched decisions per game per seat at parallelism 1; 171 ms mean with decision-level parallelism",
			Source: "archived 2026-09-24-training-approaches-summary.md §4 (docs/design/ARCHIVE.md)",
		},
		Env: true, Search: true,
		Formats:   []string{"constructed"},
		MaxSeats:  2,
		Caretaker: "bot",
	}, New: newSearch})
}

// Hosted is the hosted config (design §4.3): exactly the measured teacher's
// knobs — searchseat.Defaults(), the config its +3.3pp measurement was
// taken with — with Parallelism
// left 0 (sequential). The caller that builds a seat sets Parallelism; it
// changes latency only, never an answer (searchseat.Options.Parallelism).
func Hosted() searchseat.Options { return searchseat.Defaults() }

// newSearch is the registry factory: Hosted() plus Parallelism from Options.
func newSearch(o bots.Options) (seat.Seat, error) {
	cfg := Hosted()
	cfg.Parallelism = o.SearchParallelism
	return New(o, cfg)
}

// New builds the adapter over an explicit config overlay, for a bench that
// overlays its own flags on Hosted() (design §4.3); the host goes through the
// registry factory above.
func New(o bots.Options, cfg searchseat.Options) (seat.Seat, error) {
	return &hostedSeat{bot: searchseat.NewSearchBot(o.Seed, cfg), opts: cfg}, nil
}

// hostedSeat is the adapter. bot is held behind the searchseat.SearchSeat
// interface (which *searchseat.SearchBot satisfies) so tests can observe the
// routing with a recording delegate instead of running a real search.
type hostedSeat struct {
	bot  searchseat.SearchSeat
	opts searchseat.Options
}

var (
	_ bots.EnvSeat        = (*hostedSeat)(nil)
	_ explain.Emitter     = (*hostedSeat)(nil)
	_ heuristic.BoardSeat = (*hostedSeat)(nil)
	// A bench driver's SearchSeat contract: the driver builds an
	// observation feed for a SearchSeat and hands the decision's Env
	// to it — the live engine with -hosted-root off (the bench keeps the real
	// engine), the honest root with it on. The host never calls it; its Env
	// dispatch routes the same wrapped call through DecideEnv, gated on
	// WantsEnv.
	_ searchseat.SearchSeat = (*hostedSeat)(nil)
)

// Decide is the plain Seat half: the wrapped bot. Decisions where WantsEnv is
// false never reach it from the host (the BoardSeat branch serves them), but
// a caller holding the seat only as a Seat goes here.
func (s *hostedSeat) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	return s.bot.Decide(ctx, v, d)
}

// DecideBoard is the game-shaped half: the wrapped bot. It is the fallback
// the Env branch plays (root refused, feed stopped) and the whole answer for
// every non-Env decision.
func (s *hostedSeat) DecideBoard(ctx context.Context, b botpolicy.Board, d decision.Decision) (decision.Intent, error) {
	return s.bot.DecideBoard(ctx, b, d)
}

// WantsPaymentActions forwards the wrapped seat's payment-plan answer; a
// delegate without one is a consumer (bots default to plans).
func (s *hostedSeat) WantsPaymentActions() bool {
	if c, ok := s.bot.(seat.PaymentPlanConsumer); ok {
		return c.WantsPaymentActions()
	}
	return true
}

// DecideSearch keeps the hosted adapter usable by a bench's engine-owning driver.
func (s *hostedSeat) DecideSearch(ctx context.Context, env searchseat.Env, d decision.Decision) (decision.Intent, error) {
	return s.bot.DecideSearch(ctx, env, d)
}

// WantsEnv reports whether this decision is one the teacher would attempt:
// searchseat.Eligible, deliberately the same test Choose applies, so the host
// never pays a redeal for a decision the teacher would delegate anyway. It is
// a pure function of d and the seat's own config.
func (s *hostedSeat) WantsEnv(d *decision.Decision) bool {
	return searchseat.Eligible(d, s.opts)
}

// DecideEnv answers one of the seat's own decisions. Design §5.1: forward
// env.Search to the wrapped DecideSearch when the honest root built AND the
// actor's feed is live; otherwise play the wrapped bot on env.Board — exactly
// the bench's fallback. A nil feed is the host's signal for a stopped feed or
// a table that built none (server/host/botenv.go envData), the same state
// Feed.Live() == false reports, and DecideSearch itself re-checks both.
func (s *hostedSeat) DecideEnv(ctx context.Context, env bots.Env, d decision.Decision) (decision.Intent, error) {
	if env.Search.Engine != nil && env.Search.Feed != nil && env.Search.Feed.Live() {
		return s.bot.DecideSearch(ctx, env.Search, d)
	}
	return s.bot.DecideBoard(ctx, env.Board, d)
}

// SetExplain forwards the decision-note sink (explain.Emitter) to the
// teacher seat, which emits one note per decision DecideSearch answers.
func (s *hostedSeat) SetExplain(fn func(explain.Note)) {
	if em, ok := s.bot.(explain.Emitter); ok {
		em.SetExplain(fn)
	}
}
