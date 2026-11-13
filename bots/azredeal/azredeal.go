// Package azredeal hosts the az-redeal policy (spec
// 2026-09-28-hosted-bot-packages §3.1): the adapter over internal/azmcts.Seat
// in its honest redeal world (generation 0: uniform prior, heuristic leaf,
// 100 simulations, a fresh deal per simulation) that answers the searched
// kinds from the host's honest root and the seat's own observation feed, and
// plays the wrapped default bot everywhere else.
//
// The routing is exactly spec §5.1's adapter rule for `az-redeal`: a decision
// of a searched kind is answered from env.Search when the honest root built
// AND the feed is still live, and from env.Board — the wrapped bot, the
// bench's own fallback — otherwise. The wrapped bot (DecideBoard) is also
// what a non-searched decision costs, so an az-redeal table plays no
// differently from a bot table except at priority, attackers, blockers and
// target decisions.
//
// The adapter never sets SeatConfig.Source: the clairvoyant world source is
// a bench-only injection (the private companion module's `clairvoyant` package), and a hosted entry
// with it would clone the real engine.
package azredeal

import (
	"context"

	"github.com/adams-shaun/gorge/bots/heuristic"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/azmcts"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/explain"
	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/bots/searchseat"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
)

// Policy is the registry key.
const Policy = "az-redeal"

func init() {
	bots.Register(bots.Entry{Info: bots.Info{
		Name:        Policy,
		Label:       "az-redeal (AlphaZero search)",
		Description: "The az search seat in its honest world: at every searched decision it runs 100 MCTS simulations over freshly redealt worlds rebuilt from its own observations, and plays the production bot everywhere else. Plays constructed 1v1 and Commander pods of up to four seats.",
		Tier:        bots.Experimental,
		Cost: bots.Cost{
			MeanMS: 247, P95MS: 465, Scope: "per searched decision",
			Note:   "at 100 simulations, uncontended, in 2-seat constructed; 4-seat cost unmeasured; contention raises it, and the host never cuts the search",
			Source: "docs/design/2026-09-28-spellbench-agent-design.md §12.5",
		},
		Env: true, Search: true,
		Formats:   []string{"constructed", "commander"},
		MaxSeats:  4,
		Caretaker: "bot",
	}, New: newAZRedeal})
}

// Hosted is the hosted config (spec §4.3, §11): exactly azmcts's eval seat —
// DefaultSeatConfig(), the 100-sim / Limit 6 / MaxSteps 1000 knobs the +20.5pp
// measurement was taken with — with the honest redeal world and zero K
// (a fresh deal per simulation), the generation-0 shape (no network, no
// exploration noise). World is set explicitly (never the clairvoyant
// default), and Source is left nil: NewSeat refuses a clairvoyant world with
// a nil source, so a hosted seat can never clone the real engine.
func Hosted() azmcts.SeatConfig {
	cfg := azmcts.DefaultSeatConfig()
	cfg.World = azmcts.WorldRedeal
	cfg.Worlds = 0
	return cfg
}

// newAZRedeal is the registry factory: Hosted() with the per-seat seed from
// Options and a nil net (generation 0).
func newAZRedeal(o bots.Options) (seat.Seat, error) {
	return New(o, Hosted())
}

// New builds the adapter over an explicit config overlay. It is botbench's
// entry point (spec §4.3): botbench overlays its -az-* flags on Hosted() and
// calls this; the host goes through the registry factory above. net is
// nil here — the az-redeal entry is generation 0 (heuristic leaf, uniform
// prior); a checkpoint is a different bot, built with NewWithModel.
func New(o bots.Options, cfg azmcts.SeatConfig) (seat.Seat, error) {
	return NewWithModel(o, cfg, nil)
}

// NewWithModel is New with the search's network: its prior ranks the
// candidates and its value head scores the leaves, as cfg says. The model is
// shared and read-only (a gonet network is one per path across every game in
// the process), so one model serves every seat the factory builds.
func NewWithModel(o bots.Options, cfg azmcts.SeatConfig, model *policynet.Model) (seat.Seat, error) {
	s, err := azmcts.NewSeat(o.Seed, model, cfg)
	if err != nil {
		return nil, err
	}
	return &hostedSeat{bot: s, cfg: cfg, budgetMS: o.DecisionDeadlineMS}, nil
}

