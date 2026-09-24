package rules

import (
	"slices"

	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() { effects.RegisterNonAPI("kw:Storied") }

// checkEnduringStoryGrants folds CR 702.175a's threshold into the permanent,
// game-long designation. Each qualifying permanent is counted only once.
func (e *Engine) checkEnduringStoryGrants() {
	if e.G.Over {
		return
	}
	for _, p := range e.G.AliveFrom(0) {
		if int(p) >= len(e.G.Players) || e.G.Players[p].Lost || e.G.Players[p].EnduringStory {
			continue
		}
		board := e.G.Zone(state.ZBattlefield, p)
		storied, count := false, 0
		for _, id := range board {
			o := e.G.Obj(id)
			if o == nil || o.Zone != state.ZBattlefield {
				continue
			}
			if e.HasKeyword(id, "Storied") {
				storied = true
			}
			d := e.Derived(id)
			face := o.Face()
			if slices.Contains(d.Types, "Artifact") || slices.Contains(d.Types, "Saga") || (face != nil && face.IsLegendary()) {
				count++
			}
		}
		if storied && count >= 3 {
			e.emit(events.Event{Kind: events.EnduringStoryChange, Player: p, Text: "enduring story"})
		}
	}
}
