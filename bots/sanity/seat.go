package sanity

import (
	"context"

	"github.com/adams-shaun/gorge/bots/heuristic"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
)

// Mulligan is the London-mulligan rule the guard answers keep/mulligan and
// bottoming asks with (botpolicy.MulliganConstructed, measured in the
// 2026-10-09 evaluation, "Mulligan" section). The coin the unwrapped `bot`
// and `sb-tactical` fall back to mulligans a third of all hands whatever
// they hold, and loses about two points to this rule. The guard answers the
// ask itself and does not consult the wrapped seat, so no rng is drawn.
const Mulligan = botpolicy.MulliganConstructed

// answerMulligan answers a KMulligan ask under rule from the deciding seat's
// own board; ok is false for any other decision or a shape the rule leaves to
// the wrapped seat.
func answerMulligan(b func() botpolicy.Board, d *decision.Decision, rule botpolicy.MulliganRule) (decision.Intent, bool) {
	if d.Kind != decision.KMulligan {
		return decision.Intent{}, false
	}
	c, ok := botpolicy.MulliganAnswer(b(), rule, d)
	if !ok {
		return decision.Intent{}, false
	}
	in := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: c}
	if d.Validate(in) != nil {
		return decision.Intent{}, false
	}
	return in, true
}

// Wrap decorates inner with the guard answering mulligans under Mulligan. The
// wrapper keeps inner's surfaces: a BoardSeat inner stays a BoardSeat (the
// host still hands it the Board, never a View), an EnvSeat inner stays an
// EnvSeat and RefusalAnswerer, and the payment opt-in is delegated. It also
// opts the seat into the engine's target hints. The wrapped seat is asked
// exactly once per decision, about the filtered option list, except a London
// mulligan ask, which the guard answers itself.
func Wrap(inner seat.Seat) seat.Seat { return WrapMulligan(inner, Mulligan) }

// WrapMulligan is Wrap with the London-mulligan rule chosen by the caller, so
// a policy whose mulligan differs from the guard's default (forge-heuristic's
// MulliganForge) is not silently overridden by the common-sense wrap.
func WrapMulligan(inner seat.Seat, rule botpolicy.MulliganRule) seat.Seat {
	base := guarded{inner: inner, mulligan: rule}
	if _, ok := inner.(heuristic.BoardSeat); ok {
		return guardedBoard{base}
	}
	if es, ok := inner.(bots.EnvSeat); ok {
		if ra, ok := inner.(bots.RefusalAnswerer); ok {
			return guardedEnv{base, es, ra}
		}
	}
	return base
}

type guarded struct {
	inner    seat.Seat
	mulligan botpolicy.MulliganRule
}

func (s guarded) UnwrapSeat() seat.Seat { return s.inner }

// WantsTargetHints asks the host for decision.Option.TargetHints.
func (s guarded) WantsTargetHints() bool { return true }

func (s guarded) WantsPaymentActions() bool {
	if c, ok := s.inner.(seat.PaymentPlanConsumer); ok {
		return c.WantsPaymentActions()
	}
	return false
}

func (s guarded) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	if in, ok := answerMulligan(func() botpolicy.Board { return heuristic.BoardFromView(v) }, &d, s.mulligan); ok {
		return in, nil
	}
	f := Filter(d)
	in, err := s.inner.Decide(ctx, v, f.D)
	if err != nil {
		return in, err
	}
	return f.MapBack(in), nil
}

type guardedBoard struct{ guarded }

func (s guardedBoard) DecideBoard(ctx context.Context, b botpolicy.Board, d decision.Decision) (decision.Intent, error) {
	if in, ok := answerMulligan(func() botpolicy.Board { return b }, &d, s.mulligan); ok {
		return in, nil
	}
	f := Filter(d)
	in, err := s.inner.(heuristic.BoardSeat).DecideBoard(ctx, b, f.D)
	if err != nil {
		return in, err
	}
	return f.MapBack(in), nil
}

type guardedEnv struct {
	guarded
	es bots.EnvSeat
	ra bots.RefusalAnswerer
}

func (s guardedEnv) WantsEnv(d *decision.Decision) bool { return s.es.WantsEnv(d) }

func (s guardedEnv) DecideEnv(ctx context.Context, env bots.Env, d decision.Decision) (decision.Intent, error) {
	if in, ok := answerMulligan(func() botpolicy.Board { return heuristic.BoardFromView(env.View) }, &d, s.mulligan); ok {
		return in, nil
	}
	f := Filter(d)
	in, err := s.es.DecideEnv(ctx, env, f.D)
	if err != nil {
		return in, err
	}
	return f.MapBack(in), nil
}

func (s guardedEnv) AnswerRefused(v view.View, d decision.Decision, refused decision.Intent) decision.Intent {
	f := Filter(d)
	return f.MapBack(s.ra.AnswerRefused(v, f.D, f.MapIn(refused)))
}