// ClosedFactory builds Hosted's search with the opponents' deck lists closed
// under belief: each opponent's unknown cards are dealt from lists drawn from
// the belief, never from its real list or real hidden cards. It searches sims
// simulations per decision, over the payment plans when Options.AutoPayMana
// says the decisions carry them.
func ClosedFactory(belief *searchprobe.Belief, sims int) bots.Factory {
	return func(o bots.Options) (seat.Seat, error) {
		cfg := Hosted()
		cfg.World, cfg.Belief = azmcts.WorldRedealClosed, belief
		cfg.Search.Sims = sims
		// A human meets modes, may-triggers, small choices and multi-target
		// picks, so the paired bot weighs them; they stay off for hosted seats.
		cfg.Search.Kinds = azmcts.FullKinds()
		// The mulligan too, both asks, on its own budget; the seat's own
		// answer there, and every fallback, reads the hand (never the
		// default bot's coin).
		cfg.Search.Kinds.Mulligan = true
		// The commander-zone choice too: a commander's owner chooses whether
		// it returns to the command zone (CR 903.9), and the paired bot weighs
		// that choice instead of answering it by rule.
		cfg.Search.Kinds.CommanderZone = true
		cfg.Search.MulliganWorlds, cfg.Search.MulliganTurns = MulliganWorlds, MulliganTurns
		cfg.Mulligan = botpolicy.MulliganConstructed
		cfg.Search.AutoPayment = o.AutoPayMana
		cfg.Search.Line = o.ExpectedLine
		return New(o, cfg)
	}
}

// The paired bot's mulligan budget (azmcts.Options.MulliganWorlds,
// MulliganTurns), measured on rigged and dealt sevens of three repo decks.
const (
	// MulliganWorlds is the deals every candidate is played in. At
	// thirty-two the search kept every rigged three-land seven of cheap
	// spells and kept no rigged seven of 0, 6 or 7 lands, and a keep or
	// mulligan ask stays under a second of CPU, a three-candidate bottoming
	// ask well inside the two-second advice deadline.
	MulliganWorlds = 32
	// MulliganTurns is the game turns a walk plays, five a seat. At six the
	// leaf, which reads a land in play as material and a card in hand as two
	// points, still preferred keeping a six-land seven; at ten it kept none
	// of 0, 6 or 7 lands; sixteen doubled the CPU without judging those
	// hands better.
	MulliganTurns = 10
)

// DecisionBudgetMS is the factory's per-decision wall-clock budget
// (bots.Options.DecisionDeadlineMS), for the host to arm as the DecideEnv
// context's deadline. Only DecideEnv reaches the search here -- Decide and
// DecideBoard forward to the wrapped bot's plain halves, which never build a
// tree -- so the budget is declared on this adapter and armed by the host
// (this package may not import time, internal/archtest).
func (s *hostedSeat) DecisionBudgetMS() int { return s.budgetMS }

// hostedSeat is the adapter. bot is held behind the searchseat.SearchSeat
// interface (which *azmcts.Seat satisfies) so the routing tests can observe
// the forwarding with a recording delegate instead of running real searches.
// budgetMS is the factory's per-decision wall-clock budget
// (bots.Options.DecisionDeadlineMS): the host arms it as its DecideEnv
// context's deadline, and a search that outlives it is aborted by azmcts
// Search's ctx bail-out, which plays the wrapped bot's answer -- this
// policy's non-searched fallback. Zero -- bench and training -- is
// unbounded.
// DecideRootRefused forwards the driver's live-feed refused-root fallback
// (bench's search branch) to the wrapped seat, or plays DecideBoard.
func (s *hostedSeat) DecideRootRefused(ctx context.Context, b botpolicy.Board, d decision.Decision, reason string) (decision.Intent, error) {
	if rr, ok := s.bot.(rootRefuser); ok {
		return rr.DecideRootRefused(ctx, b, d, reason)
	}
	return s.bot.DecideBoard(ctx, b, d)
}

// rootRefuser is the wrapped seat's root-refused fallback (*azmcts.Seat's
// DecideRootRefused); a delegate without it plays DecideBoard.
type rootRefuser interface {
	DecideRootRefused(ctx context.Context, b botpolicy.Board, d decision.Decision, reason string) (decision.Intent, error)
}

type hostedSeat struct {
	bot      searchseat.SearchSeat
	cfg      azmcts.SeatConfig
	budgetMS int
}

var (
	_ bots.EnvSeat              = (*hostedSeat)(nil)
	_ bots.BudgetedSeat         = (*hostedSeat)(nil)
	_ heuristic.BoardSeat       = (*hostedSeat)(nil)
	_ seat.PaymentPlanConsumer  = (*hostedSeat)(nil)
	_ searchseat.SearchSeat     = (*hostedSeat)(nil)
	_ searchseat.ClosedListSeat = (*hostedSeat)(nil)
	_ explain.Emitter           = (*hostedSeat)(nil)
)

// Decide is the plain Seat half: the wrapped bot. Decisions where WantsEnv is
// false never reach it from the host (the BoardSeat branch serves them), but
// a caller holding the seat only as a Seat goes here.
func (s *hostedSeat) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	return s.bot.Decide(ctx, v, d)
}

// UnwrapSeat lets botbench attach its visit recorder to the underlying az seat.
func (s *hostedSeat) UnwrapSeat() seat.Seat { return s.bot }

