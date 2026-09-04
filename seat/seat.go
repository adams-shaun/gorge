// Package seat is who answers the engine's decisions. It sits above rules in
// the dependency order (cards -> state -> decision -> events -> effects ->
// rules -> view -> seat -> replay -> cmd/*): a Seat is handed a view.View,
// never a *state.Game or a *rules.Engine, so it only ever sees what a real
// client would.
package seat

import (
	"context"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/view"
)

// Seat is anything that answers decisions: a bot, a scripted test seat, or
// (later) a WebSocket-backed human. ctx lets a human seat time out; a bot
// ignores it and never returns an error (Ruling P8).
type Seat interface {
	Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error)
}
