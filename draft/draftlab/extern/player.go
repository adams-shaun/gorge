package extern

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"sync/atomic"

	"github.com/adams-shaun/gorge/bots/heuristic"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/draft/draftlab"
	"github.com/adams-shaun/gorge/draft/draftlab/extern/wire"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
)

// decideSeedSalt folds the seat's seed away from the pick/build seeds and
// from any other seat's seed: one PCG stream per (seatSeed, decision seq).
const decideSeedSalt = 0xE5D1 << 48

// unscopedCounter numbers the "u<u>.<player>" streams of seats used
// outside a draftlab game (never begun). Scheduling-dependent by design:
// a stream id never reaches a result.
var unscopedCounter atomic.Uint64

// Player wraps the agent as a draftlab.NamedPlayer: New builds a fresh
// seat per game that answers the engine's decisions over the wire, falling
// back to the deterministic botpolicy answer on every refusal (and, under
// FailFallback, on every session failure). The agent's hello must serve
// the "decide" kind; ErrRole otherwise. The seat never reads every seat's
// real decklist (ReadsDecklists is false), so the lab's Env gate passes it.
//
// The returned NamedPlayer (and every seat it builds) is safe to use from
// several goroutines at once; the agent serves them all on one session.
func (a *Agent) Player(l *draftlab.Lab) (draftlab.NamedPlayer, error) {
	if !servesKind(a.hello, wire.KindDecide) {
		return draftlab.NamedPlayer{}, fmt.Errorf("%w: agent %q serves %v, not %q",
			ErrRole, a.hello.Agent, a.hello.Kinds, wire.KindDecide)
	}
	return draftlab.NamedPlayer{
		Ref:            a.Ref(),
		ReadsDecklists: false,
		New: func(seed uint64) (seat.Seat, error) {
			// Fail-closed at seat construction only when the agent is
			// dead AND OnFailure is FailError; FailFallback keeps
			// building seats whose answers fall back per request.
			if a.dead() && a.opts.OnFailure != FailFallback {
				return nil, fmt.Errorf("%w: player seat (agent %s is gone)", ErrAgentGone, a.Ref())
			}
			return &extSeat{
				agent:          a,
				seatSeed:       seed,
				u:              unscopedCounter.Add(1),
				paymentActions: a.hello.PaymentActions,
				targetHints:    a.hello.TargetHints,
				omit:           a.omit,
			}, nil
		},
	}, nil
}

// extSeat is one Agent's game seat: a fresh instance per game (NamedPlayer
// New), driven by the game's single goroutine in seat order, so its fields
// need no lock.
type extSeat struct {
	agent    *Agent
	seatSeed uint64
	u        uint64 // the unscoped-stream number, taken at New

	// The game this seat serves, set at BeginGame. A nil token means the
	// seat was never begun: its requests go out unscoped ("u" streams).
	tok     *draftlab.GameToken
	gameNum uint64

	// Set by the first decide this seat sent during its game; EndGame
	// closes the stream only then (the agent keeps no state for a stream
	// it never saw).
	sentDecide bool
	lastStream string

	paymentActions bool
	targetHints    bool
	omit           view.Omit
}