// SetRecorder preserves botbench's per-seat az corpus hook through the hosted adapter.
func (s *hostedSeat) SetRecorder(fn func(policynet.VisitRecord)) {
	if recorder, ok := s.bot.(interface {
		SetRecorder(func(policynet.VisitRecord))
	}); ok {
		recorder.SetRecorder(fn)
	}
}

// SetExplain forwards the decision-note sink (explain.Emitter) to the
// wrapped az seat, which emits one note per decision it answers.
func (s *hostedSeat) SetExplain(fn func(explain.Note)) {
	if em, ok := s.bot.(explain.Emitter); ok {
		em.SetExplain(fn)
	}
}

// SearchBelief forwards the wrapped seat's closed-list belief
// (searchseat.ClosedListSeat): the host and the bench close this seat's
// public game, and so the honest root they build for it, with it. Nil for
// the open-list config.
func (s *hostedSeat) SearchBelief() *searchprobe.Belief {
	if c, ok := s.bot.(searchseat.ClosedListSeat); ok {
		return c.SearchBelief()
	}
	return nil
}

// WantsPaymentActions forwards the wrapped seat's payment-plan opt-in
// (seat.PaymentPlanConsumer). A delegate without one is a consumer: bots
// default to receiving plans (seat.PaymentPlanConsumer's contract).
func (s *hostedSeat) WantsPaymentActions() bool {
	if c, ok := s.bot.(seat.PaymentPlanConsumer); ok {
		return c.WantsPaymentActions()
	}
	return true
}

// DecideSearch keeps this adapter usable by botbench's engine-owning driver.
func (s *hostedSeat) DecideSearch(ctx context.Context, env searchseat.Env, d decision.Decision) (decision.Intent, error) {
	return s.bot.DecideSearch(ctx, env, d)
}

// DecideBoard is the game-shaped half: the wrapped bot, the whole answer for
// every decision WantsEnv declines (the host and mtgsim ask it there). It is
// played through the wrapped seat's uncounted half when it has one: the Env
// branch's fallbacks (root refused, feed stopped) call the wrapped
// DecideBoard themselves and are counted there, but a kind this seat never
// searches was routed around nothing.
func (s *hostedSeat) DecideBoard(ctx context.Context, b botpolicy.Board, d decision.Decision) (decision.Intent, error) {
	if u, ok := s.bot.(unsearchedDecider); ok {
		return u.DecideUnsearched(ctx, b, d)
	}
	return s.bot.DecideBoard(ctx, b, d)
}

// unsearchedDecider is the wrapped seat's uncounted bot half (*azmcts.Seat's
// DecideUnsearched); a delegate without it plays DecideBoard.
type unsearchedDecider interface {
	DecideUnsearched(ctx context.Context, b botpolicy.Board, d decision.Decision) (decision.Intent, error)
}

// WantsEnv reports whether this decision is one of the az seat's searched
// kinds (priority, attackers, blockers, target, spec §5.1) under the seat's
// own kind set: the host gates its redeal on it, so a config that narrows
// Kinds must narrow the host's redeal cost with it. It is a pure function of
// d and the seat's own config.
func (s *hostedSeat) WantsEnv(d *decision.Decision) bool {
	k := s.cfg.Search.Kinds
	switch d.Kind {
	case decision.KPriority:
		return k.Priority
	case decision.KAttackers:
		return k.Attackers
	case decision.KBlockers:
		return k.Blockers
	case decision.KTarget:
		return k.Target || k.Searches(d)
	default:
		// The fuller kinds (modes, optional triggers, small choices) when the
		// config enables them; the four-kind default config enables none, so
		// az-redeal's answer is unchanged.
		return k.Searches(d)
	}
}

// DecideEnv answers one of the seat's own decisions. Spec §5.1: forward
// env.Search to the wrapped DecideSearch when the honest root built AND the
// actor's feed is live; otherwise play the wrapped bot on env.Board — exactly
// the bench's fallback. The gate is not optional decoration: the az seat's
// redeal source is built from the feed's history, so a nil or stopped feed
// reaches it as an error and the az seat fails loudly instead of playing its
// fallback. A nil feed is the host's signal for a stopped feed or a table
// that built none (server/host/botenv.go envData), the same state Feed.Live() ==
// false reports.
func (s *hostedSeat) DecideEnv(ctx context.Context, env bots.Env, d decision.Decision) (decision.Intent, error) {
	if env.Search.Engine != nil && env.Search.Feed != nil && env.Search.Feed.Live() {
		return s.bot.DecideSearch(ctx, env.Search, d)
	}
	if env.Search.Feed != nil && env.Search.Feed.Live() {
		// A live feed whose honest root refused: counted apart from a true
		// feed stop (azmcts.Stats.RootRefused), with the builder's reason.
		if rr, ok := s.bot.(rootRefuser); ok {
			return rr.DecideRootRefused(ctx, env.Board, d, env.RootRefused)
		}
	}
	return s.bot.DecideBoard(ctx, env.Board, d)
}
