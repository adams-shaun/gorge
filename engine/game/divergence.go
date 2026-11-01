package game

import (
	"github.com/adams-shaun/gorge/engine/eventdiff"
	"github.com/adams-shaun/gorge/engine/events"
)

// Divergence is the error Engine.Replay, ReplayTo and Walk report when the
// rebuilt events leave the recording.
type Divergence = eventdiff.Divergence

// FirstDivergence compares two complete event streams, want the reference.
func FirstDivergence(want, got []events.Event) *Divergence {
	return eventdiff.First(want, got)
}