// Decide answers one decision over the wire. A forced decision (with the
// hello's skip_forced) is answered locally and never sent; an illegal
// answer is a refusal answered with the deterministic fallback; a session
// failure follows Options.OnFailure.
func (s *extSeat) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	if s.agent.hello.SkipForced && forced(&d) {
		s.agent.statsForced.Add(1)
		return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{d.Options[0].Index}}, nil
	}
	fallback := func() decision.Intent {
		return bots.Fallbacks(&d, heuristic.BoardFromView(v), int(d.Player))[1]
	}
	if s.agent.dead() {
		var out decision.Intent
		err := s.agent.failed(s.agent.sess.stoppedError(), wire.KindDecide, func() { out = fallback() })
		return out, err
	}
	// Marshal NOW: the driver refills this View for the next decision as
	// soon as Decide returns, and the raw JSON must not read it after.
	raw, err := json.Marshal(v)
	if err != nil {
		return decision.Intent{}, fmt.Errorf("extern: encode view: %w", err)
	}
	// A nil *GameToken stored in an any is NOT a nil interface — guard it,
	// so a seat used outside a game counts unscoped instead of on a token
	// the hub has no record of.
	game := any(nil)
	if s.tok != nil {
		game = s.tok
	}
	item := wire.Item{
		Kind:   wire.KindDecide,
		Seed:   wire.Seed53(rand.New(rand.NewPCG(s.seatSeed^decideSeedSalt, d.Seq)).Uint64()),
		Stream: s.stream(int(d.Player)),
		View:   raw,
	}
	ans, err := s.agent.call(ctx, game, item)
	s.sentDecide = true
	s.lastStream = item.Stream
	if err != nil {
		var out decision.Intent
		err = s.agent.failed(err, wire.KindDecide, func() { out = fallback() })
		return out, err
	}
	if ans.Error != nil {
		var out decision.Intent
		if rerr := s.agent.refused(wire.KindDecide, itemErrorReason(ans.Error), func() { out = fallback() }); rerr != nil {
			return decision.Intent{}, rerr
		}
		return out, nil
	}
	if ans.Intent == nil {
		var out decision.Intent
		if rerr := s.agent.refused(wire.KindDecide, "the answer is not an intent", func() { out = fallback() }); rerr != nil {
			return decision.Intent{}, rerr
		}
		return out, nil
	}
	in := *ans.Intent
	in.Seq, in.Player = d.Seq, d.Player
	if err := d.Validate(in); err != nil {
		var out decision.Intent
		if rerr := s.agent.refused(wire.KindDecide, err.Error(), func() { out = fallback() }); rerr != nil {
			return decision.Intent{}, rerr
		}
		return out, nil
	}
	return in, nil
}

// AnswerRefused answers the decision the engine refused a validated intent
// for: counted, logged, and answered with the deterministic fallback. The
// strict mode does not apply here — the answer already passed the wire
// contract's Validate; only the engine found it illegal.
func (s *extSeat) AnswerRefused(v view.View, d decision.Decision, refused decision.Intent) decision.Intent {
	s.agent.statsRefused.Add(1)
	s.agent.logf("extern: %s decide refused (%s): answered by fallback",
		s.agent.Ref(), "the engine refused the intent")
	return bots.Fallbacks(&d, heuristic.BoardFromView(v), int(d.Player))[1]
}

// ViewOmit names the hello's mask for every decision.
func (s *extSeat) ViewOmit(*decision.Decision) view.Omit { return s.omit }

// WantsPaymentActions mirrors the hello's payment_actions flag.
func (s *extSeat) WantsPaymentActions() bool { return s.paymentActions }

// WantsTargetHints mirrors the hello's target_hints flag.
func (s *extSeat) WantsTargetHints() bool { return s.targetHints }

// BeginGame records the game token and its process-wide number; the seat's
// decide items name their stream "g<n>.<player>" from it.
func (s *extSeat) BeginGame(t *draftlab.GameToken) {
	s.tok = t
	s.gameNum = s.agent.beginGame(t)
}

// EndGame closes the seat's stream (only if it ever sent a decide — the
// agent keeps no state for a stream it never saw) and drops the game's
// reference on the hub.
func (s *extSeat) EndGame(t *draftlab.GameToken) {
	if s.tok == nil {
		return
	}
	if s.sentDecide {
		s.agent.sendStreamEnd(s.lastStream)
	}
	s.agent.endGame(t)
}

// stream names the seat's stream: "g<n>.<player>" inside a begun game,
// "u<u>.<player>" outside one. Stream ids are scheduling-dependent and
// never reach a result.
func (s *extSeat) stream(player int) string {
	if s.tok != nil {
		return fmt.Sprintf("g%d.%d", s.gameNum, player)
	}
	return fmt.Sprintf("u%d.%d", s.u, player)
}

// forced reports the skip_forced rule: a decision with no payment actions,
// no mana payment, exactly one option and Min == Max == 1 — the seat's
// answer is forced, so it can be produced locally.
func forced(d *decision.Decision) bool {
	return len(d.PaymentActions) == 0 && d.ManaPayment == nil &&
		len(d.Options) == 1 && d.Min == 1 && d.Max == 1
}
