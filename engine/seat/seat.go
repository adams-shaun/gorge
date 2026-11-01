// Package seat is who answers the engine's decisions. It sits above rules in
// the dependency order (cards -> state -> decision -> events -> effects ->
// rules -> view -> seat -> replay -> cmd/*): a Seat is handed a view.View,
// never a *state.Game or a *rules.Engine, so it only ever sees what a real
// client would.
package seat

import (
	"context"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/view"
)

// Seat is anything that answers decisions: a bot, a scripted test seat, or
// (later) a WebSocket-backed human. ctx lets a human seat time out; a bot
// ignores it and never returns an error.
type Seat interface {
	Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error)
}

// PaymentPlanConsumer opts into the lazy payment-action extension: a driver
// (testkit/bench.PlayGame, server/host/match.go, cmd tools) calls
// Engine.EnsurePaymentActions before a decision is handed to a seat that
// answers true, so Decision.PaymentActions carries the engine's mana-payment
// plans. A seat that does not implement it, or answers false, gets no plans.
//
// Bots answer true by default. A bot without plans pays by tapping mana by
// hand, and a search seat whose fallback bot taps sees that tap as an answer
// outside its candidates: az lost 89% of its counterspell windows that way.
// So every bot implements this interface, and a wrapper
// forwards to what it wraps (or answers true). A bot opts out only with a
// stated reason: its policy reads the manual option list and never a plan
// (a manual-mana Bot, heuristic.PolicyNetBot, the searchteacher seat, the
// generation-0 az-redeal config). bots/all's
// TestRegisteredBotsConsumePaymentPlans enforces this for every registered
// policy, with the opt-outs named there.
//
// Human/client seats are not consumers: the host builds their plans itself
// (host.HumanSeat under TableConfig.AutoMana); the client only picks one, and
// the engine validates the pick.
type PaymentPlanConsumer interface {
	WantsPaymentActions() bool
}

// TargetHintConsumer opts into decision.Option.TargetHints on priority
// decisions (Engine.EnsureTargetHints). Information only; the engine offers
// every legal action either way.
type TargetHintConsumer interface {
	WantsTargetHints() bool
}
